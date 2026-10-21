package rulesbench

import (
	"testing"
	"time"
)

func TestRenderCellKeepsTypeAndOrder(t *testing.T) {
	day := time.Date(2025, 8, 14, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{"a\tb", `string:"a\tb"`},
		{1, `"int:1"`},
		{1.0, "float64:1"},
		{0.1, "float64:0.1"},
		{day, "time:2025-08-14T00:00:00Z UTC"},
		{[]any{"b", 2, nil}, `[string:"b", "int:2", null]`},
	} {
		if got := renderCell(c.in); got != c.want {
			t.Errorf("renderCell(%#v) = %s, want %s", c.in, got, c.want)
		}
	}
}
