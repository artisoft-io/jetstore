package main  // import "github.com/artisoft-io/jetstore/jets/jetstore-go"

import "fmt"

// // #cgo LDFLAGS: -L${SRCDIR} -ljets_rete -l:libc.a -lstdc++ -lm
// // #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/bazel-out/k8-fastbuild/bin/external/com_github_google_glog -lglog
// // #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/bazel-out/k8-fastbuild/bin/external/com_github_gflags_gflags -lgflags
// // #include <stdlib.h>
// // #include "../rete/jets_rete_cwrapper.h"
// // typedef void* HJETS;
// import "C"
// import "unsafe"


func main() {
	fmt.Println("Hello, world.")
	fmt.Println("Loading...")
}
