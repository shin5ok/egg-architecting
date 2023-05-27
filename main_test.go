/*
This is just for local test with Spanner Emulator
Note: Before running this test, run spanner emulator and create an instance as "test-instance"
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis"
	gonanoid "github.com/matoous/go-nanoid"
	"github.com/shin5ok/egg7-architecting/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	fakeDbString = os.Getenv("SPANNER_STRING") + genStr()
	fakeServing  Serving

	itemTestID = "d169f397-ba3f-413b-bc3c-a465576ef06e"
	userTestID string

	noCleanup = func() bool {
		return os.Getenv("NO_CLEANUP") != ""
	}()
)

func genStr() string {
	var src = "abcdefghijklmnopqrstuvwxyz09123456789"
	id, err := gonanoid.Generate(src, 6)
	if err != nil {
		panic(err)
	}
	return string(id) + time.Now().Format("2006-01-02")
}

func init() {
	/*
	 for local emulator:
	 export SPANNER_STRING=projects/your-project-id/instances/test-instance/databases/game-dummy
	*/
	log.Println("Creating " + fakeDbString)

	if match, _ := regexp.MatchString("^projects/your-project-id/", fakeDbString); match {
		os.Setenv("SPANNER_EMULATOR_HOST", "localhost:9010")
	}

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:        redisHost,
		Password:    "",
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
		DialTimeout: 1 * time.Second,
	})

	client, err := newClient(ctx, spannerString, rdb)
	if err != nil {
		log.Fatal(err)
	}

	fakeServing = Serving{
		Client: client,
	}

	schemaFiles, _ := filepath.Glob("schemas/*_ddl.sql")
	if err := testutil.InitData(ctx, fakeDbString, schemaFiles); err != nil {
		log.Fatal(err)
	}

	dmlFiles, _ := filepath.Glob("schemas/*_dml.sql")
	if err := testutil.MakeData(ctx, fakeDbString, dmlFiles); err != nil {
		log.Fatal(err)
	}
}

func Test_run(t *testing.T) {

	req, err := http.NewRequest("GET", "/", nil)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.pingPong)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}

}

func Test_createUser(t *testing.T) {

	path := "test-user"
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_name", path)

	r := &http.Request{}
	req, err := http.NewRequestWithContext(r.Context(), "POST", "/api/user/"+path, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.createUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}
	var u User
	json.Unmarshal(rr.Body.Bytes(), &u)
	userTestID = u.Id

}

// This test depends on Test_createUser
func Test_addItemUser(t *testing.T) {

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_id", userTestID)
	ctx.URLParams.Add("item_id", itemTestID)

	r := &http.Request{}
	uriPath := fmt.Sprintf("/api/user_id/%s/%s", userTestID, itemTestID)
	req, err := http.NewRequestWithContext(r.Context(), "PUT", uriPath, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.addItemToUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}

}

func Test_getUserItems(t *testing.T) {

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_id", userTestID)

	r := &http.Request{}
	uriPath := fmt.Sprintf("/api/user_id/%s", userTestID)
	req, err := http.NewRequestWithContext(r.Context(), "GET", uriPath, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.getUserItems)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s, Request: %+v", http.StatusOK, rr.Code, rr.Body, req)
	}
}

func Test_cleaning(t *testing.T) {
	t.Cleanup(
		func() {
			if noCleanup {
				t.Log("skip cleanup")
				return
			}
			ctx := context.Background()
			if err := testutil.DropData(ctx, fakeDbString); err != nil {
				t.Error(err)
			}
		},
	)
}
