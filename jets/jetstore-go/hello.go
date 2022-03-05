package main  // import "github.com/artisoft-io/jetstore/jets/jetstore-go"

import "fmt"

// #cgo LDFLAGS: -L${SRCDIR} -ljets_rete -l:libc.a -lstdc++ -lm
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/bazel-out/k8-fastbuild/bin/external/com_github_google_glog -lglog
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/bazel-out/k8-fastbuild/bin/external/com_github_gflags_gflags -lgflags
// #include <stdlib.h>
// #include "jets_rete_cwrapper.h"
// typedef void* HJETS;
import "C"
import "unsafe"

type JetStore struct {
	hdl C.HJETS
}

func (js JetStore) Delete() {
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


func main() {
	fmt.Println("Hello, world.")
	fmt.Println("Loading...")
}
