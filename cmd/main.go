package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/k8-proxy/k8-go-comm/pkg/minio"
	"github.com/k8-proxy/k8-go-comm/pkg/rabbitmq"
	"github.com/streadway/amqp"

	"github.com/k8-proxy/go-k8s-process/rebuildexec"

	miniov7 "github.com/minio/minio-go/v7"
)

var (
	ProcessingRequestExchange   = "processing-request-exchange"
	ProcessingRequestRoutingKey = "processing-request"
	ProcessingRequestQueueName  = "processing-request-queue"

	ProcessingOutcomeExchange   = "processing-outcome-exchange"
	ProcessingOutcomeRoutingKey = "processing-outcome"
	ProcessingOutcomeQueueName  = "processing-outcome-queue"

	inputMount                     = os.Getenv("INPUT_MOUNT")
	adaptationRequestQueueHostname = os.Getenv("ADAPTATION_REQUEST_QUEUE_HOSTNAME")
	adaptationRequestQueuePort     = os.Getenv("ADAPTATION_REQUEST_QUEUE_PORT")
	messagebrokeruser              = os.Getenv("MESSAGE_BROKER_USER")
	messagebrokerpassword          = os.Getenv("MESSAGE_BROKER_PASSWORD")

	archiveAdaptationRequestQueueHostname = os.Getenv("ARCHIVE_ADAPTATION_QUEUE_REQUEST_HOSTNAME")
	archiveAdaptationRequestQueuePort     = os.Getenv("ARCHIVE_ADAPTATION_REQUEST_QUEUE_PORT")
	transactionEventQueueHostname         = os.Getenv("TRANSACTION_EVENT_QUEUE_HOSTNAME")
	transactionEventQueuePort             = os.Getenv("TRANSACTION_EVENT_QUEUE_PORT")

	requestProcessingTimeout = os.Getenv("REQUEST_PROCESSING_TIMEOUT")
	requestProcessingImage   = os.Getenv("REQUEST_PROCESSING_IMAGE")

	minioEndpoint    = os.Getenv("MINIO_ENDPOINT")
	minioAccessKey   = os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey   = os.Getenv("MINIO_SECRET_KEY")
	cleanMinioBucket = os.Getenv("MINIO_CLEAN_BUCKET")

	publisher   *amqp.Channel
	minioClient *miniov7.Client
)

