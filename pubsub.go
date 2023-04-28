package main

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"
)

func publishLog(client *pubsub.Client, topicName string, data []byte, async bool) error {

	message := &pubsub.Message{
		Data: data,
	}

	topic := client.Topic(topicName)
	if async {
		go func() {
			res := topic.Publish(context.Background(), message)
			log.Printf("%+v\n", res)
		}()
	} else {
		res := topic.Publish(context.Background(), message)
		log.Printf("%+v\n", res)
	}

	return nil
}
