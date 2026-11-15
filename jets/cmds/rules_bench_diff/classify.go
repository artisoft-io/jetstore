package main

import (
	"sort"
	"strconv"
	"strings"
)

// classify gives each line the first known difference that matches it. The
// kinds are tried in this order, whatever their order in the list:
//
//  1. relabelled: A's labels are rewritten into B's through the one-to-one
//     pairing of labels (see correspond and pairRepeated), and a line that then
//     equals a B line is relabelled. The rewritten line is what the later kinds
//     see.
//  2. merged: a label of one side that corresponds to several of the other.
//  3. the value pairs, double_quantised, double_near, text_double_format,
//     literal_regex_group and predicate_alias, in list order.
//  4. the single-line entries, in list order.
func classify(a, b []*line, allA, allB []string, member, file, labelA, labelB, scope string, known *KnownList) {
	if known == nil {
		return
	}
	var entries []*Entry
	for i := range known.Entries {
		if e := &known.Entries[i]; e.inScope(scope) && e.inFile(member, file) {
			entries = append(entries, e)
		}
	}
	c := correspond(a, b)
	for _, e := range entries {
		if e.Pair == PairRelabelled {
			c.pairRepeated(a, b, allA, allB)
			c.relabel(a, b, e.ID)
			break
		}
	}
	for _, e := range entries {
		if e.Pair == PairMerged {
			if e.OnlyOn == "" || e.OnlyOn == labelA {
				c.merged(a, b, c.aToB, c.bToA, anchored(allA, e), e.ID)
			}
			if e.OnlyOn == "" || e.OnlyOn == labelB {
				c.merged(b, a, c.bToA, c.aToB, anchored(allB, e), e.ID)
			}
		}
	}
	bySP, bySO := map[string][]*line{}, map[string][]*line{}
	for _, l := range b {
		bySP[l.s+"\t"+l.p] = append(bySP[l.s+"\t"+l.p], l)
		bySO[l.s+"\t"+l.o] = append(bySO[l.s+"\t"+l.o], l)
	}
	for _, e := range entries {
		index, key := bySP, func(l *line) string { return l.s + "\t" + l.p }
		switch e.Pair {
		case PairDoubleQuantised, PairDoubleNear, PairTextDoubleFormat, PairLiteralRegexGroup:
		case PairPredicateAlias:
			index, key = bySO, func(l *line) string { return l.s + "\t" + l.o }
		default:
			continue
		}
		// x is the only_on side's line, for the kinds that read it.
		swap := e.OnlyOn != "" && e.OnlyOn == labelB
		if e.OnlyOn != "" && e.OnlyOn != labelA && !swap {
			continue
		}
		for _, la := range a {
			if la.known != "" {
				continue
			}
			for _, lb := range index[key(la)] {
				x, y := la, lb
				if swap {
					x, y = lb, la
				}
				if lb.known == "" && e.matchPair(x, y) {
					la.known, lb.known = e.ID, e.ID
					break
				}
			}
		}
	}
	for _, side := range []struct {
		lines []*line
		label string
	}{{a, labelA}, {b, labelB}} {
		for _, l := range side.lines {
			for _, e := range entries {
				if l.known == "" && e.matchLine(l, side.label) {
					l.known = e.ID
					break
				}
			}
		}
	}
}

// correspondence links the minted labels of A and B. Two labels correspond when
// they hold the same place in a line shape (the line with its labels blanked)
// that occurs exactly once on each side.
//
// pair is the one-to-one map from A's labels to B's that relabelling rewrites
// through: the labels that correspond one to one, and then the labels that only
// repeating shapes hold, paired by pairRepeated. merged reads aToB and bToA only,
// so pair changes nothing it sees.
type correspondence struct {
	aToB, bToA map[string]map[string]bool
	pair       map[string]string
}

