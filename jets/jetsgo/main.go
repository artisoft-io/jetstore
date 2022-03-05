package main // import "github.com/artisoft-io/jetstore/jets/jetstore-go"

import (
	"fmt"
	"github.com/artisoft-io/jetstore/jets/bridge"
)


func main() {
	fmt.Println("Hello, world.")
	js := bridge.New("test_data/jetrule_rete_test.db")
	fmt.Println("Loaded!")
	bridge.Delete(js)
	fmt.Println("Done!!")
}
