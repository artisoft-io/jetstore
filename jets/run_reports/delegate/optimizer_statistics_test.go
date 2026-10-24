package delegate

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeS3 replaces optimizerStatsS3 with a map for the test's duration.
func fakeS3(t *testing.T, objects map[string][]byte) {
	t.Helper()
	saved := optimizerStatsS3
	t.Cleanup(func() { optimizerStatsS3 = saved })
	optimizerStatsS3.list = func(prefix string) ([]string, error) {
		var keys []string
		for k := range objects {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		return keys, nil
	}
	optimizerStatsS3.download = func(key string) ([]byte, error) {
		b, ok := objects[key]
		if !ok {
			return nil, errors.New("no such key")
		}
		return b, nil
	}
	optimizerStatsS3.upload = func(key string, buf []byte) error {
		objects[key] = append([]byte(nil), buf...)
		return nil
	}
}

// session is one session's statistics for a rule set with two predicates, its
// counts varying with n.
func session(n int) *optstats.RuleSet {
	w := optstats.NewWanted()
	w.AddAntecedent("", "rdf:type", "acme:Claim")
	w.AddAntecedent("", "acme:hasClaim", "")
	c, err := optstats.NewCounter(w, func(k string) (any, bool) { return k, true })
	if err != nil {
		panic(err)
	}
	for i := range n {
		claim := "acme:c" + string(rune('a'+i))
		c.Add(claim, "rdf:type", "acme:Claim")
		c.Add("acme:m", "acme:hasClaim", claim)
	}
	return c.Session()
}

func partial(t *testing.T, ns ...int) []byte {
	t.Helper()
	a := optstats.NewAccumulator()
	for _, n := range ns {
		if err := a.Add("jet_rules/acme_main.jr", session(n)); err != nil {
			t.Fatal(err)
		}
	}
	b, err := optstats.Marshal(a.File())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func statsArgs() *CommandArguments {
	return &CommandArguments{WorkspaceName: "jets_ws", ProcessName: "Optimizer_Statistics", SessionId: "1700000000"}
}

// The function merges every partial of the session exactly, writes the
// workspace's file, and lists its key; partials of another session are not read.
func TestMergeOptimizerStatistics(t *testing.T) {
	ca := statsArgs()
	prefix := optstats.PartialsPrefix(awsi.JetStoreStagePrefix(), ca.ProcessName, ca.SessionId)
	objects := map[string][]byte{
		prefix + "partition=none_node=0.json": partial(t, 1, 2),
		prefix + "partition=none_node=1.json": partial(t, 3),
		prefix + "partition=none_node=2.json": partial(t), // a node with no session
		prefix + "notes.txt":                  []byte("not a partial"),
		optstats.PartialsPrefix(awsi.JetStoreStagePrefix(), ca.ProcessName, "another") + "partition=none_node=0.json": partial(t, 9),
	}
	fakeS3(t, objects)
	var updated []string
	if err := MergeOptimizerStatistics((*pgxpool.Pool)(nil), ca, "", MergeOptimizerStatisticsFunc, &updated); err != nil {
		t.Fatal(err)
	}
	key := "jetstore/workspaces/jets_ws/jet_rules/optimizer_statistics.json"
	if len(updated) != 1 || updated[0] != key {
		t.Fatalf("updated keys %v, want [%s]", updated, key)
	}
	if want := partial(t, 1, 2, 3); !bytes.Equal(objects[key], want) {
		t.Errorf("merged file\n%s\nwant one accumulation over the same sessions\n%s", objects[key], want)
	}
}

func TestMergeOptimizerStatisticsRefusals(t *testing.T) {
	ca := statsArgs()
	prefix := optstats.PartialsPrefix(awsi.JetStoreStagePrefix(), ca.ProcessName, ca.SessionId)
	v2 := bytes.Replace(partial(t, 1), []byte(`"version": 1`), []byte(`"version": 2`), 1)
	for _, c := range []struct {
		name    string
		objects map[string][]byte
		want    string
	}{
		{"no partial", map[string][]byte{}, "no partial statistics under"},
		{"only empty partials", map[string][]byte{prefix + "a.json": partial(t)}, "hold no session"},
		{"another version", map[string][]byte{prefix + "a.json": partial(t, 1), prefix + "b.json": v2}, "version 2 is not supported"},
	} {
		fakeS3(t, c.objects)
		var updated []string
		err := MergeOptimizerStatistics(nil, ca, "", MergeOptimizerStatisticsFunc, &updated)
		if err == nil || !strings.Contains(err.Error(), c.want) || len(updated) != 0 {
			t.Errorf("%s: %v, updated %v; want an error containing %q and no key", c.name, err, updated, c.want)
		}
		if _, wrote := c.objects[optstats.WorkspaceKey("jets_ws")]; wrote {
			t.Errorf("%s: the workspace's file was written", c.name)
		}
	}
	if err := MergeOptimizerStatistics(nil, &CommandArguments{WorkspaceName: "jets_ws"}, "", "", new([]string)); err == nil {
		t.Errorf("no session: accepted")
	}
}

// The function is every JetStore's, under its name; a deployment's own function
// of the same name takes precedence.
func TestReportFuncHasTheBuiltIn(t *testing.T) {
	ca := &CommandArguments{}
	if ca.reportFunc(MergeOptimizerStatisticsFunc) == nil || ca.reportFunc("lookups.sql") != nil {
		t.Errorf("built-in lookup")
	}
	called := false
	ca.InstallSpecificFunc = map[string]InstallationSpecificReportFunc{
		MergeOptimizerStatisticsFunc: func(*pgxpool.Pool, *CommandArguments, string, string, *[]string) error {
			called = true
			return nil
		},
	}
	ca.reportFunc(MergeOptimizerStatisticsFunc)(nil, ca, "", "", nil)
	if !called {
		t.Errorf("the deployment's function did not take precedence")
	}
}

// The synced override: the statistics key becomes the workspace file
// jet_rules/optimizer_statistics.json, labelled jet_rules as the IDE labels it,
// whatever the report's output path; a lookup written under the workspace's
// folder is synced as before.
func TestWorkspaceOverride(t *testing.T) {
	for _, c := range []struct {
		key, outputPath, wantName, wantType string
	}{
		{optstats.WorkspaceKey("jets_ws"), "s3/output/client=x", "jet_rules/optimizer_statistics.json", "jet_rules"},
		{"jetstore/workspaces/jets_ws/lookups/drug_lookup.csv", "jetstore/workspaces/jets_ws", "lookups/drug_lookup.csv", "lookups"},
		{"jetstore/workspaces/jets_ws/top.csv", "jetstore/workspaces/jets_ws", "top.csv", "lookups"},
		{"jetstore/workspaces/other_ws/lookups/a.csv", "jetstore/workspaces/other_ws", "lookups/a.csv", "lookups"},
		{"s3/output/client=x/report.csv", "s3/output/client=x", "report.csv", "lookups"},
	} {
		trim, ct := workspaceOverride(c.key, "jets_ws", c.outputPath)
		if name := strings.TrimPrefix(c.key, trim); name != c.wantName || ct != c.wantType {
			t.Errorf("%s: file %q type %q, want %q %q", c.key, name, ct, c.wantName, c.wantType)
		}
	}
	// SyncS3Files writes the key's local copy where WORKSPACES_HOME replaces
	// jetstore/workspaces: the workspace's own jet_rules/optimizer_statistics.json.
	if got := strings.Replace(optstats.WorkspaceKey("jets_ws"), "jetstore/workspaces", "/home/ws", 1); got != "/home/ws/jets_ws/"+optstats.FileName {
		t.Errorf("local path %s", got)
	}
}

// A report's entry may carry a comment beside the directives, as jets_ws's
// statistics process does: the directives decode, and the comment is ignored.
func TestReportDirectivesWithAComment(t *testing.T) {
	var config map[string]ReportDirectives
	err := json.Unmarshal([]byte(`{"Optimizer_Statistics": {"comment": "why",
	  "reportScripts": ["merge_optimizer_statistics"], "updateLookupTables": true}}`), &config)
	rd := config["Optimizer_Statistics"]
	if err != nil || !rd.UpdateLookupTables || len(rd.ReportScripts) != 1 || rd.ReportScripts[0] != MergeOptimizerStatisticsFunc {
		t.Errorf("%v: %+v", err, rd)
	}
}
