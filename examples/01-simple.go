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
	"sync"
	"time"

	// sp "github.com/dalthon/stateful_proxy"

	redis "github.com/redis/go-redis/v9"
)

type HandlerFunc = func(http.ResponseWriter, *http.Request)

var deleteIfGivenFunction string = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
else
  return 0
end
`

type StatefulProxy struct {
	closed       bool
	cluster      *redis.ClusterClient
	url          string
	ctx          context.Context
	keys         map[string]bool
	lock         sync.Mutex
	partitionKey func(http.ResponseWriter, *http.Request) string
}

func New(cluster *redis.ClusterClient, stringUrl string) *StatefulProxy {
	_, err := url.Parse(stringUrl)
	if err != nil {
		panic(err)
	}

	proxy := &StatefulProxy{
		closed:       false,
		cluster:      cluster,
		url:          stringUrl,
		ctx:          context.Background(),
		keys:         map[string]bool{},
		partitionKey: func(w http.ResponseWriter, r *http.Request) string { return r.Header.Get("X-Partition-Key") },
	}

	go proxy.heartbeat(10 * time.Second)

	return proxy
}

func (proxy *StatefulProxy) Middleware(handleFunc HandlerFunc, duration time.Duration) HandlerFunc {
	var fn func(w http.ResponseWriter, r *http.Request)
	handler := func(w http.ResponseWriter, r *http.Request) {
		partitionKey := proxy.partitionKey(w, r)
		partitionUrl := proxy.partitionUrl(partitionKey, duration)

		if partitionUrl == proxy.url {
			label := proxy.partitionLabel(partitionKey)
			proxy.PartitionHeartbeat(label, duration)
			handleFunc(w, r)
			proxy.Release(label)
			return
		}

		if proxy.isRemoteUp(partitionUrl) {
			proxy.remoteCall(partitionUrl, w, r)
			return
		}

		proxy.cleanRemoteLock(partitionKey, partitionUrl)
		fn(w, r)
	}
	fn = handler

	return fn
}

func (proxy *StatefulProxy) PartitionHeartbeat(label string, duration time.Duration) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	proxy.keys[label] = true
	proxy.cluster.ExpireGT(proxy.ctx, label, duration)
}

func (proxy *StatefulProxy) Release(label string) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	delete(proxy.keys, label)
	proxy.cluster.Del(proxy.ctx, label)
}

func (proxy *StatefulProxy) Close() {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	var closingKeys []string = nil
	for key, _ := range proxy.keys {
		closingKeys = append(closingKeys, key)
		if len(closingKeys) == 100 {
			proxy.cluster.Del(proxy.ctx, closingKeys...)
			closingKeys = nil
		}
	}
	if len(closingKeys) > 0 {
		proxy.cluster.Del(proxy.ctx, closingKeys...)
	}
	proxy.closed = true
	proxy.keys = map[string]bool{}
}

func (proxy *StatefulProxy) serviceLabel() string {
	return "service:" + proxy.url
}

func (proxy *StatefulProxy) partitionLabel(key string) string {
	return "partition:" + key
}

func (proxy *StatefulProxy) heartbeat(duration time.Duration) {
	for {
		if proxy.closed {
			break
		}
		fmt.Println("heartbeat!")
		proxy.cluster.Set(proxy.ctx, proxy.serviceLabel(), 1, 3*duration)
		time.Sleep(duration)
	}
}

func (proxy *StatefulProxy) partitionUrl(partitionKey string, duration time.Duration) string {
	args := redis.SetArgs{
		Mode:    "NX",
		TTL:     duration,
		Get:     true,
		KeepTTL: false,
	}

	url, err := proxy.cluster.SetArgs(proxy.ctx, proxy.partitionLabel(partitionKey), proxy.url, args).Result()

	if err == redis.Nil {
		return proxy.url
	}

	if err != nil {
		panic(err)
	}

	return url
}

func (proxy *StatefulProxy) isRemoteUp(url string) bool {
	return true
}

func (proxy *StatefulProxy) cleanRemoteLock(partitionKey, partitionUrl string) {
}

func (proxy *StatefulProxy) remoteCall(stringUrl string, w http.ResponseWriter, r *http.Request) {
	remoteURL, err := url.Parse(stringUrl)
	if err != nil {
		panic(err)
	}
	remoteProxy := httputil.NewSingleHostReverseProxy(remoteURL)

	r.Host = remoteURL.Host
	remoteProxy.ServeHTTP(w, r)
}

func main() {
	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    strings.Split(os.Getenv("REDIS_URLS"), ","),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		PoolSize: 20,
	})

	url := "http://" + os.Getenv("HOSTNAME") + ":3000"

	proxy := New(cluster, url)

	duration := 10 * time.Second

	http.HandleFunc("/", proxy.Middleware(getRoot, duration))
	http.HandleFunc("/hello", proxy.Middleware(getHello, duration))

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
