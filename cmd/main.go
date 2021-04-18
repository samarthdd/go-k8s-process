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

	zlog "github.com/rs/zerolog/log"

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
		zlog.Fatal().Err(err).Msg("could not start rabbitmq connection ")
	}

	// Initiate a publisher on processing exchange
	publisher, err = rabbitmq.NewQueuePublisher(connection, ProcessingOutcomeExchange, amqp.ExchangeDirect)
	if err != nil {
		zlog.Fatal().Err(err).Msg("could not start publisher ")
	}

	// Start a consumer
	msgs, ch, err := rabbitmq.NewQueueConsumer(connection, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
	if err != nil {
		zlog.Fatal().Err(err).Msg("could not start consumer ")
	}
	defer ch.Close()

	minioClient, err = minio.NewMinioClient(minioEndpoint, minioAccessKey, minioSecretKey, false)
	if err != nil {
		zlog.Fatal().Err(err).Msg("could not start minio client ")
	}

	forever := make(chan bool)

	// Consume
	go func() {
		for d := range msgs {
			zlog.Info().Msg("received message from queue ")

			err := ProcessMessage(d)
			if err != nil {
				zlog.Error().Err(err).Msg("Failed to process message")
			}

			// Closing the channel to exit
			zlog.Info().Msg(" closing the channel")
			close(forever)
		}
	}()

	log.Printf("Waiting for messages")
	<-forever

}

func ProcessMessage(d amqp.Delivery) error {

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

	// Download the file to output file location
	downloadPath := "/tmp/" + filepath.Base(rebuiltLocation)

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

	f, err := getFile(sourcePresignedURL)
	if err != nil {
		zlog.Fatal().Err(err).Msg("failed to download from Minio ")
		return err
	}

	var fn []byte
	var gwreport []byte
	err = nil

	fn, gwreport, err = clirebuildProcess(f, fileID)
	if err != nil {

		zlog.Fatal().Err(err).Msg("failed to rebuild file")
		fn = []byte("error : the  rebuild engine failed to rebuild file")

	}

	fileid := fmt.Sprintf("rebuild-%s", fileID)
	reportid := fmt.Sprintf("report-%s.xml", fileID)
	urlp, err := uploadMinio(fn, fileid)
	if err != nil {
		log.Println("Minio upload error")
	}

	_, err = uploadMinio(gwreport, reportid)
	if err != nil {
		zlog.Fatal().Err(err).Msg("failed to upload to Minio")
	}

	d.Headers["clean-presigned-url"] = urlp

	zlog.Info().Msg("file uploaded to minio successfully")

	// Publish the details to Rabbit

	err = rabbitmq.PublishMessage(publisher, ProcessingOutcomeExchange, ProcessingOutcomeRoutingKey, d.Headers, []byte(""))
	if err != nil {
		return err
	}
	zlog.Info().Str("Exchange", ProcessingOutcomeExchange).Str("RoutingKey", ProcessingOutcomeRoutingKey).Msg("message published to queue ")

	return nil
}

func clirebuildProcess(f []byte, fileid string) ([]byte, []byte, error) {
	l := rebuildexec.RandStringRunes(16)
	fd := rebuildexec.New(f, fileid, l)
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
	err = fd.Clean()
	if err != nil {
		return nil, nil, err

	}
	return file, report, nil
}

func getFile(url string) ([]byte, error) {

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

func uploadMinio(file []byte, filename string) (string, error) {

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
