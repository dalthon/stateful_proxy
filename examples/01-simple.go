package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type HandlerFunc = func(http.ResponseWriter, *http.Request)

type StatefulProxy struct {
	cluster *redis.ClusterClient
	url     *url.URL
}

func New() *StatefulProxy {
	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    strings.Split(os.Getenv("REDIS_URLS"), ","),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		PoolSize: 20,
	})

	url, err := url.Parse("http://" + os.Getenv("HOSTNAME") + ":3000")
	if err != nil {
		panic(err)
	}

	return &StatefulProxy{
		cluster: cluster,
		url:     url,
	}
}

func (proxy *StatefulProxy) Middleware(handleFunc HandlerFunc, duration time.Duration) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		partitionKey := r.Header.Get("X-Partition-Key")

		thisUrl := proxy.url.String()
		args := redis.SetArgs{
			Mode:    "NX",
			TTL:     duration,
			Get:     true,
			KeepTTL: false,
		}

		ctx := context.Background()
		url, err := proxy.cluster.SetArgs(ctx, partitionKey, thisUrl, args).Result()
		if err != nil && err != redis.Nil {
			panic(err)
		}

		if url == thisUrl || err == redis.Nil {
			proxy.cluster.ExpireGT(ctx, partitionKey, duration)
			handleFunc(w, r)
			proxy.cluster.Del(ctx, partitionKey)
			return
		}

		proxy.remoteCall(url, w, r)
	}
}

func (proxy *StatefulProxy) remoteCall(stringURL string, w http.ResponseWriter, r *http.Request) {
	remoteURL, err := url.Parse(stringURL)
	if err != nil {
		panic(err)
	}
	remoteProxy := httputil.NewSingleHostReverseProxy(remoteURL)

	r.Host = remoteURL.Host
	remoteProxy.ServeHTTP(w, r)
}

func main() {
	proxy := New()

	duration := 10 * time.Second

	http.HandleFunc("/", proxy.Middleware(getRoot, duration))
	http.HandleFunc("/hello", proxy.Middleware(getHello, duration))

	fmt.Println("Starting...")
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
