//go:build !jets_native

package graphdump

import "testing"

// TestNativeSessionDump is in native_test.go, which links libjets.so and is built
// only with -tags jets_native.
func TestNativeSessionDump(t *testing.T) {
	t.Skip("the C++ engine's test needs -tags jets_native and the cgo override in jets/cmds/rules_unit_test/README.md")
}
