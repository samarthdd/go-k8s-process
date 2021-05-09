package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq connection ")
	}

	// Initiate a publisher on processing exchange
	publisher, err = rabbitmq.NewQueuePublisher(connection, ProcessingOutcomeExchange, amqp.ExchangeDirect)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq publisher ")
	}

	// Start a consumer
	msgs, ch, err := rabbitmq.NewQueueConsumer(connection, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq consumer ")
	}
	defer ch.Close()

	minioClient, err = minio.NewMinioClient(minioEndpoint, minioAccessKey, minioSecretKey, false)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start minio client ")
	}

	forever := make(chan bool)

	// Consume
	go func() {
		for d := range msgs {
			zlog.Info().Msg("received message from queue ")

			err := ProcessMessage(d.Headers)
			if err != nil {
				zlog.Error().Err(err).Msg("error Failed to process message")
			}

			// Closing the channel to exit
			zlog.Info().Msg(" closing the channel")
			close(forever)
		}
	}()

	log.Printf("Waiting for messages")
	<-forever

}

func ProcessMessage(d amqp.Table) error {

	if d["file-id"] == nil ||
		d["source-presigned-url"] == nil {
		return fmt.Errorf("Headers value is nil")
	}

	fileID := d["file-id"].(string)
	sourcePresignedURL := d["source-presigned-url"].(string)

	f, err := getFile(sourcePresignedURL)
	if err != nil {
		return fmt.Errorf("error failed to download from Minio:%s", err)
	}

	zlog.Info().Msg("file downloaded from minio successfully")

	clirebuildProcess(f, fileID, d)

	// Publish the details to Rabbit
	if publisher == nil {
		return fmt.Errorf("couldn't start publisher")
	}

	err = rabbitmq.PublishMessage(publisher, ProcessingOutcomeExchange, ProcessingOutcomeRoutingKey, d, []byte(""))
	if err != nil {
		return fmt.Errorf("error failed to publish message to the ProcessingOutcome queue :%s", err)
	}
	zlog.Info().Str("Exchange", ProcessingOutcomeExchange).Str("RoutingKey", ProcessingOutcomeRoutingKey).Msg("message published to queue ")

	return nil
}

func clirebuildProcess(f []byte, fileid string, d amqp.Table) {

	randPath := rebuildexec.RandStringRunes(16)
	fd := rebuildexec.New(f, fileid, randPath)
	err := fd.Rebuild()
	log.Printf("\033[34m rebuild status is  : %s\n", fd.PrintStatus())

	d["rebuild-processing-status"] = fd.PrintStatus()
	d["rebuild-sdk-version"] = rebuildexec.GetSdkVersion()

	if err != nil {
		zlog.Error().Err(err).Msg("error failed to rebuild file")

		return
	}

	zlog.Info().Msg("file rebuilt process  successfully ")

	generateReport := ""
	if d["generate-report"] != nil {
		generateReport = d["generate-report"].(string)
	}

	if generateReport == "true" {
		report, err := fd.FileRreport()
		if err != nil {
			zlog.Error().Err(err).Msg("error rebuildexec fileRreport function")

		} else {

			minioUploadProcess(report, fileid, "report.xml", "report-presigned-url", d)
		}

	}

	file, err := fd.FileProcessed()

	if err != nil {
		zlog.Error().Err(err).Msg("error rebuildexec FileProcessed function")

	} else {
		minioUploadProcess(file, "rebuild-", fileid, "clean-presigned-url", d)

	}

	gwlogFile, err := fd.GwFileLog()
	if err != nil {

		zlog.Error().Err(err).Msg("error rebuildexec GwFileLog function")

	} else {
		minioUploadProcess(gwlogFile, fileid, "gwfilelog.log", "gwlog-presigned-url", d)

	}

	logFile, err := fd.FileLog()

	if err != nil {

		zlog.Error().Err(err).Msg("error rebuildexec GwFileLog function")

	} else {
		minioUploadProcess(logFile, fileid, "filelog.log", "log-presigned-url", d)

	}

	err = fd.Clean()
	if err != nil {
		zlog.Error().Err(err).Msg("error rebuildexec Clean function : %s")

	}
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
	if minioClient == nil {
		return "", fmt.Errorf("minio client not found")
	}
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

func minioUploadProcess(file []byte, baseName, extName, headername string, d amqp.Table) {

	reportid := fmt.Sprintf("%s%s", baseName, extName)

	urlr, err := uploadMinio(file, reportid)
	if err != nil {
		m := fmt.Sprintf("failed to upload %s file to Minio", extName)
		zlog.Info().Msg(m)
		return
	}
	m := fmt.Sprintf("%s file uploaded to minio successfully", extName)

	zlog.Info().Msg(m)
	d[headername] = urlr
}
