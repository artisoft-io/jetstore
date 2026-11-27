package observe

import "fmt"

// Source is where a run's main input came from: the client, org and object
// type of its main input registration, the jetsapi.input_registry row its
// pipeline_execution_status.main_input_registry_key names. A process that reads
// the files of two vendors runs on two sources, and a run is judged against the
// runs of its own source, so that one vendor's runs are not expected to have
// the steps only another vendor's files reach.
//
// **The org alone is not a source.** Where a workspace reads internal files
// rather than a vendor's, the org is a generic one spanning several object
// types, so it is the three together that name what the run read.
//
// The zero Source is a run with no main input registration: a job's row, a run
// planted without one, or a run whose registration was deleted with its
// session. Every such run of a process has the same, empty, source.
//
// It is not the file's object type a session's file_key_staging row carries,
// which the incident list shows as the file's: the source is the domain key
// the main input was registered under.
type Source struct {
	Client     string
	Org        string
	ObjectType string
}

// IsZero reports whether the run has no main input registration.
func (s Source) IsZero() bool { return s == Source{} }

// Describe renders the source for a basis.
func (s Source) Describe() string {
	if s.IsZero() {
		return "no main input source"
	}
	return fmt.Sprintf("main input source %s/%s/%s", s.Client, s.Org, s.ObjectType)
}

// SourceColumns are the three columns of a run's source, read through
// SourceJoin's alias src: empty strings, not NULL, where the run has no main
// input registration.
const SourceColumns = `coalesce(src.client, ''), coalesce(src.org, ''), coalesce(src.object_type, '')`

// noSourceColumns stands in for SourceColumns where jetsapi.input_registry does
// not exist, so a statement reads the same columns either way and every run has
// the empty source.
const noSourceColumns = `''::text, ''::text, ''::text`

// SourceJoin returns the join that reads the source of the run header aliased
// h: its main input registration. A LEFT JOIN, so a run with none is kept, with
// the empty source.
func SourceJoin(h string) string {
	return fmt.Sprintf("LEFT JOIN jetsapi.input_registry src ON src.key = %s.main_input_registry_key", h)
}

// SourceMatch returns the predicate that keeps the runs of one source, its
// three values at the given placeholders, in SourceJoin's alias.
func SourceMatch(client, org, objectType int) string {
	return fmt.Sprintf(`coalesce(src.client, '') = $%d AND coalesce(src.org, '') = $%d AND coalesce(src.object_type, '') = $%d`,
		client, org, objectType)
}
