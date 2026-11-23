package audit

import (
	"context"
	"fmt"
)

// IncidentInput is one input registered under an incident's session: the file
// key and the kind of source, as jetsapi.input_registry records them.
//
// **It is what lets a person go from an incident to the pipeline runs behind it.**
// An incident names a session and nothing else about its input, while the
// Pipeline Status screen finds runs by file key or by session. For a file, the
// session is the one the file was registered under, which a run that read the
// file carries as its input session; for a domain table it is the run's own
// session. The source type says which of the two to look for.
type IncidentInput struct {
	FileKey string
	// SourceType is input_registry.source_type: "file" or "domain_table".
	SourceType string
}

// maxIncidentInputs bounds what one incident detail carries. A file's session
// registers one file key (two when it was registered again); a run's session
// registers a domain table per output, which can be many.
const maxIncidentInputs = 20

// IncidentInputs returns the distinct inputs registered under sessionID, files
// first, then by file key, at most maxIncidentInputs of them. None is an empty
// list, not an error: a session that registered nothing has nothing to show.
func IncidentInputs(ctx context.Context, db Querier, sessionID string) ([]IncidentInput, error) {
	rows, err := db.Query(ctx,
		`SELECT file_key, source_type
		   FROM jetsapi.input_registry
		  WHERE session_id = $1
		  GROUP BY file_key, source_type
		  ORDER BY source_type <> 'file', source_type, file_key
		  LIMIT $2`, sessionID, maxIncidentInputs)
	if err != nil {
		return nil, fmt.Errorf("while reading the inputs of session %s: %w", sessionID, err)
	}
	defer rows.Close()
	out := []IncidentInput{}
	for rows.Next() {
		var fileKey, sourceType *string
		if err := rows.Scan(&fileKey, &sourceType); err != nil {
			return nil, fmt.Errorf("while reading an input of session %s: %w", sessionID, err)
		}
		in := IncidentInput{}
		if fileKey != nil {
			in.FileKey = *fileKey
		}
		if sourceType != nil {
			in.SourceType = *sourceType
		}
		out = append(out, in)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the inputs of session %s: %w", sessionID, err)
	}
	return out, nil
}
