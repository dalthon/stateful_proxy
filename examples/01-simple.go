package main

import (
	"fmt"
	"os"
	// sp "github.com/dalthon/statefull_proxy"
)

func main() {
	fmt.Println("REDIS_URL:", os.Getenv("REDIS_URL"))
}
