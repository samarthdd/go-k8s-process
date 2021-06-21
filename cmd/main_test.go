package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/k8-proxy/dockertest/v3"
	"github.com/k8-proxy/dockertest/v3/docker"
	"github.com/k8-proxy/go-k8s-process/tracing"
	"github.com/k8-proxy/k8-go-comm/pkg/minio"
	"github.com/k8-proxy/k8-go-comm/pkg/rabbitmq"
	"github.com/opentracing/opentracing-go"
	zlog "github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

// var body = "body test"
var TestMQTable amqp.Table
var endpoint string
var ResourceMQ *dockertest.Resource
var ResourceMinio *dockertest.Resource
var ResourceJG *dockertest.Resource

var poolMq *dockertest.Pool
var poolMinio *dockertest.Pool
var poolJG *dockertest.Pool

var secretsstring string
var thisServiceName = "test-procc"

func jaegerserver() {
	var errpool error

	poolJG, errpool = dockertest.NewPool("")
	if errpool != nil {
		log.Fatalf("Could not connect to docker: %s", errpool)
	}
	opts := dockertest.RunOptions{
		Repository: "jaegertracing/all-in-one",
		Tag:        "latest",

		PortBindings: map[docker.Port][]docker.PortBinding{
			"5775/udp": {{HostPort: "5775"}},
			"6831/udp": {{HostPort: "6831"}},
			"6832/udp": {{HostPort: "6832"}},
			"5778/tcp": {{HostPort: "5778"}},
			"16686":    {{HostPort: "16686"}},
			"14268":    {{HostPort: "14268"}},
			"9411":     {{HostPort: "9411"}},
		},
	}
	resource, err := poolJG.RunWithOptions(&opts)
	if err != nil {
		log.Fatalf("Could not start resource: %s", err.Error())
	}
	ResourceJG = resource

}
func rabbitserver() {
	var errpool error

	poolMq, errpool = dockertest.NewPool("")
	if errpool != nil {
		log.Fatalf("Could not connect to docker: %s", errpool)
	}
	opts := dockertest.RunOptions{
		Repository: "rabbitmq",
		Tag:        "latest",
		Env: []string{
			"host=root",
		},
		ExposedPorts: []string{"5672"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"5672": {
				{HostPort: "5672"},
			},
		},
	}
	resource, err := poolMq.RunWithOptions(&opts)
	if err != nil {
		log.Fatalf("Could not start resource: %s", err.Error())
	}
	ResourceMQ = resource

}

