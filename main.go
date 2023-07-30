package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	chiprometheus "github.com/766b/chi-prometheus"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/go-chi/render"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	appName       = "myapp"
	spannerString = os.Getenv("SPANNER_STRING")
	redisHost     = os.Getenv("REDIS_HOST")
	servicePort   = os.Getenv("PORT")
	projectId     = os.Getenv("GOOGLE_CLOUD_PROJECT")
	rev           = os.Getenv("K_REVISION")
)

var (
	topicName      = os.Getenv("TOPIC_NAME")
	authHeaderName = os.Getenv("AUTH_HEADER")
	pubsubClient   *pubsub.Client
)

type Serving struct {
	Client GameUserOperation
}

type User struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

func main() {

	ctx := context.Background()
	tp, err := newTracer(projectId)
	if err != nil {
		log.Fatal(err)
	}
	defer tp.Shutdown(ctx)

	p, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		log.Fatal(err)
	}
	pubsubClient = p
	defer pubsubClient.Close()

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

	defer client.sc.Close()
	defer rdb.Close()

	s := Serving{
		Client: client,
	}

	oplog := httplog.LogEntry(context.Background())
	/* jsonify logging */
	httpLogger := httplog.NewLogger(appName, httplog.Options{JSON: true, LevelFieldName: "severity", Concise: true})

	/* exporter for prometheus */
	m := chiprometheus.NewMiddleware(appName)

	r := chi.NewRouter()
	// r.Use(middleware.Throttle(8))
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(httplog.RequestLogger(httpLogger))
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(headerAuth)

	r.Use(m)
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/ping", s.pingPong)

	r.Route("/api", func(t chi.Router) {
		t.Get("/user_id/{user_id:[a-z0-9-.]+}", s.getUserItems)
		t.Post("/user/{user_name:[a-z0-9-.]+}", s.createUser)
		t.Put("/user_id/{user_id:[a-z0-9-.]+}/{item_id:[a-z0-9-.]+}", s.addItemToUser)
	})

	if err := http.ListenAndServe(":"+servicePort, r); err != nil {
		oplog.Err(err)
	}

}

var errorRender = func(w http.ResponseWriter, r *http.Request, httpCode int, err error) {
	render.Status(r, httpCode)
	render.JSON(w, r, map[string]interface{}{"ERROR": err.Error()})
}

func (s Serving) getUserItems(w http.ResponseWriter, r *http.Request) {

	userID := chi.URLParam(r, "user_id")
	ctx := r.Context()

	ctx, span := otel.Tracer("main").Start(ctx, "getUserItems.root")
	span.SetAttributes(attribute.String("server", "getUserItems"))
	defer span.End()

	oplog := httplog.LogEntry(ctx)
	// projects/PROJECT_ID/traces/TRACE_ID
	trace := fmt.Sprintf("projects/%s/traces/%s", projectId, span.SpanContext().TraceID().String())
	oplog.Info().Str("trace", trace).Str("spanId", span.SpanContext().SpanID().String()).Msg("test")

	results, err := s.Client.userItems(ctx, w, userID)
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}

	// publish log, just for test
	if topicName != "" {
		p := map[string]interface{}{"id": userID, "rev": rev}
		publishLog(pubsubClient, topicName, p)
	}

	render.JSON(w, r, results)
}

func (s Serving) createUser(w http.ResponseWriter, r *http.Request) {
	userId, _ := uuid.NewRandom()
	userName := chi.URLParam(r, "user_name")
	ctx := r.Context()

	ctx, span := otel.Tracer("main").Start(ctx, "createUser.root")
	span.SetAttributes(attribute.String("server", "createUser"))
	defer span.End()

	err := s.Client.createUser(ctx, w, userParams{userID: userId.String(), userName: userName})
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, User{
		Id:   userId.String(),
		Name: userName,
	})
}

func (s Serving) addItemToUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	itemID := chi.URLParam(r, "item_id")
	ctx := r.Context()

	ctx, span := otel.Tracer("main").Start(ctx, "addItemToUser.root")
	span.SetAttributes(attribute.String("server", "addItemToUser"))
	defer span.End()

	err := s.Client.addItemToUser(ctx, w, userParams{userID: userID}, itemParams{itemID: itemID})
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, map[string]string{})
}

func (s Serving) pingPong(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusOK)
	render.PlainText(w, r, "Pong\n")
}

func headerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHeaderName != "" {
			auth := r.Header.Get(authHeaderName)
			if auth == "" {
				// w.WriteHeader(http.StatusForbidden)
				log.Printf("Forbidden request info: %+v", r)
				http.Error(w, "You're NOT permitted to enter here", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
