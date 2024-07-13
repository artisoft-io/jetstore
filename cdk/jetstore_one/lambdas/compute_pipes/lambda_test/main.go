package main

import (
	"github.com/artisoft-io/jetstore/jets/copytest/scratchpad"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler)
}
func handler() error {
	return scratchpad.Scratchpad()
}
