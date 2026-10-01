package compute_pipes

import "testing"

// The jetstore_s3_output location moves a key into the output area. A key under the
// input area always moved; a key under the stage area now does too, so a pipeline whose
// main input is a part-file folder on stage writes its default output under output
// rather than beside the files it read. The same cases are in the Python node's
// tests_merge.py, against do_substitution, so the two engines agree on the rule.
func TestToOutputArea(t *testing.T) {
	const in, stage, out = "jetstore/input", "jetstore/stage", "jetstore/output"
	cases := []struct{ name, value, want string }{
		{"stage folder", stage + "/process_name=P/session_id=1/step_id=x", out + "/process_name=P/session_id=1/step_id=x"},
		{"the stage prefix itself", stage, out},
		{"input folder, as before", in + "/client=C/object_type=T", out + "/client=C/object_type=T"},
		{"a stage path not at the start is left alone", "custom/" + stage + "/x", "custom/" + stage + "/x"},
		{"a prefix that only begins like stage is left alone", stage + "_archive/x", stage + "_archive/x"},
		{"already in output", out + "/x", out + "/x"},
	}
	for _, c := range cases {
		if got := toOutputArea(c.value, in, stage, out); got != c.want {
			t.Errorf("%s: toOutputArea(%q) = %q, want %q", c.name, c.value, got, c.want)
		}
	}
	// Empty prefixes move nothing: strings.ReplaceAll with an empty old string would
	// insert the output prefix between every character.
	if got := toOutputArea("a/b", "", "", out); got != "a/b" {
		t.Errorf("empty prefixes: got %q, want %q", got, "a/b")
	}
}