func main() {

	// Get a connection
	connection, err := rabbitmq.NewInstance(adaptationRequestQueueHostname, adaptationRequestQueuePort, messagebrokeruser, messagebrokerpassword)
	if err != nil {
		log.Fatalf("%s", err)
	}

	// Initiate a publisher on processing exchange
	publisher, err = rabbitmq.NewQueuePublisher(connection, ProcessingOutcomeExchange, amqp.ExchangeDirect)
	if err != nil {
		log.Fatalf("%s", err)
	}

	// Start a consumer
	msgs, ch, err := rabbitmq.NewQueueConsumer(connection, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer ch.Close()

	minioClient, err = minio.NewMinioClient(minioEndpoint, minioAccessKey, minioSecretKey, false)
	if err != nil {
		log.Fatalf("%s", err)
	}

	forever := make(chan bool)

	// Consume
	go func() {
		for d := range msgs {
			err := processMessage(d)
			if err != nil {
				log.Printf("Failed to process message: %v", err)
			}

			// Closing the channel to exit
			log.Printf("File processed, closing the channel")
			close(forever)
		}
	}()

	log.Printf("[*] Waiting for messages. To exit press CTRL+C")
	<-forever

}

func processMessage(d amqp.Delivery) error {

	if d.Headers["file-id"] == nil ||
		d.Headers["source-presigned-url"] == nil {
		return fmt.Errorf("Headers value is nil")
	}

	generateReport := "false"

	if d.Headers["generate-report"] != nil {
		generateReport = d.Headers["generate-report"].(string)
	}

	fileID := d.Headers["file-id"].(string)
	sourcePresignedURL := d.Headers["source-presigned-url"].(string)
	rebuiltLocation := d.Headers["rebuilt-file-location"].(string)

	log.Printf("Received a message for file: %s, sourcePresignedURL: %s, rebuiltLocation: %s", fileID, sourcePresignedURL, rebuiltLocation)

	// Download the file to output file location
	downloadPath := "/tmp/" + filepath.Base(rebuiltLocation)
	err := minio.DownloadObject(sourcePresignedURL, downloadPath)
	if err != nil {
		return err
	}

	output := "/tmp/" + fileID

	os.Setenv("FileId", fileID)
	os.Setenv("InputPath", downloadPath)
	os.Setenv("OutputPath", output)
	os.Setenv("GenerateReport", generateReport)
	os.Setenv("ReplyTo", d.ReplyTo)
	os.Setenv("ProcessingTimeoutDuration", requestProcessingTimeout)
	os.Setenv("AdaptationRequestQueueHostname", adaptationRequestQueueHostname)
	os.Setenv("AdaptationRequestQueuePort", adaptationRequestQueuePort)
	os.Setenv("ArchiveAdaptationRequestQueueHostname", archiveAdaptationRequestQueueHostname)
	os.Setenv("ArchiveAdaptationRequestQueuePort", archiveAdaptationRequestQueuePort)
	os.Setenv("TransactionEventQueueHostname", transactionEventQueueHostname)
	os.Setenv("TransactionEventQueuePort", transactionEventQueuePort)
	os.Setenv("MessageBrokerUser", messagebrokeruser)
	os.Setenv("MessageBrokerPassword", messagebrokerpassword)

	//cmd := exec.Command("dotnet", "Service.dll")
	//out, err := cmd.CombinedOutput()
	//	if err != nil {
	//log.Printf("File processing error : %s\n", err.Error())
	//	return err
	//	}
	//	fmt.Printf("File processing output : %s\n", out)

	f, err := getfile(sourcePresignedURL)
	if err != nil {
		log.Println("Minio download error")
		return err
	}

	var fn []byte
	err = nil

	fn, _, err = clirebuildprocess(f, fileID)
	if err != nil {

		log.Println("error rebuild ", err)
		fn = []byte("error")

	}

	// Upload the source file to Minio and Get presigned URL
	//cleanPresignedURL, err := minio.UploadAndReturnURL(minioClient, cleanMinioBucket, output, time.Second*60*60*24)
	//if err != nil {
	//	return err
	//}
	fileid := fmt.Sprintf("rebuild-%s", fileID)
	urlp, err := st(fn, fileid)
	if err != nil {
		log.Println("Minio upload error")
	}
	d.Headers["clean-presigned-url"] = urlp

	// Publish the details to Rabbit
	fmt.Printf("%+v\n", d.Headers)

	err = rabbitmq.PublishMessage(publisher, ProcessingOutcomeExchange, ProcessingOutcomeRoutingKey, d.Headers, []byte(""))
	if err != nil {
		return err
	}

	return nil
}

func clirebuildprocess(f []byte, reqid string) ([]byte, []byte, error) {

	fd := rebuildexec.New(f, reqid)
	err := fd.Rebuild()
	if err != nil {
		return nil, nil, err
	}

	report, err := fd.FileRreport()
	if err != nil {
		return nil, nil, err
	}
	file, err := fd.FileProcessed()

	if err != nil {
		return nil, nil, err

	}
	return file, report, nil
}

func getfile(url string) ([]byte, error) {

	f := []byte{}
	resp, err := http.Get(url)
	if err != nil {
		return f, err
	}
	defer resp.Body.Close()

	f, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return f, err
	}
	return f, nil

}

func st(file []byte, filename string) (string, error) {

	exist, err := minio.CheckIfBucketExists(minioClient, cleanMinioBucket)
	if err != nil || !exist {
		return "", err

	}
	_, errm := minio.UploadFileToMinio(minioClient, cleanMinioBucket, filename, bytes.NewReader(file))
	if errm != nil {
		return "", errm
	}

	expirein := time.Second * 60 * 2
	urlx, err := minio.GetPresignedURLForObject(minioClient, cleanMinioBucket, filename, expirein)
	if err != nil {
		return "", err

	}

	return urlx.String(), nil

}
