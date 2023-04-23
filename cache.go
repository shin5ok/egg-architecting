package main

import (
	"github.com/go-redis/redis"
)

type SimpleCache struct {
	RedisHost string
	RedisPort string
}

type SimpleCacher interface {
	Get(key string) ([]byte, error)
	Store(key, value string) error
}

func (v SimpleCache) New() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     v.RedisHost + ":" + v.RedisPort,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return rdb
}

func (v SimpleCache) Get(key string) (string, error) {
	panic("not implemented") // TODO: Implement
}

func (v SimpleCache) Store(key string, value string) error {
	panic("not implemented") // TODO: Implement
}
