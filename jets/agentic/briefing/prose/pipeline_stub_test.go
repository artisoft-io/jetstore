//go:build !jets_native

package prose_test

import "testing"

// The tests over a real rdf session are in pipeline_test.go, which links libjets.so
// and is built only with -tags jets_native.
const nativeSkip = "the C++ engine's test needs -tags jets_native and the cgo override in jets/cmds/rules_unit_test/README.md"

func TestTheProseEncodingRendersFromARealSession(t *testing.T) { t.Skip(nativeSkip) }
func TestPipelineEntityShape(t *testing.T)                     { t.Skip(nativeSkip) }
func TestConditionOrderIsAProcessProperty(t *testing.T)        { t.Skip(nativeSkip) }