func correspond(a, b []*line) *correspondence {
	c := &correspondence{aToB: map[string]map[string]bool{}, bToA: map[string]map[string]bool{}}
	shapes := func(lines []*line) map[string][][]string {
		m := map[string][][]string{}
		for _, l := range lines {
			if shape, labels := blank(l.text); len(labels) > 0 {
				m[shape] = append(m[shape], labels)
			}
		}
		return m
	}
	shapesA, shapesB := shapes(a), shapes(b)
	link := func(m map[string]map[string]bool, x, y string) {
		if m[x] == nil {
			m[x] = map[string]bool{}
		}
		m[x][y] = true
	}
	for shape, la := range shapesA {
		lb := shapesB[shape]
		if len(la) != 1 || len(lb) != 1 {
			continue
		}
		for i, x := range la[0] {
			link(c.aToB, x, lb[0][i])
			link(c.bToA, lb[0][i], x)
		}
	}
	return c
}

// blank returns text with each minted label replaced by _:, and the labels.
func blank(text string) (shape string, labels []string) {
	fields := strings.Split(text, "\t")
	for i, f := range fields {
		if !strings.HasPrefix(f, "_:") {
			continue
		}
		label, isText := strings.CutSuffix(f[2:], "^^text")
		labels = append(labels, label)
		fields[i] = "_:"
		if isText {
			fields[i] += "^^text"
		}
	}
	return strings.Join(fields, "\t"), labels
}

// rewrite replaces each minted label of text by f's answer, when it has one.
func rewrite(text string, f func(label string) (string, bool)) (string, bool) {
	fields := strings.Split(text, "\t")
	changed := false
	for i, fl := range fields {
		if !strings.HasPrefix(fl, "_:") {
			continue
		}
		label, isText := strings.CutSuffix(fl[2:], "^^text")
		to, ok := f(label)
		if !ok {
			continue
		}
		changed = changed || to != label
		fields[i] = "_:" + to
		if isText {
			fields[i] += "^^text"
		}
	}
	return strings.Join(fields, "\t"), changed
}

// oneToOne is the label of B that x corresponds to alone, and that corresponds
// to x alone.
func oneToOne(x string, there, back map[string]map[string]bool) (string, bool) {
	if len(there[x]) != 1 {
		return "", false
	}
	for y := range there[x] {
		if len(back[y]) == 1 {
			return y, true
		}
	}
	return "", false
}

// relabel rewrites A's lines into B's labels through c.pair (or through the one
// to one correspondence when pairRepeated has not run) and classifies as id each
// line that then equals an unclassified B line.
func (c *correspondence) relabel(a, b []*line, id string) {
	byText := unclassifiedByText(b)
	to := func(x string) (string, bool) { return oneToOne(x, c.aToB, c.bToA) }
	if c.pair != nil {
		to = func(x string) (string, bool) { y, ok := c.pair[x]; return y, ok }
	}
	for _, l := range a {
		if l.known != "" {
			continue
		}
		text, changed := rewrite(l.text, to)
		if !changed {
			continue
		}
		l.setFields(text)
		if lb := byText[text]; lb != nil {
			l.known, lb.known = id, id
			delete(byText, text)
		}
	}
}

