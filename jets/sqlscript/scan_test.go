package sqlscript

import (
	"errors"
	"testing"
)

func TestScannerTokens(t *testing.T) {
	src := "SELECT e'a\\'b', \"x;y\" -- c;\n/* a /* b */ c */ $t$ ; $t$, $1, foo$bar, E'z'\n;"
	type tk struct {
		kind  TokenKind
		text  string
		inner string
		line  int
	}
	want := []tk{
		{Ident, "SELECT", "SELECT", 1},
		{EscapeString, `e'a\'b'`, `a\'b`, 1},
		{QuotedIdent, `"x;y"`, "x;y", 1},
		{LineComment, "-- c;", "-- c;", 1},
		{BlockComment, "/* a /* b */ c */", "/* a /* b */ c */", 2},
		{DollarString, "$t$ ; $t$", " ; ", 2},
		{Other, "$", "$", 2},
		{Other, "1", "1", 2},
		{Ident, "foo$bar", "foo$bar", 2},
		{EscapeString, "E'z'", "z", 2},
		{Other, ";", ";", 3},
	}
	sc := NewScanner(src)
	var got []tk
	for {
		tok, ok, err := sc.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if tok.Text != src[tok.Offset:tok.Offset+len(tok.Text)] {
			t.Fatalf("token %q is not at offset %d", tok.Text, tok.Offset)
		}
		if tok.Kind == Other && (tok.Text == " " || tok.Text == "," || tok.Text == "\n") {
			continue
		}
		got = append(got, tk{tok.Kind, tok.Text, tok.Inner(), tok.Line})
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tokens %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Skip goes on after a construct the caller reads itself, counting the
// newlines it passes.
func TestScannerSkip(t *testing.T) {
	sc := NewScanner("{{a\nb}} x")
	tok, _, _ := sc.Next()
	if tok.Text != "{" {
		t.Fatalf("first token %q", tok.Text)
	}
	sc.Skip(len("{{a\nb}}") - 1)
	sc.Next() // the space
	tok, ok, _ := sc.Next()
	if !ok || tok.Text != "x" || tok.Line != 2 {
		t.Fatalf("after Skip: %+v %v", tok, ok)
	}
}

func TestScannerUnterminated(t *testing.T) {
	sc := NewScanner("SELECT 1;\nSELECT $q$ never")
	for {
		_, ok, err := sc.Next()
		if err != nil {
			var ue *UnterminatedError
			if !errors.As(err, &ue) || ue.Line != 2 || ue.Construct != "dollar-quoted string $q$" {
				t.Fatalf("got %v", err)
			}
			if _, ok, _ := sc.Next(); ok {
				t.Fatal("the scanner went on after an error")
			}
			return
		}
		if !ok {
			t.Fatal("no error")
		}
	}
}
