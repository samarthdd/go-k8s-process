package main

import (
	"bytes"
	"context"
	"errors"
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

	"github.com/k8-proxy/go-k8s-process/tracing"
	miniov7 "github.com/minio/minio-go/v7"
	"github.com/opentracing/opentracing-go"

	traclog "github.com/opentracing/opentracing-go/log"
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

	publisher     *amqp.Channel
	minioClient   *miniov7.Client
	ctx           context.Context
	helloTo       string
	helloStr      string
	ProcessTracer opentracing.Tracer
)

type amqpHeadersCarrier map[string]interface{}

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

			tracer, closer := tracing.Init("process")
			defer closer.Close()
			opentracing.SetGlobalTracer(tracer)
			ProcessTracer = tracer

			zlog.Info().Msg("received message from queue")

			err := ProcessMessage(d)
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
	if d.Headers["file-id"] != nil {

		if d.Headers["uber-trace-id"] != nil {

			spCtx1, _ := Extract(d.Headers)
			if spCtx1 == nil {
				fmt.Println("cpctxsub nil 2")
			}
			spCtx, ctxsuberr := ExtractWithTracer(d.Headers, ProcessTracer)
			if spCtx == nil {
				fmt.Println("cpctxsub nil 1")
			}
			if ctxsuberr != nil {
				fmt.Println(ctxsuberr)
			}

			// Extract the span context out of the AMQP header.
			sp := opentracing.StartSpan(
				"go-k8s-process",
				opentracing.FollowsFrom(spCtx),
			)
			helloTo = d.Headers["file-id"].(string)
			sp.SetTag("msg-procces", helloTo)
			defer sp.Finish()
			ctxsubtx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			// Update the context with the span for the subsequent reference.
			ctx = opentracing.ContextWithSpan(ctxsubtx, sp)

		} else {
			helloTo = d.Headers["file-id"].(string)
			span := ProcessTracer.StartSpan("go-k8s-process")
			span.SetTag("msg-procces", helloTo)
			defer span.Finish()

			ctx = opentracing.ContextWithSpan(context.Background(), span)

		}

		//helloStr = msgrecivedTrace(helloTo)
	}

	f, err := getFile(sourcePresignedURL)
	if err != nil {
		return fmt.Errorf("error failed to download from Minio:%s", err)
	}

	zlog.Info().Msg("file downloaded from minio successfully")

	var fn []byte
	var gwreport []byte
	err = nil

	fn, gwreport, err = clirebuildProcess(f, fileID)
	if err != nil {

		zlog.Error().Err(err).Msg("error failed to rebuild file")
		fn = []byte("error : the  rebuild engine failed to rebuild file")

	} else {package main

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

	log.Printf("\033[32m GW rebuild SDK version : %s\n", rebuildexec.GetSdkVersion())

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
	fileTtype := "*" // wild card
	fd := rebuildexec.New(f, fileid, fileTtype, randPath)
	err := fd.Rebuild()
	log.Printf("\033[34m rebuild status is  : %s\n", fd.PrintStatus())

	if err != nil {
		zlog.Error().Err(err).Msg("error failed to rebuild file")

		return
	}

	d["rebuild-processing-status"] = fd.PrintStatus()
	d["rebuild-sdk-version"] = rebuildexec.GetSdkVersion()

	zlog.Info().Msg("file rebuilt process  successfully ")

	generateReport := ""
	if d["generate-report"] != nil {
		generateReport = d["generate-report"].(string)
	}

	if generateReport == "true" {
		report := fd.ReportFile
		if report == nil {
			zlog.Error().Msg("error rebuildexec fileRreport function")

		} else {

			minioUploadProcess(report, fileid, ".xml", "report-presigned-url", d)
		}

	}

	file := fd.RebuiltFile

	if file == nil {
		zlog.Error().Msg("error rebuildexec FileProcessed function")

	} else {
		minioUploadProcess(file, "rebuild-", fileid, "clean-presigned-url", d)

	}

	gwlogFile := fd.GwLogFile
	if gwlogFile == nil {

		zlog.Error().Msg("error rebuildexec GwFileLog function")

	} else {
		minioUploadProcess(gwlogFile, fileid, ".gw.log", "gwlog-presigned-url", d)

	}

	logFile := fd.LogFile

	if logFile == nil {

		zlog.Error().Msg("error rebuildexec GwFileLog function")

	} else {
		minioUploadProcess(logFile, fileid, ".log", "log-presigned-url", d)

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
		zlog.Info().Msg("file rebuilt successfully ")

	}

	fileid := fmt.Sprintf("rebuild-%s", fileID)
	reportid := fmt.Sprintf("report-%s.xml", fileID)
	urlp, err := uploadMinio(fn, fileid)
	if err != nil {
		return fmt.Errorf("error failed to upload file to Minio :%s", err)

	}
	zlog.Info().Msg("file uploaded to minio successfully")

	if generateReport == "true" {
		_, err = uploadMinio(gwreport, reportid)
		if err != nil {
			return fmt.Errorf("failed to upload report file to Minio :%s", err)
		}
		zlog.Info().Msg("report file uploaded to minio successfully")
	}

	d.Headers["clean-presigned-url"] = urlp

	// Publish the details to Rabbit
	if publisher != nil {
		span, _ := opentracing.StartSpanFromContext(ctx, "ProcessingOutcomeExchange")
		defer span.Finish()
		span.LogKV("event", "publish")
		headers := d.Headers
		// Inject the span context into the AMQP header.

		if err := Inject(span, headers); err != nil {
			return err
		}
		err = rabbitmq.PublishMessage(publisher, ProcessingOutcomeExchange, ProcessingOutcomeRoutingKey, d.Headers, []byte(""))
		if err != nil {
			return fmt.Errorf("error failed to publish message to the ProcessingOutcome queue :%s", err)
		}
		zlog.Info().Str("Exchange", ProcessingOutcomeExchange).Str("RoutingKey", ProcessingOutcomeRoutingKey).Msg("message published to queue ")
	} else {
		return fmt.Errorf("publisher not found")
	}

	return nil
}

