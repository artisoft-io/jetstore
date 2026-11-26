package keyset

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

func ptr(s string) *string { return &s }

// The text and the hash are the rules both sides compute, so they are pinned
// to values computed outside Go (Python's hashlib).
func TestKeyTextAndHashArePinned(t *testing.T) {
	cases := []struct {
		values []*string
		text   string
		hash   uint64
	}{
		{[]*string{ptr("a"), nil}, "1a\x1f0", 249405891899316678},
		{[]*string{ptr("CLM-1"), ptr("12")}, "1CLM-1\x1f112", 15235548030381960349},
	}
	for _, c := range cases {
		if got := KeyText(c.values); got != c.text {
			t.Errorf("KeyText = %q, want %q", got, c.text)
		}
		if got := Hash(c.text); got != c.hash {
			t.Errorf("Hash(%q) = %d, want %d", c.text, got, c.hash)
		}
	}
	if Hash("") != 16406829232824261652 {
		t.Errorf("Hash of the empty text is %d", Hash(""))
	}
	// A null and an empty value differ, and so does where the empty one is.
	texts := map[string]bool{}
	for _, v := range [][]*string{{nil}, {ptr("")}, {ptr("a"), ptr("")}, {ptr(""), ptr("a")}} {
		texts[KeyText(v)] = true
	}
	if len(texts) != 4 {
		t.Errorf("four different keys made %d texts", len(texts))
	}
}

func keys(prefix string, from, to int) []string {
	out := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, KeyText([]*string{ptr(fmt.Sprintf("%s-%07d", prefix, i)), ptr("1")}))
	}
	return out
}

func summarise(texts ...[]string) Summary {
	b := NewBuilder()
	for _, ts := range texts {
		for _, t := range ts {
			b.AddText(t)
		}
	}
	return b.Summary()
}

