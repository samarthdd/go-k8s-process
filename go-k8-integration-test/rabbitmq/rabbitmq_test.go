package rabbitmq

import (
	"log"
	"os"
	"testing"

	"github.com/k8-proxy/k8-go-comm/pkg/rabbitmq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type Rabbitmq struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

type RabbitmqTestSuite struct {
	suite.Suite
	queue *Rabbitmq
}

func (s *RabbitmqTestSuite) TestprocessmsgMessage() {
	s.T().Run("K8 process massge", func(t *testing.T) {
		endpoint := "play.min.io"

		accessKeyID := os.Getenv("TEST_MINIO_ACCESS_KEY")
		secretAccessKey := os.Getenv("TEST_MINIO_SECRET_KEY")
		// Initialize minio client object.

		// Get a connection //rabbitmq
		conn, err := amqp.Dial("amqp://rabbitmq:5672")
		if err != nil {
			log.Fatalf("%s", err)
		}
		defer conn.Close()

		// Initiate a publisher on processing exchange
		publisher, err = rabbitmq.NewQueuePublisher(conn, ProcessingOutcomeExchange, amqp.ExchangeDirect)
		if err != nil {
			log.Fatalf("%s", err)
		}
		minioClient, _ = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
			Secure: true,
		})
		if err != nil {
			log.Fatalf("%s", err)
		}

		// Producer
		headers := make(amqp.Table)
		headers["file-id"] = "544"
		headers["source-presigned-url"] = "http://www.orimi.com/pdf-test.pdf"
		headers["rebuilt-file-location"] = "/test"
		var d amqp.Delivery
		d.ConsumerTag = "test-tag"
		d.Headers = headers
		d.ContentType = "application/pdf"
		d.Body = []byte("Hello")
		err = ProcessMessage(d)
		assert.NoError(t, err, "Publish() error:\nwant  nil\ngot  %v", err)
	})
}
func TestRabbitmqTestSuite(t *testing.T) {
	suite.Run(t, new(RabbitmqTestSuite))
}
