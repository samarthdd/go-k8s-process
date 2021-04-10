package main

import (
	"testing"

	"github.com/NeowayLabs/wabbit"
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
