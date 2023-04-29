package main

import (
	"context"
	"log"

	"encoding/json"

	"cloud.google.com/go/pubsub"
)

func publishLog(client *pubsub.Client, topicName string, data map[string]interface{}, async bool) error {

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return err
	}

	message := &pubsub.Message{
		Data: jsonData,
	}

	topic := client.Topic(topicName)
	ctx := context.Background()
	if async {
		go func() {
			res := topic.Publish(ctx, message)
			log.Printf("Async %+v\n", res)
		}()
	} else {
		res := topic.Publish(ctx, message)
		log.Printf("Sync %+v\n", res)
	}

	return nil
}
