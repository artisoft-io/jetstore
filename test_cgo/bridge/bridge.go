package bridge

// #cgo CFLAGS: -I/home/michel/projects/repos/jetstore/jets
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/jets -ljets -lsqlite3
// #cgo LDFLAGS: -labsl_city -labsl_low_level_hash -labsl_raw_hash_set
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/test_cgo -ltestc_static 
// #include "rete/jets_rete_cwrapper.h"
import "C"

import "fmt"
import "errors"
import "unsafe"

// func Random() int {
// 	fmt.Println("Caling C.my_random():")
//   ret := int(C.my_random())
// 	fmt.Println("DONE Caling C.my_random()! so far the result is",ret,". Now try C.my_random2()")
//   ret += int(C.my_random2())
// 	fmt.Println("DONE Caling C.my_random2()! Very good, the result is:",ret)
// 	fmt.Println("Now let's see if we can call a say_hello0 from jets!")
// 	C.say_hello0()
// 	fmt.Println("Well done!")
//   return ret
// }

func Hello(name string) (int, error) {
	fmt.Println("Caling C.say_hello3(name) with name ", name)

	c_name := C.CString(name)
	retc, err := C.say_hello3(c_name)
	C.free(unsafe.Pointer(c_name))
	if err != nil {
		fmt.Println("OOps got error from say_hello!!")
		return 0, errors.New("error calling Hello(name) function: " + err.Error())
	}
	// Get the result as a go type
	ret := int(retc)
	fmt.Println("OK get got: ", ret)
	return ret, nil
}

type JetStore struct {
	hdl C.HJETS
}

func LoadJetRules(rete_db_path string) (JetStore, error) {
	var js JetStore
	cstr := C.CString(rete_db_path)
	hdl := C.go_create_jetstore_hdl(cstr)
	if hdl == nil {
		fmt.Println("OOps got error in LoadJetRules!! ")
		return js, errors.New("error calling LoadJetRules()! ")
	}
	// hdl, err := C.go_create_jetstore_hdl(cstr)
	// if err != nil {
	// 	fmt.Println("OOps got error in LoadJetRules!!: " + err.Error())
	// 	return js, errors.New("error calling LoadJetRules(): " + err.Error())
	// }
	C.free(unsafe.Pointer(cstr)) 
	js.hdl = hdl
	return js, nil
}

func ReleaseJetRules(jr JetStore) error {
	retc, err := C.delete_jetstore_hdl(jr.hdl)
	if err != nil {
		fmt.Println("OOps got error in ReleaseJetRules!!")
		return errors.New("error calling ReleaseJetRules() function: " + err.Error())
	}
	ret := int(retc)
	if ret != 0 {
		fmt.Println("OOps got error in c++ ReleaseJetRules!!")
		return errors.New("error calling ReleaseJetRules() in c++ function: " + string(ret))
	}
	return nil
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

