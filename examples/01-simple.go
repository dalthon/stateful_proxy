package main

import (
	"fmt"
	"time"

	redis "github.com/go-redis/redis"
)

func main() {
	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{"redis:10001", "redis:10002", "redis:10003"},
	})

	cluster.Set("123", "456", 20*time.Second).Result()
	result, err := cluster.Get("123").Result()
	if err != nil {
		panic(err)
	}

	fmt.Println("Result:", result)
}
