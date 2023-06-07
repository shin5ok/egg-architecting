package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"encoding/json"

	"cloud.google.com/go/spanner"
	"github.com/go-redis/redis"
	"google.golang.org/api/iterator"
)

type GameUserOperation interface {
	createUser(context.Context, io.Writer, userParams) error
	addItemToUser(context.Context, io.Writer, userParams, itemParams) error
	userItems(context.Context, io.Writer, string) ([]map[string]interface{}, error)
}

type userParams struct {
	userID   string
	userName string
}

type itemParams struct {
	itemID string
}

type dbClient struct {
	sc    *spanner.Client
	cache *redis.Client
}

var baseItemSliceCap = 100

func newClient(ctx context.Context, dbString string, redisClient *redis.Client) (dbClient, error) {

	client, err := spanner.NewClient(ctx, dbString)
	if err != nil {
		return dbClient{}, err
	}

	return dbClient{
		sc:    client,
		cache: redisClient,
	}, nil
}

// create a user
func (d dbClient) createUser(ctx context.Context, w io.Writer, u userParams) error {

	_, err := d.sc.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		sqlToUsers := `INSERT users (user_id, name, created_at, updated_at)
		  VALUES (@userID, @userName, @timestamp, @timestamp)`
		t := time.Now().Format("2006-01-02 15:04:05")
		params := map[string]interface{}{
			"userID":    u.userID,
			"userName":  u.userName,
			"timestamp": t,
		}
		stmtToUsers := spanner.Statement{
			SQL:    sqlToUsers,
			Params: params,
		}
		rowCountToUsers, err := txn.Update(ctx, stmtToUsers)
		_ = rowCountToUsers
		if err != nil {
			return err
		}

		return nil
	})
	return err
}

// add item specified item_id to specific user
func (d dbClient) addItemToUser(ctx context.Context, w io.Writer, u userParams, i itemParams) error {

	_, err := d.sc.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		sqlToUsers := `INSERT user_items (user_id, item_id, created_at, updated_at)
		  VALUES (@userID, @itemID, @timestamp, @timestamp)`
		t := time.Now().Format("2006-01-02 15:04:05")
		params := map[string]interface{}{
			"userID":    u.userID,
			"itemId":    i.itemID,
			"timestamp": t,
		}
		stmtToUsers := spanner.Statement{
			SQL:    sqlToUsers,
			Params: params,
		}
		rowCountToUsers, err := txn.Update(ctx, stmtToUsers)
		_ = rowCountToUsers
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

// get items the user has
func (d dbClient) userItems(ctx context.Context, w io.Writer, userID string) ([]map[string]interface{}, error) {

	key := fmt.Sprintf("userItems_%s", userID)
	data, err := d.cache.Get(key).Result()

	if err != nil {
		log.Println(key, "Error", err)
	} else {
		results := []map[string]interface{}{}
		err := json.Unmarshal([]byte(data), &results)
		if err != nil {
			log.Println(err)
		}
		log.Println(key, "from cache")
		return results, nil
	}

	txn := d.sc.ReadOnlyTransaction()
	defer txn.Close()
	sql := `select users.name,items.item_name,user_items.item_id
		from user_items join items on items.item_id = user_items.item_id join users on users.user_id = user_items.user_id
		where user_items.user_id = @user_id`
	stmt := spanner.Statement{
		SQL: sql,
		Params: map[string]interface{}{
			"user_id": userID,
		},
	}

	iter := txn.Query(ctx, stmt)
	defer iter.Stop()

	results := make([]map[string]interface{}, 0, baseItemSliceCap)
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return results, err
		}
		var userName string
		var itemNames string
		var itemIds string
		if err := row.Columns(&userName, &itemNames, &itemIds); err != nil {
			return results, err
		}

		results = append(results,
			map[string]interface{}{
				"user_name": userName,
				"item_name": itemNames,
				"item_id":   itemIds,
			})

	}

	jsonedResults, _ := json.Marshal(results)
	err = d.cache.Set(key, string(jsonedResults), 10*time.Second).Err()
	if err != nil {
		log.Println(err)
	}

	return results, nil
}
