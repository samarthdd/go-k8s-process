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
	JeagerStatus  bool
)

type amqpHeadersCarrier map[string]interface{}

func main() {
	JeagerStatusEnv := os.Getenv("JAEGER_AGENT_ON")
	if JeagerStatusEnv == "true" {
		JeagerStatus = true
	} else {
		JeagerStatus = false
	}
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

			if JeagerStatus == true {
				tracer, closer := tracing.Init("process")
				defer closer.Close()
				opentracing.SetGlobalTracer(tracer)
				ProcessTracer = tracer
			}
			zlog.Info().Msg("received message from queue ")

			err := ProcessMessage(d.Headers)
			if err != nil {
				processend(err)
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
func processend(err error) {
	if JeagerStatus == true && ctx != nil {
		fmt.Println(err)

		span, _ := opentracing.StartSpanFromContext(ctx, "ProcessingEndError")
		defer span.Finish()
		span.LogKV("event", err)
	}

}

func ProcessMessage(d amqp.Table) error {
	if JeagerStatus == true {

		if d["uber-trace-id"] != nil {

			spCtx, ctxsuberr := ExtractWithTracer(d, ProcessTracer)
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
			if d["file-id"] == nil {
				helloTo = "nil-file-id"
			} else {
				helloTo = d["file-id"].(string)

			}
			sp.SetTag("msg-procces", helloTo)
			defer sp.Finish()
			ctxsubtx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			// Update the context with the span for the subsequent reference.
			ctx = opentracing.ContextWithSpan(ctxsubtx, sp)

		} else {
			if d["file-id"] == nil {
				helloTo = "nil-file-id"
			} else {
				helloTo = d["file-id"].(string)

			}
			span := ProcessTracer.StartSpan("go-k8s-process")
			span.SetTag("msg-procces", helloTo)
			defer span.Finish()

			ctx = opentracing.ContextWithSpan(context.Background(), span)

		}

		//helloStr = msgrecivedTrace(helloTo)
	}

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
	if JeagerStatus == true {

		span, _ := opentracing.StartSpanFromContext(ctx, "PublishOutExchange")
		defer span.Finish()
		span.LogKV("event", "publish")
	}

	err = rabbitmq.PublishMessage(publisher, ProcessingOutcomeExchange, ProcessingOutcomeRoutingKey, d, []byte(""))
	if err != nil {
		return fmt.Errorf("error failed to publish message to the ProcessingOutcome queue :%s", err)
	}

	// Inject the span context into the AMQP header.
	zlog.Info().Str("Exchange", ProcessingOutcomeExchange).Str("RoutingKey", ProcessingOutcomeRoutingKey).Msg("message published to queue ")

	return nil
}

func clirebuildProcess(f []byte, fileid string, d amqp.Table) {
	var span opentracing.Span
	if JeagerStatus == true {
		span, _ = opentracing.StartSpanFromContext(ctx, "clirebuildProcess")
		defer span.Finish()
		span.LogKV("event", "clirebuildProcess")
	}

	randPath := rebuildexec.RandStringRunes(16)
	fileTtype := "*" // wild card
	fd := rebuildexec.New(f, fileid, fileTtype, randPath)
	err := fd.Rebuild()
	log.Printf("\033[34m rebuild status is  : %s\n", fd.PrintStatus())

	if err != nil {
		if JeagerStatus == true {

			span.LogKV("Errror", err)
		}
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
	if JeagerStatus == true {
		span, _ := opentracing.StartSpanFromContext(ctx, "getFile")
		defer span.Finish()
		span.LogKV("event", "getFile")
	}
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
	return f, nil

}

func uploadMinio(file []byte, filename string) (string, error) {
	if minioClient == nil {
		return "", fmt.Errorf("minio client not found")
	}
	if JeagerStatus == true {
		span, _ := opentracing.StartSpanFromContext(ctx, "uploadMinio")
		defer span.Finish()
		span.LogKV("event", filename)
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
