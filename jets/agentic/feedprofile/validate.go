package feedprofile

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Validate checks a decoded profile against what feed_profile.schema.json
// says, and against three things the schema does not say: a size range's
// minimum is at most its maximum, a required field is named once, and a table
// or column name has no leading or trailing blank and no control character.
// The schema says max_out_of_span_share needs max_service_date_span_days, and
// so does Validate.
// The detectors that read the names still quote them as identifiers.
//
// Every finding is an error. A profile with none is usable.
func Validate(p Profile) []wsvalidate.Finding {
	v := validator{}

	v.text("/declared_by", p.DeclaredBy, true)
	if strings.TrimSpace(p.DeclaredAt) == "" {
		v.add("/declared_at", "declared_at is required: the date of the declaration, YYYY-MM-DD")
	} else if _, err := time.Parse(time.DateOnly, p.DeclaredAt); err != nil {
		v.add("/declared_at", fmt.Sprintf("declared_at %q is not a date written YYYY-MM-DD", p.DeclaredAt))
	}

	if p.PeriodType != "" && !slices.Contains(PeriodTypes, p.PeriodType) {
		v.add("/period_type", fmt.Sprintf("period_type %q is not one of %s",
			p.PeriodType, strings.Join(PeriodTypes, ", ")))
	}
	v.atLeast("/periods_per_delivery", p.PeriodsPerDelivery, 1)
	v.atLeast("/overdue_after_days", p.OverdueAfterDays, 0)
	v.atLeast("/max_service_date_lag_days", p.MaxServiceDateLagDays, 0)
	v.atLeast("/max_service_date_span_days", p.MaxServiceDateSpanDays, 0)
	if p.MaxRedeliveredShare != nil {
		v.share("/max_redelivered_share", *p.MaxRedeliveredShare, true)
	}
	if x := p.MaxOutOfSpanShare; x != nil {
		switch {
		case p.MaxServiceDateSpanDays == nil:
			v.add("/max_out_of_span_share", "max_out_of_span_share is a share of rows outside the span, and needs "+
				"max_service_date_span_days, the span")
		case *x < 0 || *x >= 1:
			v.add("/max_out_of_span_share", fmt.Sprintf("max_out_of_span_share is %g and must be at least 0 and "+
				"below 1", *x))
		}
	}

	if s := p.SizeBytes; s != nil {
		switch {
		case s.Min == nil && s.Max == nil:
			v.add("/size_bytes", "size_bytes needs a min, a max or both")
		case s.Min != nil && *s.Min < 0:
			v.add("/size_bytes/min", "size_bytes.min cannot be negative")
		case s.Max != nil && *s.Max < 0:
			v.add("/size_bytes/max", "size_bytes.max cannot be negative")
		case s.Min != nil && s.Max != nil && *s.Min > *s.Max:
			v.add("/size_bytes", fmt.Sprintf("size_bytes.min %d is larger than size_bytes.max %d", *s.Min, *s.Max))
		}
	}

	if c := p.ClaimSet; c != nil {
		v.text("/claim_set/table", c.Table, true)
		v.text("/claim_set/service_date_column", c.ServiceDateColumn, true)
		v.text("/claim_set/member_column", c.MemberColumn, true)
		if len(c.KeyColumns) == 0 {
			v.add("/claim_set/key_columns", "claim_set.key_columns needs at least one column: the business key is declared, never inferred")
		}
		seen := map[string]bool{}
		for i, col := range c.KeyColumns {
			path := fmt.Sprintf("/claim_set/key_columns/%d", i)
			if v.text(path, col, true) && seen[col] {
				v.add(path, fmt.Sprintf("claim_set.key_columns names %q twice", col))
			}
			seen[col] = true
		}
	}

	seen := map[string]bool{}
	for i, rf := range p.RequiredFields {
		path := fmt.Sprintf("/required_fields/%d", i)
		if v.text(path+"/column", rf.Column, true) && seen[rf.Column] {
			v.add(path+"/column", fmt.Sprintf("required_fields names %q twice", rf.Column))
		}
		seen[rf.Column] = true
		if rf.MinPresentShare == nil {
			v.add(path+"/min_present_share", "min_present_share is required: the smallest share of rows, above 0 and at most 1, in which the column is present")
		} else {
			v.share(path+"/min_present_share", *rf.MinPresentShare, false)
		}
	}
	return v.findings
}

type validator struct {
	findings []wsvalidate.Finding
}

func (v *validator) add(path, msg string) {
	v.findings = append(v.findings, wsvalidate.Finding{
		Severity: wsvalidate.Error, Code: "feed_profile.invalid", Message: msg, Path: path,
	})
}

// text checks a required name: present, not padded with blanks, and free of
// control characters. It reports whether the name passed.
func (v *validator) text(path, s string, required bool) bool {
	name := path[strings.LastIndex(path, "/")+1:]
	switch {
	case s == "":
		if required {
			v.add(path, name+" is required")
		}
		return false
	case strings.TrimSpace(s) != s:
		v.add(path, fmt.Sprintf("%s %q begins or ends with a blank", name, s))
		return false
	case strings.IndexFunc(s, unicode.IsControl) >= 0:
		v.add(path, fmt.Sprintf("%s %q holds a control character", name, s))
		return false
	}
	return true
}

func (v *validator) atLeast(path string, n *int, min int) {
	if n != nil && *n < min {
		v.add(path, fmt.Sprintf("%s is %d and must be at least %d", path[1:], *n, min))
	}
}

// share checks a number between 0 and 1: zero allowed or not, one allowed.
func (v *validator) share(path string, x float64, zeroAllowed bool) {
	switch {
	case x > 1:
		v.add(path, fmt.Sprintf("%s is %g and must be at most 1", path[strings.LastIndex(path, "/")+1:], x))
	case x < 0 || (!zeroAllowed && x == 0):
		bound := "above 0"
		if zeroAllowed {
			bound = "at least 0"
		}
		v.add(path, fmt.Sprintf("%s is %g and must be %s", path[strings.LastIndex(path, "/")+1:], x, bound))
	}
}
