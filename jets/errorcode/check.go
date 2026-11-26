package errorcode

import (
	"fmt"
	"sort"
	"strings"
)

// Raise is one rule consequent that asserts jets:exception, as the workspace
// compile reads it from a rule file's model.
type Raise struct {
	// File is the rule file the rule is in, relative to the workspace.
	File string
	// Rule is the rule's name.
	Rule string
	// Head is the message's literal head: the literal itself, or the literals
	// that start a '+' chain, up to its first value. Empty when the message
	// starts with a value, which no catalogue row can match.
	Head string
	// HasValue is true when a value follows the head.
	HasValue bool
}

// Ref is the raise's source_ref: "<file> <rule>".
func (r Raise) Ref() string {
	return strings.TrimSpace(r.File + " " + r.Rule)
}

// CheckReport is what Check finds when it compares a workspace's raises with its
// catalogue rows. It is advice for the compile log: nothing in it fails a
// compile.
type CheckReport struct {
	// Raises is the number of raises compared, and Heads the distinct heads.
	Raises, Heads int
	// Rows is the number of catalogue rows compared.
	Rows int
	// Uncatalogued are the raises no row matches, one per distinct head.
	Uncatalogued []Raise
	// NoHead are the raises whose message starts with a value.
	NoHead []Raise
	// Unmatched are the rules rows no raise matches.
	Unmatched []Entry
	// Unreachable are the rows whose threshold is above what one operator
	// instance stores at the default cap.
	Unreachable []Entry
	// SlugCollisions are the groups of codes that share one slug.
	SlugCollisions [][]string
	// Problems are rows that are malformed: an unknown kind or source, a "*"
	// row with a match prefix, a jetstore row naming no fixed code, a row
	// shadowed by a fixed code, a code with no slug.
	Problems []string
}

// Check compares the raises a compile found with the catalogue rows. maxErrorCount
// is the cap a threshold is compared with, DefaultMaxErrorCount when 0 or less.
func Check(raises []Raise, rows []Entry, maxErrorCount int) CheckReport {
	if maxErrorCount <= 0 {
		maxErrorCount = DefaultMaxErrorCount
	}
	rep := CheckReport{Raises: len(raises), Rows: len(rows)}

	normalized := make([]Entry, len(rows))
	for i := range rows {
		normalized[i] = normalize(rows[i])
	}
	var ruleRows []Entry
	for _, r := range normalized {
		if r.OperatorType == OperatorJetrules && r.Source != SourceJetStore {
			ruleRows = append(ruleRows, r)
		}
	}

	// Raises: one per distinct head, the first by file and rule
	sorted := append([]Raise(nil), raises...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].File != sorted[j].File {
			return sorted[i].File < sorted[j].File
		}
		return sorted[i].Rule < sorted[j].Rule
	})
	seenHead := map[string]bool{}
	for _, r := range sorted {
		if r.Head == "" {
			rep.NoHead = append(rep.NoHead, r)
			continue
		}
		if seenHead[r.Head] {
			continue
		}
		seenHead[r.Head] = true
		covered := false
		for _, row := range ruleRows {
			if strings.HasPrefix(r.Head, row.MatchPrefix) {
				covered = true
				break
			}
		}
		if !covered {
			rep.Uncatalogued = append(rep.Uncatalogued, r)
		}
	}
	rep.Heads = len(seenHead)

	// Rules rows no raise matches. A row longer than a head matches it when a
	// value follows the head, because the value may complete it.
	for _, row := range normalized {
		if row.Source != SourceRules || row.OperatorType == DefaultOperator {
			continue
		}
		matched := false
		if row.OperatorType == OperatorJetrules {
			for _, r := range raises {
				if r.Head == "" {
					continue
				}
				if strings.HasPrefix(r.Head, row.MatchPrefix) ||
					(r.HasValue && strings.HasPrefix(row.MatchPrefix, r.Head)) {
					matched = true
					break
				}
			}
		}
		if !matched {
			rep.Unmatched = append(rep.Unmatched, row)
		}
	}

	// Thresholds above one instance's cap, and malformed rows
	fixed := Fixed()
	fixedByKey := map[string]Entry{}
	for _, f := range fixed {
		fixedByKey[f.Key()] = f
	}
	for _, row := range normalized {
		if row.ReportThreshold != nil && *row.ReportThreshold > maxErrorCount {
			rep.Unreachable = append(rep.Unreachable, row)
		}
		if row.ReportThreshold != nil && *row.ReportThreshold < 1 {
			rep.Problems = append(rep.Problems, fmt.Sprintf(
				"%s: report_threshold %d is below 1, so the code reports on every run with a row",
				rowName(row), *row.ReportThreshold))
		}
		if row.Kind != KindError && row.Kind != KindDebug {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: kind '%s' is neither 'error' nor 'debug'", rowName(row), row.Kind))
		}
		switch row.Source {
		case SourceRules, SourcePipelineConfig, SourceJetStore:
		default:
			rep.Problems = append(rep.Problems, fmt.Sprintf(
				"%s: source '%s' is not 'rules', 'pipeline_config' or 'jetstore'", rowName(row), row.Source))
		}
		if row.OperatorType == DefaultOperator && row.MatchPrefix != "" {
			rep.Problems = append(rep.Problems, fmt.Sprintf(
				"%s: a '*' row with a match prefix matches no message; the default row has an empty match_prefix", rowName(row)))
		}
		if row.OperatorType == "" {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: operator_type is empty, so the row matches no message", rowName(row)))
		}
		_, isFixed := fixedByKey[row.Key()]
		if row.Source == SourceJetStore && !isFixed {
			rep.Problems = append(rep.Problems, fmt.Sprintf(
				"%s: a 'jetstore' row overrides a fixed code and names none; it is read as an ordinary row", rowName(row)))
		}
		if row.Source != SourceJetStore && isFixed {
			rep.Problems = append(rep.Problems, fmt.Sprintf(
				"%s: the match prefix is a fixed code's, which wins; give the row source 'jetstore' to override it", rowName(row)))
		}
		if !row.IsDefaultRow() && Slug(row.Code) == "" {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: code '%s' has no letter or digit, so it has no slug", rowName(row), row.Code))
		}
	}

	// Slugs: the codes a reader can show, fixed and catalogued, and the
	// uncatalogued ones, two of which must not share an identifier
	codes := map[string]bool{}
	for _, f := range fixed {
		codes[f.Code] = true
	}
	for _, row := range normalized {
		if row.IsDefaultRow() || row.OperatorType == DefaultOperator {
			continue
		}
		if f, ok := fixedByKey[row.Key()]; ok && row.Source == SourceJetStore {
			codes[f.Code] = true
			continue
		}
		codes[row.Code] = true
		codes[UncataloguedCode(row.OperatorType)] = true
	}
	for _, f := range fixed {
		codes[UncataloguedCode(f.OperatorType)] = true
	}
	codes[UncataloguedCode("")] = true
	bySlug := map[string][]string{}
	for code := range codes {
		if s := Slug(code); s != "" {
			bySlug[s] = append(bySlug[s], code)
		}
	}
	for _, group := range bySlug {
		if len(group) > 1 {
			sort.Strings(group)
			rep.SlugCollisions = append(rep.SlugCollisions, group)
		}
	}
	sort.Slice(rep.SlugCollisions, func(i, j int) bool { return rep.SlugCollisions[i][0] < rep.SlugCollisions[j][0] })
	return rep
}

