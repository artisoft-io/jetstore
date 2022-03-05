package bridge

// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/jets -ljets -lsqlite3
// #cgo LDFLAGS: -labsl_city -labsl_low_level_hash -labsl_raw_hash_set
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/test_cgo -ltestc_static 
// #include "cwrapper.h"
import "C"

import "fmt"

func Random() int {
	fmt.Println("Caling C.my_random():")
  ret := int(C.my_random())
	fmt.Println("DONE Caling C.my_random()! so far the result is",ret,". Now try C.my_random2()")
  ret += int(C.my_random2())
	fmt.Println("DONE Caling C.my_random2()! Very good, the result is:",ret)
	fmt.Println("Now let's see if we can call a say_hello0 from jets!")
	C.say_hello0()
	fmt.Println("Well done!")
  return ret
}

// func Seed(i int) {
//     C.srandom(C.uint(i))
// }
// PREVIOUS
// #cgo CFLAGS:  -I/home/michel/projects/repos/jetstore/test_cgo
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/test_cgo -ltestc_static 
// #cgo LDFLAGS:  -llibc -lstdc++ -lm -lpthread
// #cgo LDFLAGS:  -labsl_hash -labsl_city -labsl_low_level_hash -labsl_raw_hash_set
// #cgo LDFLAGS: -lglog -lgflags -lsqlite3
// #include "../testc/cwrapper.h"

