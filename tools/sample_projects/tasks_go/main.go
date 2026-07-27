// Command tasks_go sends a prompt to a local Ollama server and validates the
// model's JSON response against a JSON Schema.
//
// It is the Go twin of ../tasks_py (Pydantic) and ../tasks_ts (TypeBox): the
// same JSON Schema (task_schema.json) is used both to constrain Ollama's
// structured output (the request "format" field) and to validate the response
// client-side.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// taskSchema is the JSON Schema describing a []Task value. It is embedded so the
// sample is self-contained and always validates against the exact schema sent
// to Ollama.
//
//go:embed task_schema.json
var taskSchema []byte

const defaultPromptFile = "/home/michel/projects/repos/workspaces/jets_ws/data/" +
	"patient_clinical_summary/initial_test_prompt_with_model.md"

// promptRe splits a prompt file into its "System:" and "User:" sections.
var promptRe = regexp.MustCompile(`(?s)\s*System:\s*(.*?)\n\s*User:\s*(.*)`)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parsePrompt splits a prompt file's contents into system and user sections.
func parsePrompt(text string) (system, user string, err error) {
	m := promptRe.FindStringSubmatch(text)
	if m == nil {
		return "", "", fmt.Errorf("prompt file must contain a 'System:' and a 'User:' section")
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), nil
}

// chatRequest is the payload sent to Ollama's /api/chat endpoint.
type chatRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Format   json.RawMessage `json:"format"`
	Options  map[string]any  `json:"options"`
	Messages []chatMessage   `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the (subset of the) response returned by /api/chat.
type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error"`
}

// callOllama posts a chat request to Ollama and returns the message content.
func callOllama(ctx context.Context, host, model, system, user string) (string, error) {
	payload := chatRequest{
		Model:  model,
		Stream: false,
		// Constrain generation to our JSON Schema.
		Format:  json.RawMessage(taskSchema),
		Options: map[string]any{"temperature": 0},
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encoding request: %w", err)
	}

	// Print the payload for debugging purposes.
	pretty, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Fprintf(os.Stderr, "Payload sent to Ollama:\n%s\n", pretty)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach Ollama at %s: %w", host, err)
	}
	defer resp.Body.Close()

	var data chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if data.Error != "" {
		return "", fmt.Errorf("ollama error: %s", data.Error)
	}
	return data.Message.Content, nil
}

// stripIDs recursively removes "$id" keys from a decoded JSON Schema document.
//
// The schema (mirrored from a TypeScript type) reuses the same "$id": "Status"
// label in each Task variant. Ollama tolerates that, but a strict draft
// 2020-12 validator rejects the duplicate anchors. Since the label is not used
// for any "$ref", dropping it is safe and lets us validate the response.
func stripIDs(v any) any {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "$id")
		for _, val := range t {
			stripIDs(val)
		}
	case []any:
		for _, val := range t {
			stripIDs(val)
		}
	}
	return v
}

// compileSchema compiles the embedded JSON Schema into a validator.
func compileSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(taskSchema))
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("task_schema.json", stripIDs(doc)); err != nil {
		return nil, fmt.Errorf("adding schema: %w", err)
	}
	return c.Compile("task_schema.json")
}

func run() error {
	host := getenv("OLLAMA_HOST", "http://localhost:11434")
	model := getenv("OLLAMA_MODEL", "gemma4:latest")

	promptFile := defaultPromptFile
	if len(os.Args) > 1 {
		promptFile = os.Args[1]
	}

	raw, err := os.ReadFile(promptFile)
	if err != nil {
		return fmt.Errorf("reading prompt file: %w", err)
	}
	system, user, err := parsePrompt(string(raw))
	if err != nil {
		return err
	}

	fmt.Printf("Model      : %s\n", model)
	fmt.Printf("Ollama host: %s\n", host)
	fmt.Printf("Prompt file: %s\n\n", promptFile)

	schema, err := compileSchema()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, err := callOllama(ctx, host, model, system, user)
	if err != nil {
		return err
	}

	// Parse the model output as JSON, then validate it against the schema.
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return fmt.Errorf("model output is not valid JSON: %w\n\nRaw output:\n%s", err, content)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("model output failed schema validation:\n%v\n\nRaw output:\n%s", err, content)
	}

	tasks, _ := value.([]any)
	pretty, _ := json.MarshalIndent(value, "", "  ")
	fmt.Printf("Validated %d task(s):\n\n%s\n", len(tasks), pretty)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
