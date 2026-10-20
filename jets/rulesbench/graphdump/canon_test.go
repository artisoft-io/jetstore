package graphdump

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
)

func res(n string) harness.Term { return harness.Term{Kind: harness.TermResource, Text: n} }
func bn(k int) harness.Term {
	return harness.Term{Kind: harness.TermBlank, Text: fmt.Sprintf("BN%d", k)}
}
func txt(s string) harness.Term { return harness.Term{Kind: harness.TermText, Text: s} }
func num(i int) harness.Term    { return harness.Term{Kind: harness.TermInt, Text: fmt.Sprint(i)} }
func dbl(x float64) harness.Term {
	return harness.Term{Kind: harness.TermDouble, Text: FormatDouble(x), Num: x}
}
func tr(s, p, o harness.Term) harness.SnapshotTriple { return harness.SnapshotTriple{S: s, P: p, O: o} }

func uuidOf(r *rand.Rand) string {
	b := make([]byte, 16)
	r.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// claims builds two claims, each with an entity made by create_entity with an
// empty key (a UUID resource and its jets:key text) and a multi-lookup with two
// rows, named from r.
func claims(r *rand.Rand) []harness.SnapshotTriple {
	var out []harness.SnapshotTriple
	key := r.Intn(1000)
	for i, c := range []string{"hc:claim1", "hc:claim2"} {
		e := uuidOf(r)
		out = append(out,
			tr(res(c), res("hc:amount"), dbl(100.25+float64(i))),
			tr(res(c), res("hc:entity"), res(e)),
			tr(res(e), res("jets:key"), txt(e)),
			tr(res(e), res("rdf:type"), res("hc:Match")),
			tr(res(e), res("hc:rank"), num(i)),
		)
		subject := res(fmt.Sprintf("jets:lookup:multi:K%d", i))
		out = append(out,
			tr(res("jets:cache:lk:T"), res("jets:lookup_multi_rows"), subject),
			tr(res(c), res("hc:rows"), subject))
		for j := 0; j < 2; j++ {
			row := bn(key + 10*i + j)
			out = append(out,
				tr(subject, res("jets:lookup_multi_rows"), row),
				tr(row, res("lk:code"), txt(fmt.Sprintf("C%d%d", i, j))),
				tr(row, res("jets:row_id"), num(10*i+j)))
		}
	}
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func TestEqualUpToNamingGivesTheSameDump(t *testing.T) {
	want := canonicalLines(claims(rand.New(rand.NewSource(1))))
	for seed := int64(2); seed < 12; seed++ {
		got := canonicalLines(claims(rand.New(rand.NewSource(seed))))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("seed %d: dump differs\n got: %s\nwant: %s", seed, strings.Join(got, "\n      "), strings.Join(want, "\n      "))
		}
	}
	t.Logf("the dump:\n%s", strings.Join(want, "\n"))
	for _, l := range want {
		if uuidV4.MatchString(l) || strings.Contains(l, "BN") || strings.Contains(l, "jets:lookup:multi:") {
			t.Errorf("an engine name survived: %s", l)
		}
		if _, _, _, ok := ParseLine(l); !ok {
			t.Errorf("line does not parse: %q", l)
		}
	}
	// The UUID's jets:key text is the same minted node as the resource.
	var key string
	for _, l := range want {
		if strings.Contains(l, "\tjets:key\t") {
			key = l
			break
		}
	}
	s, _, o, _ := ParseLine(key)
	if s.Kind != "minted" || o.Kind != "minted_text" || s.Value != o.Value {
		t.Errorf("jets:key line %q: want the subject's own label as text", key)
	}
}

// The two engines' lookup bookkeeping: Go caches (jets:cache:<T> jets:lookup_row
// jets:lookup:<key>), C++ (<T> jets:lookup_row jets:<T>:<key>); a C++ multi-lookup
// caches with jets:lookup_row where Go uses jets:lookup_multi_rows.
func TestLookupNamingIsEngineNeutral(t *testing.T) {
	goSide := []harness.SnapshotTriple{
		tr(res("jets:cache:lk:T"), res("jets:lookup_row"), res("jets:lookup:123")),
		tr(res("jets:lookup:123"), res("lk:zip"), txt("123")),
		tr(res("jets:lookup:123"), res("jets:row_id"), num(7)),
		tr(res("hc:claim1"), res("hc:geo"), res("jets:lookup:123")),
		tr(res("jets:cache:lk:M"), res("jets:lookup_multi_rows"), res("jets:lookup:multi:9")),
		tr(res("jets:lookup:multi:9"), res("jets:lookup_multi_rows"), bn(0)),
		tr(bn(0), res("lk:v"), num(1)),
	}
	cppSide := []harness.SnapshotTriple{
		tr(res("lk:T"), res("jets:lookup_row"), res("jets:lk:T:123")),
		tr(res("jets:lk:T:123"), res("lk:zip"), txt("123")),
		tr(res("jets:lk:T:123"), res("jets:row_id"), num(7)),
		tr(res("hc:claim1"), res("hc:geo"), res("jets:lk:T:123")),
		tr(res("lk:M"), res("jets:lookup_row"), res("jets:lk:M:9")),
		tr(res("jets:lk:M:9"), res("jets:lookup_multi_rows"), bn(41)),
		tr(bn(41), res("lk:v"), num(1)),
	}
	a, b := canonicalLines(goSide), canonicalLines(cppSide)
	onlyA, onlyB := minus(a, b), minus(b, a)
	if len(onlyA) != 1 || len(onlyB) != 1 {
		t.Fatalf("want only the multi-lookup cache line to differ\n go: %s\ncpp: %s", strings.Join(a, "\n     "), strings.Join(b, "\n     "))
	}
	if !strings.HasPrefix(onlyA[0], "jets:cache:lk:M\tjets:lookup_multi_rows\t_:") ||
		!strings.HasPrefix(onlyB[0], "jets:cache:lk:M\tjets:lookup_row\t_:") ||
		strings.Split(onlyA[0], "\t")[2] != strings.Split(onlyB[0], "\t")[2] {
		t.Errorf("the cache lines differ by more than their predicate: %q, %q", onlyA[0], onlyB[0])
	}
}

func minus(a, b []string) []string {
	in := map[string]bool{}
	for _, l := range b {
		in[l] = true
	}
	var out []string
	for _, l := range a {
		if !in[l] {
			out = append(out, l)
		}
	}
	return out
}

// A difference renames the minted nodes near it and no others.
func TestLabelsAreLocal(t *testing.T) {
	cluster := func(c string, extra bool) []harness.SnapshotTriple {
		out := []harness.SnapshotTriple{
			tr(res(c), res("hc:rows"), bn(len(c))),
			tr(bn(len(c)), res("lk:v"), txt(c)),
		}
		if extra {
			out = append(out, tr(bn(len(c)), res("lk:w"), num(2)))
		}
		return out
	}
	a := canonicalLines(append(cluster("hc:a", false), cluster("hc:bbbb", false)...))
	b := canonicalLines(append(cluster("hc:a", true), cluster("hc:bbbb", false)...))
	for _, l := range minus(a, b) {
		if strings.Contains(l, "hc:bbbb") {
			t.Errorf("a change on hc:a renamed hc:bbbb's row: %s", l)
		}
	}
	if len(minus(b, a)) != 3 {
		t.Errorf("want hc:a's three lines to differ, got %v", minus(b, a))
	}
}

// Nodes refinement cannot tell apart are numbered, the same way whatever their
// engine names: interchangeable ones directly, linked ones by individualisation.
func TestTiedNodes(t *testing.T) {
	graph := func(r *rand.Rand) []harness.SnapshotTriple {
		k := r.Perm(4)
		out := []harness.SnapshotTriple{
			// interchangeable: two rows with the same values on one subject
			tr(res("hc:c"), res("hc:rows"), bn(100+k[0])),
			tr(res("hc:c"), res("hc:rows"), bn(100+k[1])),
			tr(bn(100+k[0]), res("lk:v"), num(1)),
			tr(bn(100+k[1]), res("lk:v"), num(1)),
			// linked: two pairs a-b of identical shape
			tr(res("hc:d"), res("hc:a"), bn(k[2])),
			tr(res("hc:d"), res("hc:a"), bn(k[3])),
			tr(bn(k[2]), res("hc:b"), res(uuidOf(r))),
			tr(bn(k[3]), res("hc:b"), res(uuidOf(r))),
		}
		r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		return out
	}
	want := canonicalLines(graph(rand.New(rand.NewSource(1))))
	labels := map[string]bool{}
	for _, l := range want {
		for _, f := range strings.Split(l, "\t") {
			if strings.HasPrefix(f, "_:") {
				labels[f] = true
			}
		}
	}
	t.Logf("the dump:\n%s", strings.Join(want, "\n"))
	if len(labels) != 6 {
		t.Errorf("want 6 distinct labels, got %d: %v", len(labels), want)
	}
	for seed := int64(2); seed < 30; seed++ {
		if got := canonicalLines(graph(rand.New(rand.NewSource(seed)))); !reflect.DeepEqual(got, want) {
			t.Fatalf("seed %d:\n got: %s\nwant: %s", seed, strings.Join(got, "\n      "), strings.Join(want, "\n      "))
		}
	}
}

// Go's quantised double and C++'s exact one differ in the line and not in the
// labels.
func TestQuantisedDoubleDoesNotRename(t *testing.T) {
	g := func(x float64) []harness.SnapshotTriple {
		return []harness.SnapshotTriple{
			tr(res("hc:c"), res("hc:rows"), bn(1)),
			tr(bn(1), res("hc:amount"), dbl(x)),
			tr(bn(1), res("lk:v"), num(3)),
		}
	}
	exact := 12345.67
	a, b := canonicalLines(g(exact)), canonicalLines(g(Quantise(exact)))
	onlyA, onlyB := minus(a, b), minus(b, a)
	if len(onlyA) != 1 || len(onlyB) != 1 {
		t.Fatalf("want the amount line alone to differ: %v / %v", onlyA, onlyB)
	}
	if !strings.HasSuffix(onlyA[0], "\t12345.67^^double") || !strings.HasSuffix(onlyB[0], "\t"+FormatDouble(Quantise(exact))+"^^double") {
		t.Errorf("doubles are not written as held: %q, %q", onlyA[0], onlyB[0])
	}
}

func TestTermsRoundTrip(t *testing.T) {
	cases := []struct {
		term harness.Term
		line string
		kind string
	}{
		{res("hc:claim1"), "hc:claim1", "resource"},
		{res(""), `""^^resource`, "resource"},
		{res("null"), `"null"^^resource`, "resource"},
		{txt("a\tb\"c"), `"a\tb\"c"`, "text"},
		{num(-4), "-4^^int", "int"},
		{harness.Term{Kind: harness.TermUint, Text: "4"}, "4^^uint", "uint"},
		{dbl(0.1), "0.1^^double", "double"},
		{harness.Term{Kind: harness.TermDate, Text: "2024-01-31"}, "2024-01-31^^date", "date"},
		{harness.Term{Kind: harness.TermDatetime, Text: "2024-01-31T13:05:09"}, "2024-01-31T13:05:09Z^^datetime", "datetime"},
		{harness.Term{Kind: harness.TermNull}, "null", "null"},
	}
	for _, c := range cases {
		got := renderGround(c.term, false)
		if got != c.line {
			t.Errorf("%v: written %q, want %q", c.term, got, c.line)
		}
		back, ok := ParseTerm(got)
		if !ok || back.Kind != c.kind {
			t.Errorf("%q: read back as %+v", got, back)
		}
		if c.kind == "text" || c.kind == "resource" {
			if back.Value != c.term.Text {
				t.Errorf("%q: value %q, want %q", got, back.Value, c.term.Text)
			}
		}
	}
	if v, ok := (Term{Kind: "double", Value: "0.1"}).Num(); !ok || v != 0.1 {
		t.Errorf("Num: %v %v", v, ok)
	}
}

func TestFileName(t *testing.T) {
	if got := FileName(4, "jet_rules/main/CM/2_CM02_Matching_SM_Main1.jr"); got != "05_2_CM02_Matching_SM_Main1.tsv" {
		t.Errorf("FileName = %q", got)
	}
	if got := Path("d", "go", "member1", 0, "a b.jr"); got != "d/go/member1/01_a_b.tsv" {
		t.Errorf("Path = %q", got)
	}
}
