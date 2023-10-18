# Stateful Proxy

[API Reference][api-reference]

This project is just a middleware that works as a reverse proxy to route
request to another instance of the same application.

This may be useful for steteful applications that may hold session in memory
or any other reason that may benefit due to performance or whatever reason
that routing to a same instance may be a good thing.

It relies on a Redis Cluster to store routes.

## Install

```sh
go get github.com/dalthon/stateful_proxy
```

## Usage

This is the example provided at [examples/01-simple.go][example]:

```go
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

	proxy := sp.New(cluster, url)

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
```

## TODO

* TEST IT!
* Add config

## Contributing

Pull requests and issues are welcome! I'll try to review them as soon as I can.

This project is quite simple and its [Makefile][makefile] is quite useful to do
whatever you may need. Run `make help` for more info.

To run tests, run `make test`.

To run test with coverage, run `make cover`.

To run a full featured example available at [examples/01-simple.go][example], run
`make example-01`.

## License

This project is released under the [MIT License][license]

[api-reference]: https://pkg.go.dev/github.com/dalthon/stateful_proxy
[example]:       examples/01-simple.go
[license]:       https://opensource.org/licenses/MIT
[makefile]:      Makefile