// pairRepeated sets c.pair: the labels that correspond one to one, and then the
// labels that only line shapes occurring more than once hold, which correspond
// does not link. Those are paired by colour refinement over the differing lines,
// the way graphdump names minted nodes:
//
//   - Fixed labels keep a colour of their own: a label paired one to one shares
//     a colour with its partner; a label of a file whose other file has the same
//     label, and that correspond links to nothing, is paired with itself; a label
//     correspond links to several (merged's case) gets a colour nothing shares,
//     so it is never paired here and merged sees it as before.
//   - Every other label starts with one colour, and a round gives it the colour
//     of its colour and its lines, each written with the label as @ and every
//     other label as its colour, until the number of colours stops growing.
//   - A colour held by one label on each side pairs them. A colour held by
//     several on each side whose lines hold no other unpaired label holds
//     interchangeable nodes: as many are paired as both sides have, in label
//     order, and the rest stay unpaired.
//   - When no colour pairs, the first colour held by the same number of labels on
//     each side is paired in label order, all of it, and the refinement runs
//     again, which pairs their neighbours next: individualisation, one class at
//     a time. In a dump the minted nodes hang from named ones as trees, where
//     nodes of one colour are interchangeable, so any order is right. In a
//     regular structure that colour refinement cannot split (graphdump's one
//     known exception) an order can be wrong, and then it costs classifications
//     and never adds a wrong one.
//
// A colour held on one side only stays unpaired, so its lines stay unclassified.
// Pairing never classifies a line by itself: relabel classifies a line only when
// its rewritten text is a line of B, so a pair is only ever the bijection under
// which the lines it classifies are equal.
func (c *correspondence) pairRepeated(a, b []*line, allA, allB []string) {
	c.pair = map[string]string{}
	const fixedA, fixedB = "A", "B"
	colA, colB := map[string]string{}, map[string]string{}
	linesA, linesB := labelLines(a), labelLines(b)
	labelsIn := func(all []string) map[string]bool {
		m := map[string]bool{}
		for _, t := range all {
			_, labels := blank(t)
			for _, x := range labels {
				m[x] = true
			}
		}
		return m
	}
	inA, inB := labelsIn(allA), labelsIn(allB)
	n := 0
	fix := func(x, y string) {
		c.pair[x] = y
		n++
		colA[x], colB[y] = "="+strconv.Itoa(n), "="+strconv.Itoa(n)
	}
	for _, x := range sortedKeys(linesA) {
		if y, ok := oneToOne(x, c.aToB, c.bToA); ok {
			fix(x, y)
		}
	}
	// A label both files have, and that correspond links on neither side, is
	// the same node: it is pinned to itself. This rewrites nothing.
	same := func(x string) bool {
		return inA[x] && inB[x] && colA[x] == "" && colB[x] == "" && len(c.aToB[x]) == 0 && len(c.bToA[x]) == 0
	}
	for _, x := range sortedKeys(linesA) {
		if same(x) {
			fix(x, x)
		}
	}
	for _, y := range sortedKeys(linesB) {
		if same(y) {
			fix(y, y)
		}
	}
	for x := range linesA {
		if colA[x] == "" && len(c.aToB[x]) > 0 {
			colA[x] = fixedA + x
		}
	}
	for y := range linesB {
		if colB[y] == "" && len(c.bToA[y]) > 0 {
			colB[y] = fixedB + y
		}
	}
	free := func(col map[string]string, lines map[string][]string) []string {
		var out []string
		for x := range lines {
			if col[x] == "" || col[x][0] == '?' {
				out = append(out, x)
			}
		}
		sort.Strings(out)
		return out
	}
	for {
		freeA, freeB := free(colA, linesA), free(colB, linesB)
		if len(freeA) == 0 || len(freeB) == 0 {
			return
		}
		refine(freeA, freeB, colA, colB, linesA, linesB)
		classA, classB := map[string][]string{}, map[string][]string{}
		for _, x := range freeA {
			classA[colA[x]] = append(classA[colA[x]], x)
		}
		for _, y := range freeB {
			classB[colB[y]] = append(classB[colB[y]], y)
		}
		progress := false
		var tied []string
		for _, k := range sortedKeys(classA) {
			xs, ys := classA[k], classB[k]
			switch {
			case len(xs) == 0 || len(ys) == 0:
			case len(xs) == 1 && len(ys) == 1:
				fix(xs[0], ys[0])
				progress = true
			case interchangeable(xs, colA, linesA) && interchangeable(ys, colB, linesB):
				for i := 0; i < len(xs) && i < len(ys); i++ {
					fix(xs[i], ys[i])
				}
				progress = true
			case len(xs) == len(ys):
				tied = append(tied, k)
			}
		}
		if !progress {
			if len(tied) == 0 {
				return
			}
			xs, ys := classA[tied[0]], classB[tied[0]]
			for i := range xs {
				fix(xs[i], ys[i])
			}
		}
	}
}

// labelLines maps each minted label of lines to the texts of its lines.
func labelLines(lines []*line) map[string][]string {
	m := map[string][]string{}
	for _, l := range lines {
		_, labels := blank(l.text)
		seen := map[string]bool{}
		for _, x := range labels {
			if !seen[x] {
				seen[x] = true
				m[x] = append(m[x], l.text)
			}
		}
	}
	return m
}

