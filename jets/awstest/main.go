package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	bucketName      string
	objectPrefix    string
	objectDelimiter string
	maxKeys         int
)

// Need the following env variables:
//	AWS_ACCESS_KEY_ID
//	AWS_SECRET_ACCESS_KEY
//	AWS_REGION

func init() {
	flag.StringVar(&bucketName, "bucket", "", "The `name` of the S3 bucket to list objects from.")
	flag.StringVar(&objectPrefix, "prefix", "", "The optional `object prefix` of the S3 Object keys to list.")
	flag.StringVar(&objectDelimiter, "delimiter", "",
		"The optional `object key delimiter` used by S3 List objects to group object keys.")
	flag.IntVar(&maxKeys, "max-keys", 0,
		"The maximum number of `keys per page` to retrieve at once.")
}

// Lists all objects in a bucket using pagination
func main() {
	flag.Parse()
	if len(bucketName) == 0 {
		flag.PrintDefaults()
		log.Fatalf("invalid parameters, bucket name required")
	}

	// Load the SDK's configuration from environment and shared config, and
	// create the client with this.
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load SDK configuration, %v", err)
	}

	client := s3.NewFromConfig(cfg)

	// Set the parameters based on the CLI flag inputs.
	params := &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	}
	if len(objectPrefix) != 0 {
		params.Prefix = &objectPrefix
	}
	if len(objectDelimiter) != 0 {
		params.Delimiter = &objectDelimiter
	}

	// Create the Paginator for the ListObjectsV2 operation.
	p := s3.NewListObjectsV2Paginator(client, params, func(o *s3.ListObjectsV2PaginatorOptions) {
		if v := int32(maxKeys); v != 0 {
			o.Limit = v
		}
	})

	// Iterate through the S3 object pages, printing each object returned.
	var i int
	log.Println("Objects:")
	for p.HasMorePages() {
		i++

		// Next Page takes a new context for each page retrieval. This is where
		// you could add timeouts or deadlines.
		page, err := p.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get page %v, %v", i, err)
		}

		// Log the objects found
		for _, obj := range page.Contents {
			fmt.Println("Object:", *obj.Key)
		}
	}

	// Download object using a download manager to a temp file
	f, err := os.CreateTemp("", "jetstore")
	if err != nil {
		log.Fatalf("failed to open temp file: %v", err)
	}
	// log.Printf("Temp file name: %s", f.Name())
	defer os.Remove(f.Name())

	// Download the object
	downloader := manager.NewDownloader(client)
	key := "usi/input/client=SBBC/object_type=USIClaim/USIClaim/obfuscated_orig.csv"
	nsz, err := downloader.Download(context.TODO(), f, &s3.GetObjectInput{Bucket: &bucketName, Key: &key})
	if err != nil {
		log.Fatalf("failed to download file: %v", err)
	}
	fmt.Println("downloaded", nsz,"bytes")

	// Print out the object
	f.Seek(0, 0)
	scanner := bufio.NewScanner(f)
	fsz := 0
	for scanner.Scan() {
		// fmt.Println(scanner.Text())
		fsz += len(scanner.Text())
	}
	fmt.Println("file content length", fsz)

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println("That's it for now!")

}
