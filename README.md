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

```go
package main

import (
	sp "github.com/dalthon/stateful_proxy"

	"net/http"
)

func main() {
}
```

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
[makefile]:      Makefile