// refine recolours the free labels of both sides together until the number of
// colours stops growing. A free label's colour starts with '?', and a label
// with none is given "?"; a fixed label's colour is never changed. It starts
// from the colours of the previous call, which pairing only made finer, so it
// reaches the same partition as starting afresh in fewer rounds.
func refine(freeA, freeB []string, colA, colB map[string]string, linesA, linesB map[string][]string) {
	start := map[string]bool{}
	for _, x := range freeA {
		if colA[x] == "" {
			colA[x] = "?"
		}
		start[colA[x]] = true
	}
	for _, y := range freeB {
		if colB[y] == "" {
			colB[y] = "?"
		}
		start[colB[y]] = true
	}
	signature := func(x string, col map[string]string, lines []string) string {
		rendered := make([]string, len(lines))
		for i, t := range lines {
			rendered[i], _ = rewrite(t, func(label string) (string, bool) {
				if label == x {
					return "@", true
				}
				if k := col[label]; k != "" {
					return "<" + k + ">", true
				}
				return "<>", true
			})
		}
		sort.Strings(rendered)
		return col[x] + "\n" + strings.Join(rendered, "\n")
	}
	count := len(start)
	for {
		sigA, sigB := make([]string, len(freeA)), make([]string, len(freeB))
		ids := map[string]string{}
		for i, x := range freeA {
			sigA[i] = signature(x, colA, linesA[x])
			ids[sigA[i]] = ""
		}
		for i, y := range freeB {
			sigB[i] = signature(y, colB, linesB[y])
			ids[sigB[i]] = ""
		}
		if len(ids) <= count {
			return
		}
		count = len(ids)
		for i, s := range sortedKeys(ids) {
			ids[s] = "?" + strconv.Itoa(i)
		}
		for i, x := range freeA {
			colA[x] = ids[sigA[i]]
		}
		for i, y := range freeB {
			colB[y] = ids[sigB[i]]
		}
	}
}

// interchangeable is true when no line of xs holds an unpaired label other than
// its own: swapping two of them cannot change a line elsewhere.
func interchangeable(xs []string, col map[string]string, lines map[string][]string) bool {
	for _, x := range xs {
		for _, t := range lines[x] {
			_, labels := blank(t)
			for _, z := range labels {
				if z != x && (col[z] == "" || col[z][0] == '?') {
					return false
				}
			}
		}
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// anchored is the set of minted labels that are the object of a line of all whose
// subject and predicate match e's: a merged entry's anchor.
func anchored(all []string, e *Entry) map[string]bool {
	m := map[string]bool{}
	for _, t := range all {
		parts := strings.Split(t, "\t")
		if len(parts) != 3 || !strings.HasPrefix(parts[2], "_:") || strings.HasSuffix(parts[2], "^^text") {
			continue
		}
		if e.s.MatchString(parts[0]) && e.p.MatchString(parts[1]) {
			m[parts[2][2:]] = true
		}
	}
	return m
}

// merged classifies as id the lines of side `from` that carry a label x which
// corresponds to several labels of side `to`, each of which corresponds to x
// alone: one node on `from` where `to` has several. x must be in anchors. A line
// matches when one of those labels in place of x gives an unclassified line of
// `to`.
func (c *correspondence) merged(from, to []*line, there, back map[string]map[string]bool, anchors map[string]bool, id string) {
	byText := unclassifiedByText(to)
	for _, l := range from {
		if l.known != "" {
			continue
		}
		_, labels := blank(l.mapped())
		for _, x := range labels {
			ys := there[x]
			if len(ys) < 2 || !anchors[x] {
				continue
			}
			whole := true
			for y := range ys {
				if len(back[y]) != 1 {
					whole = false
				}
			}
			if !whole {
				continue
			}
			for y := range ys {
				text, _ := rewrite(l.mapped(), func(label string) (string, bool) { return y, label == x })
				if lt := byText[text]; lt != nil {
					l.known, lt.known = id, id
					delete(byText, text)
					break
				}
			}
			if l.known != "" {
				break
			}
		}
	}
}

func unclassifiedByText(lines []*line) map[string]*line {
	m := make(map[string]*line, len(lines))
	for _, l := range lines {
		if l.known == "" {
			m[l.mapped()] = l
		}
	}
	return m
}
