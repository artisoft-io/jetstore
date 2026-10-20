//go:build jets_native

package graphdump

// The C++ engine's half: built only with -tags jets_native, because the package
// then links libjets.so. Build against a libjets.so of this commit with the cgo
// override in jets/cmds/rules_unit_test/README.md:
//
//	CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" LD_LIBRARY_PATH=$LIB \
//	  go test -count=1 -tags jets_native ./jets/rulesbench/graphdump/

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_native_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

func init() {
	factories[rulestest.EngineNative] = jetrules_native_adaptor.NewJetRulesFactory
}

// The C++ engine gives the dump the Go engine gives, on rules that mint subjects:
// so the meta graph is left out the same way, and UUIDs and their jets:key texts
// are renamed the same way.
func TestNativeSessionDump(t *testing.T) {
	if os.Getenv(childEnv) == "" {
		rerunOverTestWs(t)
		return
	}
	compileTestWs(t)
	native, err := jetrules_native_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, loopingRuleFile, false)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Release()
	native.JetResources()
	first, withMeta := loopingDumps(t, native)
	checkLoopingDump(t, first, withMeta)
	second, _ := loopingDumps(t, native)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two C++ sessions differ:\n%s\n---\n%s", strings.Join(first, "\n"), strings.Join(second, "\n"))
	}

	goEngine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, loopingRuleFile, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goEngine.Release()
	goEngine.JetResources()
	onGo, _ := loopingDumps(t, goEngine)
	if !reflect.DeepEqual(first, onGo) {
		t.Errorf("the engines differ:\nC++:\n%s\nGo:\n%s", strings.Join(first, "\n"), strings.Join(onGo, "\n"))
	}
}
