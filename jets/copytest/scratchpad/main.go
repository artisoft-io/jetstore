package main

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Just to try some stuff

func main() {
	arrI := []interface{}{
		5,
		"something",
		[]byte("something as bytes"),
		[7]byte([]byte("7 bytes")),
		22.5,
		nil,
		time.Now().UTC(),
	}
	values := make([]string, len(arrI))
	for i := range arrI {
		if arrI[i] == nil {
			values[i] = ""	
		} else {
			fmt.Println("Got type:", reflect.TypeOf(arrI[i]), reflect.TypeOf(arrI[i]).Kind())
			switch vv := arrI[i].(type) {
			case int:
				values[i] = strconv.Itoa(vv)
			case string:
				values[i] = vv
			case []byte:
				values[i] = string(vv)
			default:
				v := reflect.ValueOf(arrI[i])
				t := reflect.TypeOf(arrI[i])
				if t.Kind() == reflect.Array {
					values[i] = fmt.Sprintf("Got a fixed array of size %v", t.Len())
					fmt.Println(values[i])
					bb := make([]byte, t.Len())
					for i := range bb {
						bb[i] = byte(v.Index(i).Interface().(uint8))
					}
					values[i] = string(bb)
				} else {
					values[i] = fmt.Sprintf("%v", arrI[i])
				}
			}
		}
	}
	fmt.Println("Here's what we have:")
	for i := range values {
		fmt.Println("  ",values[i])
	}
}