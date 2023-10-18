package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sp "github.com/dalthon/stateful_proxy"

	redis "github.com/redis/go-redis/v9"
)

func main() {
	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    strings.Split(os.Getenv("REDIS_URLS"), ","),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		PoolSize: 20,
	})

	url := "http://" + os.Getenv("HOSTNAME") + ":3000"

	heartbeatDuration := 10 * time.Second
	proxy := sp.New(cluster, url, heartbeatDuration, &sp.Config{})

	config := &sp.Config{
		Duration: 30 * time.Second,
	}

	http.HandleFunc("/", proxy.Middleware(getRoot, config))
	http.HandleFunc("/hello", proxy.Middleware(getHello, config))

	fmt.Println("Starting ", url)
	if err := http.ListenAndServe(":3000", nil); err != nil {
		panic(err)
	}

	fmt.Println("Finished!")
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got / request\n")
	io.WriteString(w, "This is my website!\n")
}

func getHello(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got /hello request\n")
	io.WriteString(w, "Hello, HTTP!\n")
}