// Minio server
func minioserver() {
	var errpool error

	minioAccessKey = secretsstring
	minioSecretKey = secretsstring

	poolMinio, errpool = dockertest.NewPool("")
	if errpool != nil {
		log.Fatalf("Could not connect to docker: %s", errpool)
	}

	options := &dockertest.RunOptions{
		Repository: "minio/minio",
		Tag:        "latest",
		Cmd:        []string{"server", "/data"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"9000/tcp": {{HostPort: "9000"}},
		},
		Env: []string{fmt.Sprintf("MINIO_ACCESS_KEY=%s", minioAccessKey), fmt.Sprintf("MINIO_SECRET_KEY=%s", minioSecretKey)},
	}

	resource, err := poolMinio.RunWithOptions(options)
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}
	ResourceMinio = resource

	endpoint = fmt.Sprintf("localhost:%s", resource.GetPort("9000/tcp"))
	minioEndpoint = endpoint

	cleanMinioBucket = os.Getenv("MINIO_CLEAN_BUCKET")
	if err := poolMinio.Retry(func() error {
		url := fmt.Sprintf("http://%s/minio/health/live", endpoint)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code not OK")
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

}

func TestProcessMessage(t *testing.T) {
	log.Println("[√] start test")
	JeagerStatus = true

	ProcessingRequestExchange = "processing-request-exchange"
	ProcessingRequestRoutingKey = "processing-request"
	ProcessingRequestQueueName = "processing-request"

	ProcessingOutcomeExchange = "processing-outcome-exchange"
	ProcessingOutcomeRoutingKey = "processing-outcome"
	ProcessingOutcomeQueueName = "processing-outcome-queue"
	adaptationRequestQueueHostname = "localhost"
	adaptationRequestQueuePort = "5672"
	JeagerStatusEnv = "true"

	// get env secrets
	var errstring error

	secretsstring, errstring = GenerateRandomString(8)
	if errstring != nil {
		log.Fatalf("[x] GenerateRandomString error: %s", errstring)

		return
	}

	minioAccessKey = secretsstring
	minioSecretKey = secretsstring
	if JeagerStatus == true {
		jaegerserver()
		log.Println("[√] create Jaeger  successfully")
		tracer, closer := tracing.Init(thisServiceName)
		defer closer.Close()
		opentracing.SetGlobalTracer(tracer)
		ProcessTracer = tracer
		log.Println("[√] create Jaeger ProcessTracer successfully")

		tracer, closer = tracing.Init("outMsgs")
		defer closer.Close()
		opentracing.SetGlobalTracer(tracer)
		ProcessTracer = tracer
		log.Println("[√] create Jaeger ProcessTracer2 successfully")
	}

	rabbitserver()
	log.Println("[√] create AMQP  successfully")

	minioserver()
	log.Println("[√] create minio  successfully")

	time.Sleep(40 * time.Second)

	var err error
	// Get a connrecive //rabbitmq

	connrecive, err = amqp.Dial("amqp://localhost:5672")
	if err != nil {
		log.Fatalf("[x] AMQP connrecive error: %s", err)
	}
	connsend, err = amqp.Dial("amqp://localhost:5672")
	if err != nil {
		log.Fatalf("[x] AMQP connrecive error: %s", err)
	}
	log.Println("[√] AMQP Connected successfully")
	defer connrecive.Close()
	// now we can instantiate minio client

	minioClient, err = minio.NewMinioClient(minioEndpoint, minioAccessKey, minioSecretKey, false)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start minio client ")

	}
	if err != nil {
		log.Fatalf("[x] Failed to create minio client error: %s", err)
		return
	}
	log.Println("[√] create minio client successfully")
	// Start a consumer
	_, ch, err := rabbitmq.NewQueueConsumerQos(connrecive, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
	if err != nil {
		log.Fatalf("[x] could not start  AdpatationReuquest consumer error: %s", err)
	}
	log.Println("[√] create start  Adpatation Reuquest consumer successfully")
	defer ch.Close()
	defer ch.Close()

	_, outChannel, err := rabbitmq.NewQueueConsumerQos(connsend, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
	if err != nil {
		log.Fatalf("[x] Failed to create consumer error: %s", err)

	}

	log.Println("[√] create  consumer successfully")
	publisher, err = rabbitmq.NewQueuePublisher(connsend, ProcessingOutcomeExchange, amqp.ExchangeDirect)
	if err != nil {
		log.Println("[x] create  consumer publisher")

	}
	log.Println("[√] create  consumer publisher")

	defer outChannel.Close()

	sourceMinioBucket := "source"
	cleanMinioBucket = "clean"

	err = createBucketIfNotExist(sourceMinioBucket)
	if err != nil {
		log.Fatalf("[x] sourceMinioBucket createBucketIfNotExist error: %s", err)
	}
	log.Println("[√] create source Minio Bucket successfully")

	err = createBucketIfNotExist(cleanMinioBucket)
	if err != nil {
		log.Fatalf("[x] cleanMinioBucket createBucketIfNotExist error: %s", err)

	}
	log.Println("[√] create clean Minio Bucket successfully")
	fn := "unittest.pdf"
	//fullpath := "http://localhost:9000"
	//fnrebuild := fmt.Sprintf("rebuild-%s", fn)
	// Upload the source file to Minio and Get presigned URL
	presignedUrlExpireIn := time.Minute * 10
	sourcePresignedURL, err := minio.UploadAndReturnURL(minioClient, sourceMinioBucket, fn, presignedUrlExpireIn)
	if err != nil {
		t.Errorf("error uploading file from minio : %s", err)
	}

	table := amqp.Table{
		"file-id":              "unittest",
		"source-presigned-url": sourcePresignedURL.String(),
		"generate-report":      "true",
		"request-mode":         "respmod",
	}

	var d amqp.Delivery
	d.Headers = table
	TraceFileid = d.Headers["file-id"].(string)
	span := ProcessTracer.StartSpan("ProcessFile")
	span.SetTag("send-msg", TraceFileid)
	defer span.Finish()

	ctx = opentracing.ContextWithSpan(context.Background(), span)
	if err := Inject(span, table); err != nil {
		t.Errorf("ProcessMessage = %d; want nil", err)

	}
	t.Run("ProcessMessage", func(t *testing.T) {
		result := ProcessMessage(table)
		if result != nil {

			t.Errorf("ProcessMessage(amqp.Delivery) = %d; want nil", result)

		} else {
			log.Println("[√] ProcessMessage successfully")

		}

	})

	t.Run("main", func(t *testing.T) {

		done := make(chan bool)

		go func() {
			t.Run("main", func(t *testing.T) {
				main()
			})
		}()

		time.Sleep(10 * time.Second)
		close(done)
		<-done
	})
	t.Run("Test_uploadMinio", func(t *testing.T) {
		data, err := ioutil.ReadFile("unittest.pdf")
		if err != nil {
			t.Errorf("uploadMinio() error %s ", err)
		}
		type args struct {
			file     []byte
			filename string
		}
		tests := []struct {
			name string
			args args
		}{
			{
				"uploadMinio",
				args{data, "unittest.pdf"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := uploadMinio(tt.args.file, tt.args.filename)
				if err != nil {
					t.Errorf("uploadMinio()  %s", err)
				}

			})
		}
	})

	t.Run("Test_minioUploadProcess", func(t *testing.T) {
		tablefile := amqp.Table{
			"file-id":               "unittest.pdf",
			"clean-presigned-url":   "http://localhost:9000",
			"rebuilt-file-location": "./reb.pdf",
			"reply-to":              "replay",
		}
		data, err := ioutil.ReadFile("unittest.pdf")
		if err != nil {
			t.Errorf("uploadMinio() error %s ", err)
		}
		type args struct {
			file       []byte
			baseName   string
			extName    string
			headername string
			d          amqp.Table
		}
		tests := []struct {
			name string
			args args
		}{
			{
				"minioUploadProcess",
				args{data, "fileidtest", "pdf", "header", tablefile},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				minioUploadProcess(tt.args.file, tt.args.baseName, tt.args.extName, tt.args.headername, tt.args.d)
			})
		}
	})

	// When you're done, kill and remove the container
	if err = poolMq.Purge(ResourceMQ); err != nil {
		fmt.Printf("Could not purge resource: %s", err)
	}
	if err = poolMinio.Purge(ResourceMinio); err != nil {
		fmt.Printf("Could not purge resource: %s", err)
	}
	if JeagerStatus == true {

		if err = poolJG.Purge(ResourceJG); err != nil {
			fmt.Printf("Could not purge resource: %s", err)
		}
	}

}
func GenerateRandomString(n int) (string, error) {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}

func TestInject(t *testing.T) {
	tableout := amqp.Table{
		"file-id":               "fileidtest",
		"clean-presigned-url":   "http://localhost:9000",
		"rebuilt-file-location": "./reb.pdf",
		"reply-to":              "replay",
	}
	tracer, closer := tracing.Init(thisServiceName)
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)
	ProcessTracer = tracer

	var d amqp.Delivery
	d.Headers = tableout
	TraceFileid = d.Headers["file-id"].(string)
	span := ProcessTracer.StartSpan("ProcessFile")
	span.SetTag("send-msg", TraceFileid)
	defer span.Finish()

	ctx = opentracing.ContextWithSpan(context.Background(), span)

	type args struct {
		span opentracing.Span
		hdrs amqp.Table
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"Inject",
			args{span, tableout},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Inject(tt.args.span, tt.args.hdrs); (err != nil) != tt.wantErr {
				t.Errorf("Inject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtract(t *testing.T) {
	tableout := amqp.Table{
		"file-id":               "fileidtest",
		"clean-presigned-url":   "http://localhost:9000",
		"rebuilt-file-location": "./reb.pdf",
		"reply-to":              "replay",
	}
	tracer, closer := tracing.Init(thisServiceName)
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)
	ProcessTracer = tracer

	var d amqp.Delivery
	d.Headers = tableout
	TraceFileid = d.Headers["file-id"].(string)
	span := ProcessTracer.StartSpan("ProcessFile")
	span.SetTag("send-msg", TraceFileid)
	defer span.Finish()

	ctx = opentracing.ContextWithSpan(context.Background(), span)
	Inject(span, d.Headers)
	spanctx, _ := Extract(d.Headers)
	type args struct {
		hdrs amqp.Table
	}
	tests := []struct {
		name    string
		args    args
		want    opentracing.SpanContext
		wantErr bool
	}{
		{
			"Extract",
			args{tableout},
			spanctx,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Extract(tt.args.hdrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_amqpHeadersCarrier_ForeachKey(t *testing.T) {
	tableout := amqp.Table{
		"file-id":               "fileidtest",
		"clean-presigned-url":   "http://localhost:9000",
		"rebuilt-file-location": "./reb.pdf",
		"reply-to":              "replay",
	}
	c := amqpHeadersCarrier(tableout)

	tetsc := c

	type args struct {
		handler func(key, val string) error
	}
	han := args{
		handler: func(key string, val string) error {
			return nil
		},
	}

	tests := []struct {
		name    string
		c       amqpHeadersCarrier
		args    args
		wantErr bool
	}{
		{
			"amqpHeadersCarrier",
			tetsc,
			han,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.ForeachKey(tt.args.handler); (err != nil) != tt.wantErr {
				t.Errorf("amqpHeadersCarrier.ForeachKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractWithTracer(t *testing.T) {
	tableout := amqp.Table{
		"file-id":               "fileidtest",
		"clean-presigned-url":   "http://localhost:9000",
		"rebuilt-file-location": "./reb.pdf",
		"reply-to":              "replay",
	}
	tracer, closer := tracing.Init(thisServiceName)
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)
	ProcessTracer = tracer

	var d amqp.Delivery
	d.Headers = tableout
	TraceFileid = d.Headers["file-id"].(string)
	span := ProcessTracer.StartSpan("ProcessFile")
	span.SetTag("send-msg", TraceFileid)
	defer span.Finish()

	ctx = opentracing.ContextWithSpan(context.Background(), span)
	Inject(span, d.Headers)
	spanctx, _ := ExtractWithTracer(d.Headers, ProcessTracer)

	type args struct {
		hdrs   amqp.Table
		tracer opentracing.Tracer
	}
	tests := []struct {
		name    string
		args    args
		want    opentracing.SpanContext
		wantErr bool
	}{
		{
			"ExtractWithTracer",
			args{tableout, ProcessTracer},
			spanctx,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractWithTracer(tt.args.hdrs, tt.args.tracer)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractWithTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractWithTracer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createBucketIfNotExist(bucketName string) error {
	if JeagerStatus == true && ctx != nil {
		span, _ := opentracing.StartSpanFromContext(ctx, "createBucketIfNotExist")
		defer span.Finish()
		span.LogKV("event", "createBucket")
	}
	exist, err := minio.CheckIfBucketExists(minioClient, bucketName)
	if err != nil {

		return fmt.Errorf("error creating source  minio bucket : %s", err)
	}
	if !exist {

		err := minio.CreateNewBucket(minioClient, bucketName)
		if err != nil {
			return fmt.Errorf("error could not create minio bucket : %s", err)
		}
	}
	return nil
}

func Test_processend(t *testing.T) {
	tablefile := amqp.Table{
		"file-id":               "fileidtest",
		"clean-presigned-url":   "http://localhost:9000",
		"rebuilt-file-location": "./reb.pdf",
		"reply-to":              "replay",
	}
	tablenofile := amqp.Table{
		"clean-presigned-url":   "http://localhost:9000",
		"rebuilt-file-location": "./reb.pdf",
		"reply-to":              "replay",
	}

	err1 := errors.New("file-id:not found")

	type args struct {
		err error
		d   amqp.Table
	}
	tests := []struct {
		name string
		args args
	}{
		{
			"end-with-error",
			args{err1, tablenofile},
		},
		{
			"end-successfully",
			args{nil, tablefile},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processend(tt.args.err, tt.args.d)
		})
	}
}
