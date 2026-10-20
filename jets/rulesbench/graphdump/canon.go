package graphdump

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
)

const (
	// labelRounds is the number of refinement rounds a label is taken after.
	labelRounds = 3
	// labelLen is the number of hex digits of a label.
	labelLen = 12
	// cachePredicate is how a cache triple's predicate is written into colours.
	cachePredicate = "jets:lookup_cache"
	// GoCachePrefix is the Go engine's name for a lookup table's cache resource,
	// which C++ cache resources are renamed to.
	GoCachePrefix = "jets:cache:"
)

// lookupPredicates are the predicates a lookup cache triple has.
var lookupPredicates = map[string]bool{"jets:lookup_row": true, "jets:lookup_multi_rows": true}

// canon names the minted nodes of a set of triples.
type canon struct {
	triples []harness.SnapshotTriple
	// at is the minted node at each position of each triple, or -1.
	at [][3]int
	// facet marks a minted node's text form ('t') at a position.
	facet [][3]byte
	// ground is each position's term as written into colours; "" where minted.
	ground [][3]string
	// incident lists the triples each minted node is in.
	incident [][]int
	// names are each minted node's engine name, for breaking ties.
	names []string
}

// canonicalLines renames minted nodes, renders each triple and sorts the lines.
func canonicalLines(triples []harness.SnapshotTriple) []string {
	c := newCanon(triples)
	labels := c.labels()
	lines := make([]string, len(c.triples))
	var b strings.Builder
	for i, t := range c.triples {
		b.Reset()
		for pos, term := range [3]harness.Term{t.S, t.P, t.O} {
			if pos > 0 {
				b.WriteByte('\t')
			}
			if m := c.at[i][pos]; m >= 0 {
				b.WriteString("_:")
				b.WriteString(labels[m])
				if c.facet[i][pos] == 't' {
					b.WriteString("^^text")
				}
				continue
			}
			b.WriteString(renderGround(term, false))
		}
		lines[i] = b.String()
	}
	return sortedUnique(lines)
}

func newCanon(in []harness.SnapshotTriple) *canon {
	triples := append([]harness.SnapshotTriple(nil), in...)

	// The lookup caches: rename C++ cache resources to the Go name, and mark the
	// subjects they cache as minted.
	rename := map[string]string{}
	lookupSubject := map[string]bool{}
	cacheTriple := make([]bool, len(triples))
	for i, t := range triples {
		if t.P.Kind != harness.TermResource || !lookupPredicates[t.P.Text] ||
			t.S.Kind != harness.TermResource || t.O.Kind != harness.TermResource {
			continue
		}
		switch s, o := t.S.Text, t.O.Text; {
		case strings.HasPrefix(s, GoCachePrefix):
		case strings.HasPrefix(o, "jets:"+s+":"), strings.HasPrefix(o, "jets:rand:"+s+":"):
			rename[s] = GoCachePrefix + s
		default:
			continue
		}
		cacheTriple[i] = true
		lookupSubject[t.O.Text] = true
	}
	if len(rename) > 0 {
		for i := range triples {
			for _, term := range []*harness.Term{&triples[i].S, &triples[i].P, &triples[i].O} {
				if term.Kind == harness.TermResource {
					if n, ok := rename[term.Text]; ok {
						term.Text = n
					}
				}
			}
		}
	}

	c := &canon{
		triples: triples,
		at:      make([][3]int, len(triples)),
		facet:   make([][3]byte, len(triples)),
		ground:  make([][3]string, len(triples)),
	}
	ids := map[string]int{}
	identity := func(t harness.Term) (string, byte, bool) {
		switch t.Kind {
		case harness.TermBlank:
			return "b:" + t.Text, 'n', true
		case harness.TermResource:
			if lookupSubject[t.Text] || IsMintedName(t.Text) {
				return "r:" + t.Text, 'n', true
			}
		case harness.TermText:
			if IsMintedName(t.Text) {
				return "r:" + t.Text, 't', true
			}
		}
		return "", 0, false
	}
	for i, t := range triples {
		for pos, term := range [3]harness.Term{t.S, t.P, t.O} {
			id, facet, minted := identity(term)
			if !minted {
				c.at[i][pos] = -1
				c.ground[i][pos] = renderGround(term, true)
				continue
			}
			m, ok := ids[id]
			if !ok {
				m = len(c.names)
				ids[id] = m
				c.names = append(c.names, id)
				c.incident = append(c.incident, nil)
			}
			c.at[i][pos] = m
			c.facet[i][pos] = facet
			if n := len(c.incident[m]); n == 0 || c.incident[m][n-1] != i {
				c.incident[m] = append(c.incident[m], i)
			}
		}
		if cacheTriple[i] {
			c.ground[i][1] = cachePredicate
		}
	}
	return c
}