func rowName(e Entry) string {
	return fmt.Sprintf("row (%s, '%s')", e.OperatorType, e.MatchPrefix)
}

// Clean says whether the report has nothing to say.
func (rep CheckReport) Clean() bool {
	return len(rep.Uncatalogued) == 0 && len(rep.NoHead) == 0 && len(rep.Unmatched) == 0 &&
		len(rep.Unreachable) == 0 && len(rep.SlugCollisions) == 0 && len(rep.Problems) == 0
}

// String is the report as the compile log shows it.
func (rep CheckReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Error code catalogue: %d jets:exception raises with %d distinct heads, %d catalogue rows\n",
		rep.Raises, rep.Heads, rep.Rows)
	if rep.Clean() {
		b.WriteString("  every raise has a row and every row is sound\n")
		return b.String()
	}
	if len(rep.Uncatalogued) > 0 {
		fmt.Fprintf(&b, "  %d raise heads have no row; their messages read '%s' until one is added:\n",
			len(rep.Uncatalogued), UncataloguedCode(OperatorJetrules))
		for _, r := range rep.Uncatalogued {
			fmt.Fprintf(&b, "    '%s' (%s)\n", r.Head, r.Ref())
		}
	}
	for _, r := range rep.NoHead {
		fmt.Fprintf(&b, "  a raise starts with a value and no literal, so no row can match it: %s\n", r.Ref())
	}
	for _, e := range rep.Unmatched {
		fmt.Fprintf(&b, "  %s: a 'rules' row no raise in the rule files matches\n", rowName(e))
	}
	for _, e := range rep.Unreachable {
		fmt.Fprintf(&b, "  %s: report_threshold %d is above the %d rows one operator instance stores at the default "+
			"max_error_count; it is reached only by a run with several instances (workers or nodes), "+
			"or a larger max_error_count, since stored rows are a lower bound per instance\n",
			rowName(e), *e.ReportThreshold, DefaultMaxErrorCount)
	}
	for _, g := range rep.SlugCollisions {
		quoted := make([]string, len(g))
		for i := range g {
			quoted[i] = "'" + g[i] + "'"
		}
		fmt.Fprintf(&b, "  codes %s share the slug '%s'; change one code\n", strings.Join(quoted, ", "), Slug(g[0]))
	}
	for _, p := range rep.Problems {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	if ins := rep.InsertSQL(); ins != "" {
		b.WriteString("  The rows to add to process_config/base__workspace_init_db.sql, to review before adding:\n")
		b.WriteString(ins)
	}
	return b.String()
}

// sqlText quotes s as a SQL string literal.
func sqlText(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// InsertSQL is a ready INSERT block for the uncatalogued raises, in the style a
// workspace's base init script uses, or "" when there are none. Each row takes
// the default code and threshold; a person sets the code, the kind and the
// threshold before adding it.
func (rep CheckReport) InsertSQL() string {
	if len(rep.Uncatalogued) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("INSERT INTO jetsapi.error_code_catalogue\n")
	b.WriteString("  (operator_type, match_prefix, code, kind, report_threshold, source, source_ref, description) VALUES\n")
	for i, r := range rep.Uncatalogued {
		fmt.Fprintf(&b, "  (%s, %s, %s, %s, NULL, %s, %s, NULL)",
			sqlText(OperatorJetrules), sqlText(r.Head), sqlText(DefaultCode(r.Head)), sqlText(KindError),
			sqlText(SourceRules), sqlText(r.Ref()))
		if i < len(rep.Uncatalogued)-1 {
			b.WriteString(",\n")
		} else {
			b.WriteString("\nON CONFLICT DO NOTHING\n;\n")
		}
	}
	return b.String()
}
