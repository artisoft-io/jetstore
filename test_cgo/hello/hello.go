package main

// import "fmt"

import (
	"fmt"
	"github.com/artisoft-io/jetstore/test_cgo/bridge"
)

func main() {
	fmt.Println("Hello again!")
	// fmt.Println("My lucky lucky number today:",bridge.Random())
	fmt.Println("OK SO NOW let's try our fancy say_hello: Hello()")
	name := "ArtiSoft"
	ret, err := bridge.Hello(name)
	if err != nil {
		fmt.Println("We got a Problem")
	}
	fmt.Println("Super! we got:", ret)
	jr_name := "jetrule_rete_test.db"
	fmt.Println("Loading JetRules db:", jr_name)
	js, err := bridge.LoadJetRules(jr_name)
	if err != nil {
		fmt.Println("We got a Problem:", err)
	}
	fmt.Println("LOADED!!", jr_name)
	fmt.Println("Releasing MetaStore")
	err = bridge.ReleaseJetRules(js)
	if err != nil {
		fmt.Println("We got a Problem while releasing jetrules:", err)
	}
	fmt.Println("Got it!!")

}