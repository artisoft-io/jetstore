package rulesbench

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	_ "modernc.org/sqlite"
)

// What the network holds, observed (rules_benchmark Q-98, BU.2). With
// Options.ObserveDir set, each session writes one JSON file: per rule set, per
// vertex, the rows of its beta relation when the rule set ended and at their most,
// the times its consequents were computed and retracted, and the rules the vertex
// serves, from workspace.db. The counters are the C++ session's own
// (get_vertex_observations), kept whether or not anything reads them, so a run
// that does not observe pays nothing; one that does reads them after each rule
// set, in no timed phase. An observing run is untimed all the same, as an
// equivalence-only run is: CheckOptions refuses the timing options with it.

// ObserveSchema names the observation file's format.
const ObserveSchema = "rules_bench_observe/1"

// SessionObservation is one session's observation file.
type SessionObservation struct {
	Schema     string               `json:"schema"`
	Bench      string               `json:"bench"`
	Engine     string               `json:"engine"`
	Session    string               `json:"session"`
	Repetition int                  `json:"repetition"`
	Warmup     bool                 `json:"warmup,omitzero"`
	RuleSets   []RuleSetObservation `json:"rule_sets"`
}

// RuleSetObservation is one rule set of a session, observed when its loop and
// exception scan are done and before its rete session is released.
type RuleSetObservation struct {
	// RuleSet is the main rule file, as workspace_control.source_file_name names it.
	RuleSet string `json:"rule_set"`
	// SourceFileKey is its workspace_control.key, which rete_nodes.source_file_key
	// holds. The join to rete_nodes is on (source_file_key, vertex) with type =
	// 'antecedent' ('head_node' for vertex 0): the two columns alone are not a key,
	// a vertex having a row per consequent as well. -1 when workspace.db has no row.
	SourceFileKey int `json:"source_file_key"`
	// RowsNow and RowsPeak are the sums over the vertices.
	RowsNow  int64               `json:"rows_now"`
	RowsPeak int64               `json:"rows_peak"`
	Vertices []VertexObservation `json:"vertices"`
	Err      string              `json:"error,omitempty"`
}

// VertexObservation is one vertex: the engine's counters and what workspace.db
// says of it. RowsNow and RowsPeak count the relation's row set, withdrawn rows
// waiting to be erased included; the vertex visit limit counts Inferred only.
type VertexObservation struct {
	compute_pipes.VertexObservation
	// ParentVertex is -1 for the head vertex, by choice: its head_node row says 0.
	ParentVertex int `json:"parent_vertex"`
	// Label is rete_nodes.normalizedLabel, the antecedent term.
	Label string `json:"label,omitempty"`
	// Rules are the jet_rules names whose antecedents run through the vertex
	// (rule_terms), in name order: more than one where rules share a prefix.
	Rules []string `json:"rules,omitempty"`
}

// vertexInfo is what workspace.db says of one antecedent vertex.
type vertexInfo struct {
	parent int
	label  string
	rules  []string
}

// NetworkInfo is workspace.db's description of each main rule set's antecedent
// vertices, keyed by rule set file and vertex.
type NetworkInfo struct {
	fileKey  map[string]int
	vertices map[string]map[int]*vertexInfo
}

// LoadNetworkInfo reads the antecedent vertices of every main rule set of the
// workspace.db at path, with the rules each serves.
func LoadNetworkInfo(path string) (*NetworkInfo, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("workspace.db: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ni := &NetworkInfo{fileKey: map[string]int{}, vertices: map[string]map[int]*vertexInfo{}}
	rows, err := db.Query(`SELECT wc.source_file_name, wc.key, rn.vertex, coalesce(rn.parent_vertex, -1),
	    coalesce(rn.normalizedLabel, ''), coalesce(jr.name, '')
	  FROM workspace_control wc
	  JOIN rete_nodes rn ON rn.source_file_key = wc.key AND rn.type = 'antecedent'
	  LEFT JOIN rule_terms rt ON rt.rete_node_key = rn.key
	  LEFT JOIN jet_rules jr ON jr.key = rt.rule_key
	  WHERE wc.is_main = 1
	  ORDER BY wc.source_file_name, rn.vertex, jr.name`)
	if err != nil {
		return nil, fmt.Errorf("workspace.db %s: %w", path, err)
	}
	defer rows.Close()
	for rows.Next() {
		var file, label, rule string
		var key, vertex, parent int
		if err := rows.Scan(&file, &key, &vertex, &parent, &label, &rule); err != nil {
			return nil, err
		}
		ni.fileKey[file] = key
		vs := ni.vertices[file]
		if vs == nil {
			vs = map[int]*vertexInfo{}
			ni.vertices[file] = vs
		}
		v := vs[vertex]
		if v == nil {
			v = &vertexInfo{parent: parent, label: label}
			vs[vertex] = v
		}
		if rule != "" && (len(v.rules) == 0 || v.rules[len(v.rules)-1] != rule) {
			v.rules = append(v.rules, rule)
		}
	}
	return ni, rows.Err()
}

// observeRuleSet reads a rule set's rete session.
func observeRuleSet(ni *NetworkInfo, ruleSet string, reteSession compute_pipes.JetReteSession) RuleSetObservation {
	ro := RuleSetObservation{RuleSet: ruleSet, SourceFileKey: -1, Vertices: []VertexObservation{}}
	if ni != nil {
		if k, ok := ni.fileKey[ruleSet]; ok {
			ro.SourceFileKey = k
		}
	}
	observer, ok := reteSession.(compute_pipes.VertexObserver)
	if !ok {
		ro.Err = "this engine's rete session cannot be observed"
		return ro
	}
	obs, err := observer.ObserveVertices()
	if err != nil {
		ro.Err = err.Error()
		return ro
	}
	for _, o := range obs {
		v := VertexObservation{VertexObservation: o, ParentVertex: -1}
		if ni != nil {
			if info := ni.vertices[ruleSet][o.Vertex]; info != nil {
				v.ParentVertex, v.Label, v.Rules = info.parent, info.label, info.rules
			}
		}
		ro.RowsNow += o.RowsNow
		ro.RowsPeak += o.RowsPeak
		ro.Vertices = append(ro.Vertices, v)
	}
	return ro
}

// ObservePath is where a session's observation file goes:
// <dir>/<r01|w01>/<engine>/<session>.json, as the dump tree is laid out.
func ObservePath(dir string, p DumpPoint) string {
	return filepath.Join(dir, p.RunDir(), p.Engine, safeName(p.Session)+".json")
}

func writeObservation(path string, so *SessionObservation) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(so, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// observeErrs gathers the rule sets that could not be observed, for the session's error.
func observeErrs(so *SessionObservation) string {
	var errs []string
	for _, ro := range so.RuleSets {
		if ro.Err != "" {
			errs = append(errs, ro.RuleSet+": "+ro.Err)
		}
	}
	return strings.Join(errs, "; ")
}