func TestFingerprintIsExactOnEqualityAndOrderFree(t *testing.T) {
	a := keys("k", 0, 5000)
	shuffled := append([]string(nil), a...)
	rand.New(rand.NewPCG(1, 2)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	s1 := summarise(a)
	s2 := summarise(shuffled, a[:100]) // the same set, in another order, with repeats
	if s1.Fingerprint != s2.Fingerprint || s1.Count != s2.Count || s1.Count != 5000 {
		t.Fatalf("equal sets: %d/%d against %d/%d", s1.Fingerprint, s1.Count, s2.Fingerprint, s2.Count)
	}
	if s1.Sketch != s2.Sketch {
		t.Error("equal sets made different sketches")
	}
	// One key replaced, one removed, one added: each changes the fingerprint.
	for name, other := range map[string][]string{
		"replaced": append(append([]string(nil), a[:4999]...), keys("x", 0, 1)...),
		"removed":  a[:4999],
		"added":    append(append([]string(nil), a...), keys("x", 0, 1)...),
	} {
		if s := summarise(other); s.Fingerprint == s1.Fingerprint {
			t.Errorf("%s: the fingerprint did not change", name)
		}
	}
	// The fingerprint is the modular sum of the distinct hashes.
	var sum uint64
	for _, k := range a {
		sum += Hash(k)
	}
	if sum != s1.Fingerprint {
		t.Error("the fingerprint is not the sum of the hashes")
	}
}

func TestPackRoundTrips(t *testing.T) {
	s := summarise(keys("k", 0, 200000)).Sketch
	b := s.Pack()
	if len(b) != SketchBytes || SketchBytes != 12288 {
		t.Fatalf("packed %d bytes", len(b))
	}
	back, err := Unpack(b)
	if err != nil {
		t.Fatal(err)
	}
	if *back != s {
		t.Fatal("the sketch did not survive packing")
	}
	// Register i is bits 6i..6i+5, most significant first.
	var one Sketch
	one[0], one[1], one[Registers-1] = 51, 1, 63&51
	p := one.Pack()
	if p[0] != 51<<2 || p[1] != 1<<4 || p[SketchBytes-1] != 51 {
		t.Errorf("packing order: % x ... % x", p[:3], p[SketchBytes-3:])
	}
	if _, err := Unpack(b[:10]); err == nil {
		t.Error("a short sketch was read")
	}
}

// The estimates of known sets, and of the keys two sets share, against three
// standard errors at 2^14 registers outside the linear-counting range,
// 3 x 1.04/128 = 2.44% of the union. The measured errors are logged.
func TestEstimatesAreWithinTheirStatedError(t *testing.T) {
	bound := 3 * RelativeError
	for _, n := range []int{1000, 50000, 200000} {
		for _, overlap := range []float64{0, 0.05, 0.5, 1} {
			shared := int(math.Round(overlap * float64(n)))
			a := keys("k", 0, n)
			b := keys("k", n-shared, 2*n-shared)
			sa, sb := summarise(a), summarise(b)
			union := float64(2*n - shared)
			est, se := Merged(&sa.Sketch, &sb.Sketch).EstimateWithError()
			if rel := math.Abs(est-union) / union; rel > bound {
				t.Errorf("n %d overlap %.0f%%: union %.0f estimated %.1f, error %.3f%% above %.3f%%",
					n, 100*overlap, union, est, 100*rel, 100*bound)
			}
			sh, sse := Shared(sa.Count, sb.Count, &sa.Sketch, &sb.Sketch)
			if math.Abs(sh-float64(shared)) > Bound(sse)+1e-9 {
				t.Errorf("n %d overlap %.0f%%: shared %d estimated %.1f, beyond its bound %.1f",
					n, 100*overlap, shared, sh, Bound(sse))
			}
			single := sa.Sketch.Estimate()
			if rel := math.Abs(single-float64(n)) / float64(n); rel > bound {
				t.Errorf("n %d: estimated %.1f", n, single)
			}
			t.Logf("n %6d overlap %3.0f%%: union %7.0f est %9.1f (%+.3f%%, se %.1f); shared %6d est %9.1f (%+.1f keys, %.2f se)",
				n, 100*overlap, union, est, 100*(est-union)/union, se, shared, sh, sh-float64(shared),
				(sh-float64(shared))/math.Max(se, 1e-9))
		}
	}
}

func TestEmptyAndMerge(t *testing.T) {
	var s Sketch
	if s.Estimate() != 0 {
		t.Errorf("empty sketch estimates %f", s.Estimate())
	}
	a, b := summarise(keys("k", 0, 100)), summarise(keys("k", 50, 150))
	m := Merged(&a.Sketch, &b.Sketch)
	both := summarise(keys("k", 0, 150))
	if *m != both.Sketch {
		t.Error("the merged sketch is not the union's")
	}
	if a.Sketch == both.Sketch {
		t.Error("Merged changed its argument")
	}
}

// In the small range the error is a count of register collisions, and its
// tail is what Bound's three extra keys are for: over many pairs of disjoint
// sets of 150 and of 2,000 keys, the shared-key estimate leaves its bound in
// under 0.5% of pairs. The rates, and the rates past three standard errors
// alone, are logged.
func TestBoundHoldsInTheSmallRange(t *testing.T) {
	for _, n := range []int{150, 2000} {
		const trials = 2000
		beyondBound, beyond3se := 0, 0
		for i := range trials {
			a := summarise(keys(fmt.Sprintf("a%d", i), 0, n))
			b := summarise(keys(fmt.Sprintf("b%d", i), 0, n))
			sh, se := Shared(a.Count, b.Count, &a.Sketch, &b.Sketch)
			if sh > Bound(se) {
				beyondBound++
			}
			if sh > 3*se {
				beyond3se++
			}
		}
		t.Logf("n %d: %d of %d pairs beyond the bound, %d beyond three standard errors", n, beyondBound, trials,
			beyond3se)
		if float64(beyondBound) > 0.005*trials {
			t.Errorf("n %d: %d of %d pairs beyond the bound", n, beyondBound, trials)
		}
	}
}
