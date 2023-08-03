/*
Copyright 2023 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
