package stack

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// tagValueRe is the set of characters AWS accepts in a tag value on ECS, Lambda,
// Step Functions, S3, RDS and most other services: letters, spaces, numbers and
// _ . / = + - : @. ECS refuses a task definition whose tag holds anything else.
var tagValueRe = regexp.MustCompile(`^[\p{L}\p{Z}\p{N}_./=+\-:@]*$`)

// ec2TagValues are values on EC2 resources, which accept any character.
var ec2TagValues = map[string]bool{
	"JetStore Infer Persistent Volume (LLM Models)": true,
}

// descriptionTagRe finds a description tag's literal value in the stack's sources,
// written inline or as the agentic detection task's constant.
var descriptionTagRe = regexp.MustCompile(`(?s)Add\(descriptionTagName,\s*jsii\.String\("([^"]*)"\)`)

func TestDescriptionTagValuesAreValid(t *testing.T) {
	values := []string{agenticDetectDescription}
	files, err := filepath.Glob("../*.go")
	if err != nil {
		t.Fatal(err)
	}
	local, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, local...)
	n := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range descriptionTagRe.FindAllStringSubmatch(string(b), -1) {
			values = append(values, m[1])
			n++
		}
	}
	if n < 10 {
		t.Fatalf("found %d description tags in the sources, expected more: has the pattern drifted?", n)
	}
	for _, v := range values {
		if !ec2TagValues[v] && !tagValueRe.MatchString(v) {
			t.Errorf("tag value %q has a character AWS refuses (allowed: letters, spaces, numbers and _ . / = + - : @)", v)
		}
	}
}
