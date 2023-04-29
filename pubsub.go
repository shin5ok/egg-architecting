package main

import (
	"context"
	"log"

	"encoding/json"

	"cloud.google.com/go/pubsub"
)

func publishLog(client *pubsub.Client, topicName string, data map[string]interface{}, async bool) error {

	jsonData, _ := json.Marshal(data)

	message := &pubsub.Message{
		Data: jsonData,
	}

	topic := client.Topic(topicName)
	if async {
		go func() {
			res := topic.Publish(context.Background(), message)
			log.Printf("async %+v\n", res)
		}()
	} else {
		res := topic.Publish(context.Background(), message)
		log.Printf("sync %+v\n", res)
	}

	return nil
}
