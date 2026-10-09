package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	_ "modernc.org/sqlite"
)

// Workspace is the resolved handle the registry's tools operate on. Tools
// never see a path: the adapter (or a test) resolves a checkout once with
// ResolveWorkspace and hands the handle over — the registry interface must
// not acquire the stdio adapter's local-disk shape, or Phase 2 inherits a
// signature it cannot use (plan section 6).
type Workspace struct {
	// The workspace-wide class view, from build/classes.json (F5).
	Classes map[string]*rete.ClassNode
	// The jets_agentic sidecar, from data_model/jets_agentic.meta.json;
	// nil when the workspace has no schema-first model installed.
	Meta *Sidecar
	// workspace.db, opened read-only, for the metadata triples.
	db *sql.DB
	// dir is where this workspace was resolved from. Tools still never take
	// a path as an argument — that rule is about the tool *signature*, so a
	// Phase-2 catalogue is not born holding the stdio adapter's local-disk
	// shape — but a tool that has to run the compiler needs the rule sources
	// on a filesystem, and the handle is the right place to carry that. Empty
	// when the workspace was assembled some other way; LocalDir says so
	// rather than returning a path that is not there.
	dir string
}

// LocalDir is where the workspace's files live, for the one tool that needs
// them: compiling a rule file means resolving its imports, and imports are
// files. It fails rather than guessing when the handle has no local
// materialisation, which is the honest answer for a workspace that was never
// on this disk.
func (ws *Workspace) LocalDir() (string, error) {
	if ws == nil || ws.dir == "" {
		return "", fmt.Errorf(
			"this workspace has no local directory, so its rule sources cannot be read; " +
				"compiling requires a workspace resolved from a checkout")
	}
	return ws.dir, nil
}

// Sidecar mirrors jets_agentic.meta.json (the A1.6 emitter's shape).
type Sidecar struct {
	Model        string                       `json:"model"`
	Version      string                       `json:"version"`
	Prefix       string                       `json:"prefix"`
	Entities     map[string]SidecarEntity     `json:"entities"`
	Vocabularies map[string]SidecarVocabulary `json:"vocabularies"`
}

type SidecarEntity struct {
	Description string                 `json:"description"`
	Base        string                 `json:"base"`
	AsTable     bool                   `json:"as_table"`
	Properties  map[string]SidecarProp `json:"properties"`
}

type SidecarProp struct {
	Type               string `json:"type"`
	Array              bool   `json:"array"`
	Required           bool   `json:"required"`
	Description        string `json:"description"`
	Vocabulary         string `json:"vocabulary,omitempty"`
	Entity             string `json:"entity,omitempty"`
	DataClassification string `json:"data_classification,omitempty"`
	Key                bool   `json:"key,omitempty"`
}

type SidecarVocabulary struct {
	Properties  []string `json:"properties"`
	Values      []string `json:"values"`
	Description string   `json:"description"`
}

// ResolveWorkspace opens a compiled workspace checkout: build/classes.json
// is required (it is the F5 view tool 1 exists to prove reachable — a
// checkout without it has not been compiled, and the error says how to fix
// that), workspace.db is required, the sidecar is optional.
func ResolveWorkspace(dir string) (*Workspace, error) {
	ws := &Workspace{dir: dir}

	classesPath := filepath.Join(dir, "build", "classes.json")
	raw, err := os.ReadFile(classesPath)
	if err != nil {
		return nil, fmt.Errorf(
			"no workspace-wide class view at %s - compile the workspace first (compile_workspace writes it, no database needed): %w",
			classesPath, err)
	}
	if err := json.Unmarshal(raw, &ws.Classes); err != nil {
		return nil, fmt.Errorf("while parsing %s: %w", classesPath, err)
	}

	metaPath := filepath.Join(dir, "data_model", "jets_agentic.meta.json")
	if raw, err := os.ReadFile(metaPath); err == nil {
		ws.Meta = &Sidecar{}
		if err := json.Unmarshal(raw, ws.Meta); err != nil {
			return nil, fmt.Errorf("while parsing %s: %w", metaPath, err)
		}
	}

	dbPath := filepath.Join(dir, "workspace.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("no workspace.db at %s: %w", dbPath, err)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("while opening %s: %w", dbPath, err)
	}
	ws.db = db
	return ws, nil
}

func (ws *Workspace) Close() error {
	if ws.db != nil {
		return ws.db.Close()
	}
	return nil
}

// Triple is one metadata fact from the compiled workspace, resolved from
// its resource keys to readable values.
type Triple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// TriplesAbout returns the metadata triples whose subject is any of the
// given resource names (a class name, or its property names) — the
// rule-visible metadata channel of section 3.5, and the description source
// for .jr-authored classes.
func (ws *Workspace) TriplesAbout(subjects []string) ([]Triple, error) {
	if len(subjects) == 0 {
		return nil, nil
	}
	args := make([]any, len(subjects))
	marks := ""
	for i, s := range subjects {
		args[i] = s
		if i > 0 {
			marks += ","
		}
		marks += "?"
	}
	rows, err := ws.db.Query(`
		SELECT s.id, p.id, coalesce(o.id, o.value)
		FROM triples t
		JOIN resources s ON s.key = t.subject_key
		JOIN resources p ON p.key = t.predicate_key
		JOIN resources o ON o.key = t.object_key
		WHERE s.id IN (`+marks+`)
		ORDER BY s.id, p.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("while querying triples: %w", err)
	}
	defer rows.Close()
	var out []Triple
	for rows.Next() {
		var t Triple
		if err := rows.Scan(&t.Subject, &t.Predicate, &t.Object); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
