package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Mock readFileFunc for testing
func mockReadFile(files map[string]string) readFileFunc {
	return func(filePath string) (string, error) {
		parts := strings.Split(filePath, "/")
		fileName := parts[len(parts)-1]
		content, ok := files[fileName]
		if !ok {
			return "", errors.New("file not found: " + fileName)
		}
		return content, nil
	}
}

func TestRuleFileReader_SimpleFile(t *testing.T) {
	files := map[string]string{
		"main.rules": "rule1\nrule2",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("Combined Content:\n", content)
	if !strings.Contains(content, "rule1") || !strings.Contains(content, "rule2") {
		t.Errorf("content missing rules: %s", content)
	}
	if !strings.Contains(content, "@JetCompilerDirective source_file = \"main.rules\";") {
		t.Errorf("missing compiler directive: %s", content)
	}
	// t.Error("Done")
}

func TestRuleFileReader_ImportFile(t *testing.T) {
	files := map[string]string{
		"main.rules":   "import \"imp1.rules\"\nmain_rule",
		"imp1.rules":   "imp_rule1\nimp_rule2",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("Combined Content:\n", content)
	if !strings.Contains(content, "imp_rule1") || !strings.Contains(content, "imp_rule2") {
		t.Errorf("imported rules missing: %s", content)
	}
	if !strings.Contains(content, "main_rule") {
		t.Errorf("main rule missing: %s", content)
	}
	if strings.Count(content, "@JetCompilerDirective source_file =") != 2 {
		t.Errorf("expected 2 compiler directives, got: %s", content)
	}
	// t.Error("Done")
}

func TestRuleFileReader_CircularImport(t *testing.T) {
	files := map[string]string{
		"main.rules":   "import \"imp1.rules\"\nmain_rule",
		"imp1.rules":   "import \"main.rules\"\nimp_rule1",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("Combined Content:\n", content)
	if strings.Count(content, "@JetCompilerDirective source_file =") != 2 {
		t.Errorf("circular import should not duplicate: %s", content)
	}
	if !strings.Contains(content, "main_rule") || !strings.Contains(content, "imp_rule1") {
		t.Errorf("rules missing: %s", content)
	}
	// t.Error("Done")
}

func TestRuleFileReader_MissingFile(t *testing.T) {
	files := map[string]string{
		"main.rules": "import \"missing.rules\"\nmain_rule",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	_, err := r.ReadAll()
	if err == nil {
		t.Errorf("expected error for missing file")
	}
	fmt.Println("Error:", err)
	// t.Error("Done")
}

func TestRuleFileReader_MultipleImports(t *testing.T) {
	files := map[string]string{
		"main.rules":   "import \"imp1.rules\"\nimport \"imp2.rules\"\nmain_rule",
		"imp1.rules":   "imp1_rule",
		"imp2.rules":   "imp2_rule",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("Combined Content:\n", content)
	if !strings.Contains(content, "imp1_rule") || !strings.Contains(content, "imp2_rule") {
		t.Errorf("imported rules missing: %s", content)
	}
	if !strings.Contains(content, "main_rule") {
		t.Errorf("main rule missing: %s", content)
	}
	// t.Error("Done")
}

func TestRuleFileReader_GetLocalFileAndLine(t *testing.T) {
	files := map[string]string{
		"main.rules": "line1\nimport \"imp1.rules\"\nline3\nline4",
		"imp1.rules": "imp_line1\nimp_line2",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("Combined Content:")
	for i := range strings.Split(content, "\n") {
		fmt.Printf("%2d: %s\n", i+1, strings.Split(content, "\n")[i])
	}
	r.PrintImportedFiles()

	tests := []struct {
		globalLine int
		expFile    string
		expLine    int
	}{
		{2, "main.rules", 1},
		{4, "imp1.rules", 1},
		{5, "imp1.rules", 2},
		{6, "main.rules", 3},
		{7, "main.rules", 4},
	}

	for _, tt := range tests {
		fileName, lineNum, err := r.GetLocalFileAndLine(tt.globalLine)
		if err != nil {
			t.Errorf("unexpected error for global line %d: %v", tt.globalLine, err)
			continue
		}
		if fileName != tt.expFile || lineNum != tt.expLine {
			t.Errorf("global line %d: expected (%s,%d), got (%s,%d)", tt.globalLine, tt.expFile, tt.expLine, fileName, lineNum)
		}
	}
	t.Error("Done")
}
