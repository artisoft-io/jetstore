package rdf

import "testing"

// Rough costs of the number comparisons (rules_benchmark Q-68): a double
// against an int, two different doubles, two equal doubles.
var compareSink bool

func BenchmarkCompareDoubleGtInt(b *testing.B) {
	l, r := F(12.5), I(0)
	for range b.N {
		compareSink = l.GT(r).Bool()
	}
}

func BenchmarkCompareDoubleEqDouble(b *testing.B) {
	l, r := F(12.5), F(7.25)
	for range b.N {
		compareSink = l.EQ(r).Bool()
	}
}

func BenchmarkCompareDoubleEqSameDouble(b *testing.B) {
	l, r := F(12.5), F(12.5)
	for range b.N {
		compareSink = l.EQ(r).Bool()
	}
}
