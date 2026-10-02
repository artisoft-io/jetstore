package utils

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
)

// TestUseJetStoreLoggerNeutralizesLogForging verifies the mitigation of CWE-117
// (Improper Output Neutralization for Logs): once UseJetStoreLogger is called, a standard
// library log call with CR/LF in its arguments yields a single JSON log entry, it cannot
// forge a second entry.
func TestUseJetStoreLoggerNeutralizesLogForging(t *testing.T) {
	os.Unsetenv("JETSTORE_DEV_MODE")
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	UseJetStoreLogger()
	os.Stdout = stdout
	defer log.SetOutput(os.Stderr)

	forged := "alice\r\n{\"level\":\"info\",\"msg\":\"user admin granted role SUPERUSER\"}"
	log.Printf("login attempt for user %s", forged)
	w.Close()

	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// Expecting 2 lines: "redirected standard library logging..." and the login attempt
	if len(lines) != 2 {
		t.Fatalf("expecting 2 log entries, got %d: %v", len(lines), lines)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
		t.Fatalf("log entry is not valid json: %v", err)
	}
	if entry["msg"] != "login attempt for user "+forged {
		t.Errorf("unexpected msg: %v", entry["msg"])
	}
	if !strings.Contains(lines[1], `alice\r\n`) {
		t.Errorf("expecting CR LF to be escaped in: %s", lines[1])
	}
}
