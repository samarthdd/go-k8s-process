package main

import (
	"log"
	"testing"

	"github.com/NeowayLabs/wabbit"
	"github.com/NeowayLabs/wabbit/amqptest"
	"github.com/NeowayLabs/wabbit/amqptest/server"
	"github.com/streadway/amqp"
)

var (
	uri          = "amqp://guest:guest@localhost:5672/%2f"
	queueName    = "test-queue"
	exchange     = "test-exchange"
	exchangeType = "direct"
	body         = "body test"
	reliable     = true
)

type Delivery struct {
	data          []byte
	headers       wabbit.Option
	tag           uint64
	consumerTag   string
	originalRoute string
	messageId     string
	channel       wabbit.Channel
}

func TestProcessMessage(t *testing.T) {
	JeagerStatus = false
	fakeServer := server.NewServer("amqp://localhost:5672/%2f")
	fakeServer.Start()

	// Connects opens an AMQP connrecive from the credentials in the URL.
	//	publish(uri, queueName, exchange, exchangeType, body, reliable)
	log.Println("[-] Connecting to", uri)
	connrecive, err := amqptest.Dial("amqp://localhost:5672/%2f") // now it works =D

	if err != nil {
		log.Fatalf("[x] AMQP connrecive error: %s", err)
	}

	log.Println("[√] Connected successfully")

	channel, err := connrecive.Channel()

	if err != nil {
		log.Fatalf("[x] Failed to open a channel: %s", err)
	}

	defer channel.Close()
	headers := make(amqp.Table)
	headers["file-id"] = "544"
	headers["source-presigned-url"] = "https://s23.q4cdn.com/202968100/files/doc_downloads/test.pdf"
	headers["rebuilt-file-location"] = "./rbulid"
	var d amqp.Delivery
	d.ConsumerTag = "test-tag"
	d.Headers = headers
	d.ContentType = "text/plain"
	d.Body = []byte(body)
	t.Run("ProcessMessage", func(t *testing.T) {
		ProcessMessage(d.Headers)

	})

	type testSample struct {
		data          []byte
		headers       wabbit.Option
		tag           uint64
		consumerTag   string
		originalRoute string
		messageId     string
		channel       wabbit.Channel
	}
	sampleTable := []testSample{
		{
			data: []byte("teste"),
			headers: wabbit.Option{
				"contentType": "binary/fuzz",
			},
			tag: uint64(23473824),
		},
		{
			data: []byte("teste"),
			headers: wabbit.Option{
				"contentType": "binary/fuzz",
			},
			tag: uint64(23473824),
		},
	}

	for _, sample := range sampleTable {

		t.Run("ProcessMessage", func(t *testing.T) {

			if sample.headers["contentType"].(string) != "binary/fuzz" {
				t.Errorf("Headers value is nil")

			}

		})
	}

}
func publishMessage(body string, exchange string, queue wabbit.Queue, channel wabbit.Channel) error {
	return channel.Publish(
		exchange,     // exchange
		queue.Name(), // routing key
		[]byte(body),
		wabbit.Option{
			"deliveryMode": 2,
			"contentType":  "text/plain",
		})
}

func confirmOne(confirms <-chan wabbit.Confirmation) {
	log.Printf("[-] Waiting for confirmation of one publishing")

	if confirmed := <-confirms; confirmed.Ack() {
		log.Printf("[√] Confirmed delivery with delivery tag ")

	} else {
		log.Printf("[x] Failed delivery of delivery tag: ")

	}
}
func publish(uri string, queueName string, exchange string, exchangeType string, body string, reliable bool) {
	log.Println("[-] Connecting to", uri)
	connrecive, err := amqptest.Dial("amqp://localhost:5672/%2f") // now it works =D

	if err != nil {
		log.Fatalf("[x] AMQP connrecive error: %s", err)
	}

	log.Println("[√] Connected successfully")

	channel, err := connrecive.Channel()

	if err != nil {
		log.Fatalf("[x] Failed to open a channel: %s", err)
	}

	defer channel.Close()

	log.Println("[-] Declaring Exchange", exchangeType, exchange)
	err = channel.ExchangeDeclare(exchange, exchangeType, nil)

	if err != nil {
		log.Fatalf("[x] Failed to declare exchange: %s", err)
	}
	log.Println("[√] Exchange", exchange, "has been declared successfully")

	log.Println("[-] Declaring queue", queueName, "into channel")
	queue, err := declareQueue(queueName, channel)

	if err != nil {
		log.Fatalf("[x] Queue could not be declared. Error: %s", err.Error())
	}
	log.Println("[√] Queue", queueName, "has been declared successfully")

	err = channel.QueueBind(queueName, queueName, exchange, nil)

	if err != nil {
		log.Fatalf("[x] QueueBind could not be bind. Error: %s", err.Error())
		return
	}

	log.Println("[√] QueueBind", queueName, "has been bind successfully")

	deliveries, err := channel.Consume(
		queue.Name(), // name
		"test-tag",   // consumerTag,
		wabbit.Option{
			"noAck":     false,
			"exclusive": false,
			"noLocal":   false,
			"noWait":    false,
		},
	)
	if err != nil {
		log.Fatalf("[x] Failed to deliveries. Error: %s", err.Error())
		return
	}
	log.Println("[√] deliveries", queue.Name(), "has deliveries bind successfully")
	if reliable {
		log.Printf("[-] Enabling publishing confirms.")
		if err := channel.Confirm(false); err != nil {
			log.Fatalf("[x] Channel could not be put into confirm mode: %s", err)
		}

		confirms := channel.NotifyPublish(make(chan wabbit.Confirmation, 1))

		defer confirmOne(confirms)
	}

	log.Println("[-] Sending message to queue:", queueName, "- exchange:", exchange)
	log.Println("\t", body)

	err = publishMessage(body, exchange, queue, channel)

	if err != nil {
		log.Fatalf("[x] Failed to publish a message. Error: %s", err.Error())
	}

	data := <-deliveries
	if string(data.Body()) != "body test" {
		log.Fatalf("Failed to publish message to specified route")

	}

	log.Printf(
		"got %dB delivery: [%v] %q",
		len(data.Body()),
		data.DeliveryTag(),
		data.Body(),
	)

}

func declareQueue(queueName string, channel wabbit.Channel) (wabbit.Queue, error) {
	return channel.QueueDeclare(
		queueName,
		wabbit.Option{
			"durable":    true,
			"autoDelete": false,
			"exclusive":  false,
			"noWait":     false,
		},
	)
}
