package stateful_proxy

import (
	"context"
	"sync"
	"time"

	"net/http"
	"net/http/httputil"
	"net/url"

	redis "github.com/redis/go-redis/v9"
)

type handlerFunc = func(http.ResponseWriter, *http.Request)

var deleteIfGivenScript string = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
else
  return 0
end
`

var defaultDuration time.Duration = 30 * time.Second

type Config struct {
	PartitionKey func(http.ResponseWriter, *http.Request) string
	Duration     time.Duration
}

type StatefulProxy struct {
	closed  bool
	cluster *redis.ClusterClient
	url     string
	ctx     context.Context
	keys    map[string]bool
	lock    sync.Mutex
	config  *Config
}

func New(cluster *redis.ClusterClient, stringUrl string, heartbeatDuration time.Duration, config *Config) *StatefulProxy {
	_, err := url.Parse(stringUrl)
	if err != nil {
		panic(err)
	}

	if config == nil {
		config = &Config{}
	}

	proxy := &StatefulProxy{
		closed:  false,
		cluster: cluster,
		url:     stringUrl,
		ctx:     context.Background(),
		keys:    map[string]bool{},
		config:  config,
	}

	go proxy.heartbeat(heartbeatDuration)

	return proxy
}

func (proxy *StatefulProxy) Middleware(handleFunc handlerFunc, config *Config) handlerFunc {
	if config == nil {
		config = &Config{}
	}

	var fn func(w http.ResponseWriter, r *http.Request)
	handler := func(w http.ResponseWriter, r *http.Request) {
		partitionKey := proxy.partitionKey(config, w, r)
		duration := proxy.duration(config)
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

func (proxy *StatefulProxy) remoteServiceLabel(url string) string {
	return "service:" + url
}

func (proxy *StatefulProxy) partitionLabel(key string) string {
	return "partition:" + key
}

func (proxy *StatefulProxy) heartbeat(duration time.Duration) {
	for {
		if proxy.closed {
			break
		}
		proxy.cluster.Set(proxy.ctx, proxy.serviceLabel(), 1, 3*duration)
		time.Sleep(duration)
	}
}

func (proxy *StatefulProxy) partitionKey(config *Config, w http.ResponseWriter, r *http.Request) string {
	if config.PartitionKey != nil {
		return config.PartitionKey(w, r)
	}

	if proxy.config.PartitionKey != nil {
		return proxy.config.PartitionKey(w, r)
	}

	return r.Header.Get("X-Partition-Key")
}

func (proxy *StatefulProxy) duration(config *Config) time.Duration {
	if config.Duration != time.Duration(0) {
		return config.Duration
	}

	if proxy.config.Duration != time.Duration(0) {
		return proxy.config.Duration
	}

	return defaultDuration
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
	_, err := proxy.cluster.Get(proxy.ctx, proxy.remoteServiceLabel(url)).Result()
	return err != redis.Nil
}

func (proxy *StatefulProxy) cleanRemoteLock(partitionKey, partitionUrl string) {
	_, err := proxy.cluster.Eval(
		proxy.ctx,
		deleteIfGivenScript,
		[]string{proxy.partitionLabel(partitionKey)},
		partitionUrl,
	).Result()

	if err != nil {
		panic(err)
	}
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
