package main

import (
	"context"
	"log"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

func main() {
	// Create a s3 client
	s3Client, err := awsi.NewS3Client()
	if err != nil {
		log.Panicln(err)
	}

	err = awsi.MultiPartCopy(context.TODO(), s3Client, "bucket.jetstore.io", 
	"jetstore/input/client=metlife/year=2023/month=9/day=6/object_type=USIClaim/obfuscated_orig.csv", 
	"bucket.jetstore.io", "jetstore/output/copy/obf-test.csv")
	if err != nil {
		log.Panicln(err)
	}
}