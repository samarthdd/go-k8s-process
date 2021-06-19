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
	JeagerStatusEnv  = os.Getenv("JAEGER_AGENT_ON")

	publisher     *amqp.Channel
	minioClient   *miniov7.Client
	ctx           context.Context
	TraceFileid   string
	ProcessTracer opentracing.Tracer
	JeagerStatus  bool
	connrecive    *amqp.Connection
	connsend      *amqp.Connection
)

const (
	presignedUrlExpireIn = time.Minute * 10
)

type amqpHeadersCarrier map[string]interface{}

func main() {

	if JeagerStatusEnv == "true" {
		JeagerStatus = true
	} else {
		JeagerStatus = false
	}
	var err error
	// Get a connrecive
	connrecive, err = rabbitmq.NewInstance(adaptationRequestQueueHostname, adaptationRequestQueuePort, messagebrokeruser, messagebrokerpassword)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq connrecive")
	}
	// Get a send
	connsend, err = rabbitmq.NewInstance(adaptationRequestQueueHostname, adaptationRequestQueuePort, messagebrokeruser, messagebrokerpassword)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq connrecive ")
	}

	// Initiate a publisher on processing exchange
	publisher, err = rabbitmq.NewQueuePublisher(connsend, ProcessingOutcomeExchange, amqp.ExchangeDirect)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq publisher ")
	}

	// Start a consumer
	msgs, ch, err := rabbitmq.NewQueueConsumerQos(connrecive, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
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
			d.Ack(true)

			zlog.Info().Msg("received message from queue ")

			err := ProcessMessage(d.Headers)
			if JeagerStatus == true {
				processend(err, d.Headers)
			}
			connsend.Close()
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
func processend(err error, d amqp.Table) {

	if err != nil {
		tracer, closer := tracing.Init("error-process")
		defer closer.Close()
		opentracing.SetGlobalTracer(tracer)
		spCtx, ctxsuberr := ExtractWithTracer(d, ProcessTracer)
		if ctxsuberr != nil {
			zlog.Error().Err(ctxsuberr).Msg("error Jaeger Extract With Tracer")
		}
		sp := opentracing.StartSpan(
			"go-k8s-process-error",
			opentracing.FollowsFrom(spCtx),
		)
		if d["file-id"] == nil {
			TraceFileid = "nil-file-id"
		} else {
			TraceFileid = d["file-id"].(string)

		}
		sp.SetTag("file-id", TraceFileid)
		defer sp.Finish()
		ctxsubtx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
		defer cancel()
		// Update the context with the span for the subsequent reference.
		ctx = opentracing.ContextWithSpan(ctxsubtx, sp)

		span, _ := opentracing.StartSpanFromContext(ctx, "ProcessingEndError")
		defer span.Finish()
		span.LogKV("error", "true")
		span.LogKV("error-msg", err)
	} else {
		tracer, closer := tracing.Init("successful-process")
		defer closer.Close()
		opentracing.SetGlobalTracer(tracer)
		spCtx, ctxsuberr := ExtractWithTracer(d, ProcessTracer)
		if ctxsuberr != nil {
			zlog.Error().Err(ctxsuberr).Msg("error Jaeger Extract With Tracer")
		}
		sp := opentracing.StartSpan(
			"go-k8s-process-successful",
			opentracing.FollowsFrom(spCtx),
		)
		if d["file-id"] == nil {
			TraceFileid = "nil-file-id"
		} else {
			TraceFileid = d["file-id"].(string)

		}
		sp.SetTag("file-id", TraceFileid)
		defer sp.Finish()
		ctxsubtx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
		defer cancel()
		// Update the context with the span for the subsequent reference.
		ctx = opentracing.ContextWithSpan(ctxsubtx, sp)

		span, _ := opentracing.StartSpanFromContext(ctx, "ProcessingEndsuccessful")
		defer span.Finish()
		span.LogKV("error", "false")
		span.LogKV("error-msg", "false")

	}

}

func ProcessMessage(d amqp.Table) error {
	connrecive.Close()

	if JeagerStatus == true {
		if JeagerStatus == true {
			tracer, closer := tracing.Init("process")
			defer closer.Close()
			opentracing.SetGlobalTracer(tracer)
			ProcessTracer = tracer
		}

		if d["uber-trace-id"] != nil {
			// Extract the span context out of the AMQP header.
			spCtx, ctxsuberr := ExtractWithTracer(d, ProcessTracer)
			if spCtx == nil {
				fmt.Println("cpctxsub nil 1")
			}
			if ctxsuberr != nil {
				zlog.Error().Err(ctxsuberr).Msg("error Jaeger Extract With Tracer")
			}

			sp := opentracing.StartSpan(
				"go-k8s-process",
				opentracing.FollowsFrom(spCtx),
			)
			if d["file-id"] == nil {
				TraceFileid = "nil-file-id"
			} else {
				TraceFileid = d["file-id"].(string)

			}
			sp.SetTag("file-id", TraceFileid)
			defer sp.Finish()
			ctxsubtx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
			defer cancel()
			// Update the context with the span for the subsequent reference.
			ctx = opentracing.ContextWithSpan(ctxsubtx, sp)

		} else {
			if d["file-id"] == nil {
				TraceFileid = "nil-file-id"
			} else {
				TraceFileid = d["file-id"].(string)

			}
			span := ProcessTracer.StartSpan("go-k8s-process")
			span.SetTag("file-id", TraceFileid)
			defer span.Finish()
			ctx = opentracing.ContextWithSpan(context.Background(), span)
		}
	}

	if d["file-id"] == nil ||
		d["source-presigned-url"] == nil {
		return fmt.Errorf("headers value is nil")
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

	cmp, _ := d["content-management-policy"].([]byte)

	fd := rebuildexec.New(f, cmp, fileid, fileTtype, randPath)
	err := fd.Rebuild()

	log.Printf("\033[34m rebuild status is  : %s\n", fd.PrintStatus())
	if err != nil {
		if JeagerStatus == true {

			span.LogKV("error", err)
		}
		zlog.Error().Err(err).Msg("error failed to rebuild file")

	}

	status := fd.PrintStatus()
	d["rebuild-processing-status"] = status
	d["rebuild-sdk-version"] = rebuildexec.GetSdkVersion()
	d["file-outcome"] = rebuildexec.Gwoutcome(status)

	if status == "INTERNAL ERROR" {
		return
	}

	zlog.Info().Msg("file rebuilt process  successfully ")

	generateReport := ""
	if d["generate-report"] != nil {
		generateReport = d["generate-report"].(string)
	}

	if generateReport == "true" {
		report := fd.ReportFile
		if report == nil {
			zlog.Error().Msg("error report file  not found ")

		} else {
			fileExt := "report.xml"
			if fd.FileType == "zip" {
				fileExt = "report.xml.zip"
			}

			minioUploadProcess(report, fileid, fileExt, "report-presigned-url", d)
		}

	}

	file := fd.RebuiltFile

	if file == nil {
		zlog.Error().Msg("error rebuilt file not found")

	} else {
		fileExt := "rebuilt"
		if fd.FileType == "zip" {
			fileExt = "rebuilt.zip"
		}
		minioUploadProcess(file, fileid, fileExt, "clean-presigned-url", d)

	}

	gwlogFile := fd.GwLogFile
	if gwlogFile == nil {

		zlog.Error().Msg("error  GwFileLog file not found")

	} else {
		minioUploadProcess(gwlogFile, fileid, "gw.log", "gwlog-presigned-url", d)

	}

	logFile := fd.LogFile

	if logFile == nil {

		zlog.Error().Msg("error  log file not found ")

	} else {
		fileExt := "log"
		if fd.FileType == "zip" {
			fileExt = "log.zip"
		}

		minioUploadProcess(logFile, fileid, fileExt, "log-presigned-url", d)

	}

	metaDataFile := fd.Metadata
	if metaDataFile == nil {

		zlog.Error().Msg("error  metadata function")

	} else {
		minioUploadProcess(metaDataFile, fileid, "metadata.json", "metadata-presigned-url", d)

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

	urlx, err := minio.GetPresignedURLForObject(minioClient, cleanMinioBucket, filename, presignedUrlExpireIn)
	if err != nil {
		return "", err

	}

	return urlx.String(), nil

}

func minioUploadProcess(file []byte, baseName, extName, headername string, d amqp.Table) {

	minioFileId := fmt.Sprintf("%s/%s", baseName, extName)

	urlr, err := uploadMinio(file, minioFileId)
	if err != nil {
		m := fmt.Sprintf("failed to upload %s file to Minio", minioFileId)
		zlog.Info().Msg(m)
		return
	}
	m := fmt.Sprintf("%s file uploaded to minio successfully", minioFileId)

	zlog.Info().Msg(m)
	d[headername] = urlr
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
