package bridge

import "fmt"

// #cgo CFLAGS: -I/home/michel/projects/repos/jetstore/jets
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/jets -ljets_static 
// #cgo LDFLAGS:  -l:libc.a -lstdc++ -lm -lglog -lgflags -lsqlite3
// #cgo LDFLAGS:  -labsl_hash -labsl_city -labsl_low_level_hash -labsl_raw_hash_set
// #include "rete/jets_rete_cwrapper.h"
import "C"
import "unsafe"

type JetStore struct {
	hdl C.HJETS
}

func Delete(js JetStore) {
	C.delete_jetstore_hdl(js.hdl)
}

// func (js JetStore) Bar() {
// 	C.FooBar(js.foo)
// }

func New(rete_db_path string) JetStore {
	var js JetStore
	cstr := C.CString(rete_db_path)

	hdl, ret := C.go_create_jetstore_hdl(cstr)
	if ret != nil {
		fmt.Println("OOps ret is non zero", ret)
	}
	js.hdl = hdl
	C.free(unsafe.Pointer(&cstr)) 
	return js
}