// labels names every minted node; see the package documentation.
func (c *canon) labels() []string {
	n := len(c.names)
	if n == 0 {
		return nil
	}
	col := make([]string, n)
	for m := range col {
		col[m] = "m"
	}
	for r := 0; r < labelRounds; r++ {
		col = c.round(col)
	}
	local := col

	deep := c.refine(local)
	for {
		groups := groupBy(deep)
		var tied [][]int
		for _, g := range groups {
			if len(g) > 1 {
				tied = append(tied, g)
			}
		}
		if len(tied) == 0 {
			break
		}
		size := make(map[string]int, len(groups))
		for _, g := range groups {
			size[deep[g[0]]] = len(g)
		}
		var unsafe []int
		next := append([]string(nil), deep...)
		for _, g := range tied {
			if !c.interchangeable(g, deep, size) {
				if unsafe == nil {
					unsafe = g
				}
				continue
			}
			sort.Slice(g, func(i, j int) bool { return c.names[g[i]] < c.names[g[j]] })
			for k, m := range g {
				next[m] = hash(deep[m], "#"+strconv.Itoa(k))
			}
		}
		deep = next
		if unsafe != nil {
			first := unsafe[0]
			for _, m := range unsafe[1:] {
				if c.names[m] < c.names[first] {
					first = m
				}
			}
			deep[first] = hash(deep[first], "!")
			deep = c.refine(deep)
		}
	}

	// A label is the local colour's prefix, suffixed within a shared local colour
	// in the order of the stable colours.
	labels := make([]string, n)
	width := prefixWidth(local)
	for _, g := range groupBy(local) {
		base := local[g[0]][:width]
		if len(g) == 1 {
			labels[g[0]] = base
			continue
		}
		sort.Slice(g, func(i, j int) bool { return deep[g[i]] < deep[g[j]] })
		for k, m := range g {
			labels[m] = base + "." + strconv.Itoa(k+1)
		}
	}
	return labels
}

// round is one refinement round.
func (c *canon) round(col []string) []string {
	next := make([]string, len(col))
	var lines []string
	var b strings.Builder
	for m := range col {
		lines = lines[:0]
		for _, i := range c.incident[m] {
			b.Reset()
			for pos := 0; pos < 3; pos++ {
				if pos > 0 {
					b.WriteByte('\t')
				}
				switch o := c.at[i][pos]; {
				case o == m:
					b.WriteByte('@')
				case o >= 0:
					b.WriteByte('#')
					b.WriteString(col[o])
				default:
					b.WriteString(c.ground[i][pos])
					continue
				}
				if c.facet[i][pos] == 't' {
					b.WriteString("^^text")
				}
			}
			lines = append(lines, b.String())
		}
		sort.Strings(lines)
		next[m] = hash(col[m], lines...)
	}
	return next
}

// refine runs rounds until the partition into colours stops changing.
func (c *canon) refine(col []string) []string {
	classes := countDistinct(col)
	for {
		next := c.round(col)
		k := countDistinct(next)
		col = next
		if k == classes {
			return col
		}
		classes = k
	}
}

// interchangeable reports whether the tied nodes g can be numbered in any order
// without changing the dump: none of them has a minted neighbour in a class of
// more than one node, so all of them have the same neighbours.
func (c *canon) interchangeable(g []int, col []string, size map[string]int) bool {
	for _, m := range g {
		for _, i := range c.incident[m] {
			for pos := 0; pos < 3; pos++ {
				if o := c.at[i][pos]; o >= 0 && o != m && size[col[o]] > 1 {
					return false
				}
			}
		}
	}
	return true
}

func hash(prev string, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(prev))
	for _, p := range parts {
		h.Write([]byte{'\n'})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// groupBy groups nodes by colour, in colour order, members in node order.
func groupBy(col []string) [][]int {
	byColour := map[string][]int{}
	for m, k := range col {
		byColour[k] = append(byColour[k], m)
	}
	keys := make([]string, 0, len(byColour))
	for k := range byColour {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	groups := make([][]int, len(keys))
	for i, k := range keys {
		groups[i] = byColour[k]
	}
	return groups
}

func countDistinct(col []string) int {
	seen := make(map[string]struct{}, len(col))
	for _, k := range col {
		seen[k] = struct{}{}
	}
	return len(seen)
}

// prefixWidth is labelLen, or more if two colours share their first labelLen
// digits.
func prefixWidth(col []string) int {
	for w := labelLen; ; w += 4 {
		seen := map[string]string{}
		ok := true
		for _, k := range col {
			if w >= len(k) {
				return len(k)
			}
			if prev, dup := seen[k[:w]]; dup && prev != k {
				ok = false
				break
			}
			seen[k[:w]] = k
		}
		if ok {
			return w
		}
	}
}
