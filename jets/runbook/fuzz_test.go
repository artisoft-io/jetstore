package runbook

import (
	"os"
	"testing"
)

// Decode, Validate and Bind return findings and errors; they do not panic,
// whatever the content.
func FuzzDecodeValidateBind(f *testing.F) {
	if readme, err := os.ReadFile("README.md"); err == nil {
		if m := fence.FindStringSubmatch(string(readme)); m != nil {
			f.Add(m[2], "SELECT :since_utc::date, ':x' -- :y\n; DO $$ :z $$")
		}
	}
	f.Add(`{"format":1,"runbook":"r","steps":[{"id":"a","type":"assert","condition":{"not":{"all":[{"count":"x.y","in":[1]}]}}}]}`, "SELECT ${x} {{y}} $1 E'\\'' \"q\"")
	f.Add(`{"steps":[{"type":"repeat","steps":[{"type":"repeat","steps":[null]}]}],"parameters":{"p":null}}`, "/* /* */")
	f.Fuzz(func(t *testing.T, content, sql string) {
		rb, _ := Load("r.rb.json", []byte(content), map[string]string{"r/a.sql": sql})
		_ = ValidateFile(content)
		env := MapEnv{"params.x": {Type: Type{Base: TypeText}, V: "v"}}
		_, _ = Bind(sql, &Step{Bind: map[string]string{"y": "params.x"}}, env)
		if rb == nil {
			return
		}
		rb.Walk(func(s, _ *Step) bool {
			for _, c := range []*Condition{s.When, s.Condition, s.Until} {
				if c != nil {
					_, _ = c.Eval(env)
				}
			}
			return true
		})
	})
}
