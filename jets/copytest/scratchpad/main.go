package main

import (
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

func anonymize(space uuid.UUID, data string) string {
	encoded := uuid.NewSHA1(space, []byte(data))
	// encodedText := fmt.Sprintf("%s", encoded)
	return hex.EncodeToString(encoded[:])
}

func main() {
	space := uuid.New()
	fmt.Println("space is",space)
	encodedText := anonymize(space, "michel")
	fmt.Println("got michel","->",encodedText,"len",len(encodedText))

	encodedText = anonymize(space, "0123456789")
	fmt.Println("got 0123456789","->",encodedText,"len",len(encodedText))

	encodedText = anonymize(space, "123 main street #1")
	fmt.Println("got '123 main street #1","->",encodedText,"len",len(encodedText))

}