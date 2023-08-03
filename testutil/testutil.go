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
package testutil

import (
	"context"
	"log"
	"os"
	"regexp"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
)

func InitData(ctx context.Context, db string, files []string) error {
	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		log.Fatalf("Invalid database id %s", db)
	}

	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer adminClient.Close()

	var createTablesSQL []string
	for _, file := range files {
		sqlData, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		createTablesSQL = append(createTablesSQL, string(sqlData))
	}

	op, err := adminClient.CreateDatabase(ctx, &adminpb.CreateDatabaseRequest{
		Parent:          matches[1],
		CreateStatement: "CREATE DATABASE `" + matches[2] + "`",
		ExtraStatements: createTablesSQL,
	})
	if err != nil {
		return err
	}
	if _, err := op.Wait(ctx); err != nil {
		return err
	}
	return nil
}

// func InitData(ctx context.Context, db string, files []string) error {
func MakeData(ctx context.Context, db string, files []string) error {
	dataClient, err := spanner.NewClient(ctx, db)
	if err != nil {
		return err
	}
	defer dataClient.Close()

	for _, file := range files {
		sqlData, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		_, err = dataClient.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
			stmt := spanner.Statement{
				SQL: string(sqlData),
			}
			_, err := txn.Update(ctx, stmt)
			if err != nil {
				return err
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func DropData(ctx context.Context, db string) error {

	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		log.Fatalf("Invalid database id %s", db)
	}
	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer adminClient.Close()

	err = adminClient.DropDatabase(ctx, &adminpb.DropDatabaseRequest{
		Database: db,
	})
	if err != nil {
		return err
	}
	return nil

}
