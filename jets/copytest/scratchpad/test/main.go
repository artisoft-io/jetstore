package main

import (
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/copytest/scratchpad"
)

func main() {
	if err := scratchpad.Scratchpad(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
