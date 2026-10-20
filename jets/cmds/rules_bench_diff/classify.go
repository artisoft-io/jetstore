package main

import "strings"

// classify gives each line the first known difference that matches it. The
// kinds are tried in this order, whatever their order in the list:
//
//  1. relabelled: A's labels are rewritten into B's through the one-to-one
//     correspondence (see correspond), and a line that then equals a B line is
//     relabelled. The rewritten line is what the later kinds see.
//  2. merged: a label of one side that corresponds to several of the other.
//  3. the value pairs, double_quantised, double_near, text_double_format,
//     literal_regex_group and predicate_alias, in list order.
//  4. the single-line entries, in list order.
func classify(a, b []*line, allA, allB []string, labelA, labelB, scope string, known *KnownList) {
	if known == nil {
		return
	}
	var entries []*Entry
	for i := range known.Entries {
		if e := &known.Entries[i]; e.inScope(scope) {
			entries = append(entries, e)
		}
	}
	c := correspond(a, b)
	for _, e := range entries {
		if e.Pair == PairRelabelled {
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
type correspondence struct {
	aToB, bToA map[string]map[string]bool
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

// relabel rewrites A's lines into B's labels and classifies as id each line
// that then equals an unclassified B line.
func (c *correspondence) relabel(a, b []*line, id string) {
	byText := unclassifiedByText(b)
	for _, l := range a {
		if l.known != "" {
			continue
		}
		text, changed := rewrite(l.text, func(x string) (string, bool) { return oneToOne(x, c.aToB, c.bToA) })
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
