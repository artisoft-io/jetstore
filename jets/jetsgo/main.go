package main // import "github.com/artisoft-io/jetstore/jets/jetstore-go"

import (
	"fmt"
	"github.com/artisoft-io/jetstore/jets/bridge"
)


func main() {
	fmt.Println("Hello, world.")
	// ret, err := bridge.SayHello()
	// if err != nil {
	// 	fmt.Println("WHAT, got error from SayHello:",err)
	// } else {
	// 	fmt.Println("The result is:",ret)
	// }
	bridge.Say0Hello()
	// js := bridge.New("test_data/jetrule_rete_test.db")
	// fmt.Println("Loaded!")
	// bridge.Delete(js)
	fmt.Println("Done!!")
}
