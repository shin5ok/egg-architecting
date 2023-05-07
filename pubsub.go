package main

import (
	"context"
	"fmt"
	"log"

	"encoding/json"

	"cloud.google.com/go/pubsub"
)

func publishLog(client *pubsub.Client, topicName string, data map[string]interface{}) error {

	if topicName == "" {
		return fmt.Errorf("topic name is empty")
	}

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
	go func() {
		res := topic.Publish(ctx, message)
		log.Printf("Async %+v\n", res)
	}()

	return nil
}