func clirebuildProcess(f []byte, fileid string) ([]byte, []byte, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "clirebuildProcess")
	defer span.Finish()
	span.LogKV("event", "clirebuildProcess")
	if minioClient == nil {
		return nil, nil, fmt.Errorf("minio client not found")
	}

	randPath := rebuildexec.RandStringRunes(16)
	fd := rebuildexec.New(f, fileid, randPath)
	err := fd.Rebuild()
	if err != nil {
		err = fmt.Errorf("error rebuild function : %s", err)
		return nil, nil, err
	}

	report, err := fd.FileRreport()
	if err != nil {
		err = fmt.Errorf("error rebuildexec fileRreport function : %s", err)

		return nil, nil, err

	}

	file, err := fd.FileProcessed()

	if err != nil {
		err = fmt.Errorf("error rebuildexec FileProcessed function : %s", err)

		return nil, nil, err

	}
	err = fd.Clean()
	if err != nil {
		err = fmt.Errorf("error rebuildexec Clean function : %s", err)

		return nil, nil, err

	}
	return file, report, nil
}

func getFile(url string) ([]byte, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "getFile")
	defer span.Finish()
	span.LogKV("event", "getFile")
	if minioClient == nil {
		return nil, fmt.Errorf("minio client not found")
	}

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
	fmt.Println(string(f))
	return f, nil

}

func uploadMinio(file []byte, filename string) (string, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "uploadMinio")
	defer span.Finish()
	span.LogKV("event", "uploadMinio")
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

func tracest(msg string) {
	tracer, closer := tracing.Init("process")
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)
	helloTo = msg
	span := tracer.StartSpan("process file")
	span.SetTag("msg-procces", helloTo)
	defer span.Finish()

	ctx = opentracing.ContextWithSpan(context.Background(), span)

	helloStr = msgrecivedTrace(helloTo)
}
func msgrecivedTrace(helloTo string) string {
	span, _ := opentracing.StartSpanFromContext(ctx, "processing-request")
	defer span.Finish()

	helloStr = fmt.Sprintf("new processing file id, %s!", helloTo)
	span.LogFields(
		traclog.String("event", "start-processing-request"),
		traclog.String("value", helloStr),
	)

	return helloStr
}
func mgsendTrace(ctx context.Context, helloStr string) {
	span, _ := opentracing.StartSpanFromContext(ctx, helloStr)
	defer span.Finish()
	span.LogKV("event", helloStr)
}

func Inject(span opentracing.Span, hdrs amqp.Table) error {
	c := amqpHeadersCarrier(hdrs)
	return span.Tracer().Inject(span.Context(), opentracing.TextMap, c)
}
func Extract(hdrs amqp.Table) (opentracing.SpanContext, error) {
	c := amqpHeadersCarrier(hdrs)
	return opentracing.GlobalTracer().Extract(opentracing.TextMap, c)
}
func (c amqpHeadersCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, val := range c {
		v, ok := val.(string)
		if !ok {
			continue
		}
		if err := handler(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Set implements Set() of opentracing.TextMapWriter.
func (c amqpHeadersCarrier) Set(key, val string) {
	c[key] = val
}
func ExtractWithTracer(hdrs amqp.Table, tracer opentracing.Tracer) (opentracing.SpanContext, error) {
	if tracer == nil {
		return nil, errors.New("tracer is nil")
	}
	c := amqpHeadersCarrier(hdrs)
	return tracer.Extract(opentracing.TextMap, c)
}
