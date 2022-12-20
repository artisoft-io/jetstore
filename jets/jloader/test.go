package main

import (
    "encoding/json"
    "fmt"
    "strings"
	"reflect"
)

func main2() {
    const jsonStream = `
                [
                    {"Name": "Ed", "Value": 5, "Text": ["knock1", "knock2"]},
                    {"Name": "Sam", "Text": "Who's there?"},
                    {"Name": "Ed", "Text": "Go fmt."},
                    {"Name": "Sam", "Text": "Go fmt who?"},
                    {"Name": "Ed", "Text": "Go fmt yourself!"}
                ]
            `
    type Message struct {
        Name, Text string
    }
    dec := json.NewDecoder(strings.NewReader(jsonStream))

    // read open bracket
    t, err := dec.Token()
    if err != nil {
			fmt.Println(err)
			panic(err)
		}
	
	switch v := t.(type) { 
    default:
        fmt.Printf("unexpected type %T", v)
	case json.Delim:
		if fmt.Sprintf("%v", t) == "[" {
			fmt.Println("Hey got start of array!")
		}
        fmt.Println("Got Delim", t)
    case string:
        fmt.Println("Got string", t)
    } 
	
    fmt.Printf("%T: %v\n", t, t)

    // while the array contains values
    for dec.More() {
	
		t, err := dec.Token()
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		if reflect.ValueOf(t).Kind() == reflect.String {
			fmt.Println("It's a string!")
		}
		fmt.Printf("%T: %v\n", t, t)


//        var m Message
        // decode an array value (Message)
//        err := dec.Decode(&m)
//        if err != nil {
	// fmt.Println(err)
	// panic(err)
//        }

//        fmt.Printf("%v: %v\n", m.Name, m.Text)
    }

    // read closing bracket
    t, err = dec.Token()
    if err != nil {
			fmt.Println(err)
			panic(err)
			}
    fmt.Printf("%T: %v\n", t, t)

}