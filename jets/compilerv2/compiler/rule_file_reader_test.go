package compiler

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Mock readFileFunc for testing
func mockReadFile(files map[string]string) readFileFunc {
	return func(baseDir, fileName string) (string, error) {
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
		"main.rules": "import \"imp1.rules\"\nmain_rule",
		"imp1.rules": "imp_rule1\nimp_rule2",
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
	// Three, not two: main.rules, then imp1.rules, then main.rules again when
	// the import returns. Without the third, main_rule is attributed to
	// imp1.rules by the listener.
	if strings.Count(content, "@JetCompilerDirective source_file =") != 3 {
		t.Errorf("expected 3 compiler directives, got: %s", content)
	}
	// t.Error("Done")
}

func TestRuleFileReader_CircularImport(t *testing.T) {
	files := map[string]string{
		"main.rules": "import \"imp1.rules\"\nmain_rule",
		"imp1.rules": "import \"main.rules\"\nimp_rule1",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Println("Combined Content:\n", content)
	// main.rules, imp1.rules, and main.rules again on the return. The circular
	// import contributes no fourth: imp1.rules' import of main.rules is skipped,
	// and resuming imp1.rules re-marks a file that is already in effect.
	if strings.Count(content, "@JetCompilerDirective source_file =") != 3 {
		t.Errorf("circular import should not duplicate: %s", content)
	}
	if strings.Count(content, "main_rule") != 1 {
		t.Errorf("circular import included main.rules twice: %s", content)
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
		"main.rules": "import \"imp1.rules\"\nimport \"imp2.rules\"\nmain_rule",
		"imp1.rules": "imp1_rule",
		"imp2.rules": "imp2_rule",
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
	filesContent := make(map[string][]string)
	for k, v := range files {
		filesContent[k] = strings.Split(v, "\n")
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fmt.Println("Files Content:")
	for fileName, lines := range filesContent {
		fmt.Printf("File: %s\n", fileName)
		for i, line := range lines {
			fmt.Printf("%2d: %s\n", i+1, line)
		}
	}

	fmt.Println("Combined Content:")
	combinedContent := strings.Split(content, "\n")
	for i := range combinedContent {
		fmt.Printf("%2d: %s\n", i+1, combinedContent[i])
	}
	r.PrintImportedFiles()

	for i, line := range combinedContent {
		if strings.HasPrefix(line, "@JetCompilerDirective") || len(line) == 0 {
			continue
		}
		globalLine := i + 1
		fileName, localLine, err := r.GetLocalFileAndLine(globalLine)
		if err != nil {
			t.Errorf("error getting local file and line for global line %d: %v", globalLine, err)
			continue
		}
		fmt.Printf("Global line %d -> File: %s, Local line: %d\n", globalLine, fileName, localLine)
		if line != filesContent[fileName][localLine-1] {
			t.Errorf("line mismatch at global line %d: expected %q, got %q", globalLine, filesContent[fileName][localLine-1], line)
		}
	}
	// t.Error("Done")
}

func TestRuleFileReader_GetLocalFileAndLine2(t *testing.T) {
	files := map[string]string{
		"main.rules": "line1\nline2\nimport \"imp1.rules\"\nline3\nline4",
		"imp1.rules": "imp1_line1\nimp1_line2\nimport \"imp2.rules\"\nimp1_line3\nimp1_line4",
		"imp2.rules": "imp2_line1\nimp2_line2",
	}
	filesContent := make(map[string][]string)
	for k, v := range files {
		filesContent[k] = strings.Split(v, "\n")
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fmt.Println("Files Content:")
	for fileName, lines := range filesContent {
		fmt.Printf("File: %s\n", fileName)
		for i, line := range lines {
			fmt.Printf("%2d: %s\n", i+1, line)
		}
	}

	fmt.Println("Combined Content:")
	combinedContent := strings.Split(content, "\n")
	for i := range combinedContent {
		fmt.Printf("%2d: %s\n", i+1, combinedContent[i])
	}
	r.PrintImportedFiles()

	for i, line := range combinedContent {
		if strings.HasPrefix(line, "@JetCompilerDirective") || len(line) == 0 {
			continue
		}
		globalLine := i + 1
		fileName, localLine, err := r.GetLocalFileAndLine(globalLine)
		if err != nil {
			t.Errorf("error getting local file and line for global line %d: %v", globalLine, err)
			continue
		}
		fmt.Printf("Global line %d -> File: %s, Local line: %d\n", globalLine, fileName, localLine)
		if line != filesContent[fileName][localLine-1] {
			t.Errorf("line mismatch at global line %d: expected %q, got %q", globalLine, filesContent[fileName][localLine-1], line)
		}
	}
	// t.Error("Done")
}

// Every line's source_file directive must name the file that line actually came
// from. It did not: the directive was written when a file was entered and never
// re-written when an import returned, so everything after an import in a file
// was attributed to the file it had imported. In usi_ws that made
// build/classes.json report every class declared after line 4 of
// data_model/usi_data_model.jr as belonging to data_model/jets_model.jr.
//
// The check is a cross-check rather than a fixture: the directive in effect at
// each line is compared with GetLocalFileAndLine's answer for the same line,
// which is derived independently.
func TestRuleFileReader_DirectiveMatchesLineAttribution(t *testing.T) {
	files := map[string]string{
		"main.rules":  "main_line1\nimport \"model.rules\"\nmain_line2\nmain_line3",
		"model.rules": "model_line1\nimport \"base.rules\"\nmodel_line2",
		"base.rules":  "base_line1\nbase_line2",
	}
	r := NewRuleFileReader("", "main.rules", mockReadFile(files))
	content, err := r.ReadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inEffect := ""
	for i, line := range strings.Split(content, "\n") {
		if name := directiveFileName(line); name != "" {
			inEffect = name
			continue
		}
		if len(line) == 0 {
			continue
		}
		fileName, _, err := r.GetLocalFileAndLine(i + 1)
		if err != nil {
			t.Errorf("global line %d (%q): %v", i+1, line, err)
			continue
		}
		if inEffect != fileName {
			t.Errorf("line %q is attributed to %s by the directive in effect, but belongs to %s",
				line, inEffect, fileName)
		}
	}
}

func directiveFileName(line string) string {
	const prefix = "@JetCompilerDirective source_file = \""
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	rest := line[len(prefix):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}
