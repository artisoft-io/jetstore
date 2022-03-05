package main

// import "fmt"

import (
	"fmt"
	"github.com/artisoft-io/jetstore/test_cgo/bridge"
)

func main() {
	fmt.Println("Hello again!")
	fmt.Println("My lucky lucky number today:",bridge.Random())
}