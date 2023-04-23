package main

import (
	"context"

	"github.com/go-redis/redis"
)

type SimpleCache struct {
	RedisHost   string
	RedisPort   string
	RedisClient *redis.Client
}

type SimpleCacher interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Store(ctx context.Context, key string, value []byte) error
}

func (v *SimpleCache) New() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     v.RedisHost + ":" + v.RedisPort,
		Password: "",
		DB:       0,
	})
	v.RedisClient = rdb
	return rdb
}

func (v *SimpleCache) Get(ctx context.Context, key string) ([]byte, error) {
	panic("not implemented") // TODO: Implement
}

func (v *SimpleCache) Store(ctx context.Context, key string, value []byte) error {
	panic("not implemented") // TODO: Implement
}
