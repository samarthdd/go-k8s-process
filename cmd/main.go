package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/k8-proxy/k8-go-comm/pkg/minio"
	"github.com/k8-proxy/k8-go-comm/pkg/rabbitmq"
	"github.com/streadway/amqp"

	miniov7 "github.com/minio/minio-go/v7"
)

var (
	exchange   = "processing-exchange"
	routingKey = "processing-request"
	queueName  = "processing-request-queue"

	clean_exchange   = "processing-outcome-exchange"
	clean_routingKey = "processing-outcome-request"

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
	publisher, err = rabbitmq.NewQueuePublisher(connection, clean_exchange)
	if err != nil {
		log.Fatalf("%s", err)
	}

	// Start a consumer
	msgs, ch, err := rabbitmq.NewQueueConsumer(connection, queueName, exchange, routingKey)
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer ch.Close()

	minioClient, err = minio.NewMinioClient(minioEndpoint, minioAccessKey, minioSecretKey, true)
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

	cmd := exec.Command("dotnet", "Service.dll")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("File processing error : %s\n", err.Error())
		return err
	}
	fmt.Printf("File processing output : %s\n", out)

	// Upload the source file to Minio and Get presigned URL
	cleanPresignedURL, err := minio.UploadAndReturnURL(minioClient, cleanMinioBucket, output, time.Second*60*60*24)
	if err != nil {
		return err
	}
	d.Headers["clean-presigned-url"] = cleanPresignedURL.String()

	// Publish the details to Rabbit
	fmt.Printf("%+v\n", d.Headers)

	err = rabbitmq.PublishMessage(publisher, clean_exchange, clean_routingKey, d.Headers, []byte(""))
	if err != nil {
		return err
	}

	return nil
}
