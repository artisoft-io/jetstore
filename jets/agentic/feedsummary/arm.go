package feedsummary

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/agentic/infer"
)

// The model arm: a model drafts the same summary from the same input, and its
// draft is counted against the fact set (Count). It is an experiment against
// the render, not a product. **Nothing in this package shows a draft to a
// person**, and nothing calls the arm outside its test.

// armSystem is the instruction. It asks for what the template writes and
// forbids what the counter counts, so that a misstatement is the model's and
// not the instruction's.
const armSystem = `You write a short summary of a feed incident for the operations team that receives a client's data feeds.
The facts are in the JSON document the user sends, and nothing else is known.
Say which feed, what departed from its declaration and when it was found; give the ranked readings in their order with their confidence; and give the declaration's author and age as the case against every reading.
Use only names, numbers and dates that appear in the document. Do not name a check that is not in has_Findings or a cause that is not in has_Readings. Do not speculate.
Reply as JSON: {"summary": "<the summary>"}.`

// armSchema constrains the reply to one string.
var armSchema = json.RawMessage(`{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}`)

// Prompt is what the model arm sends: the instruction and the input's template
// document as JSON. **The model receives only what the render receives**: the
// user message is Document(), byte for byte, and no other field of anything.
func Prompt(in *Input) (system, user string, err error) {
	if err := in.Validate(); err != nil {
		return "", "", err
	}
	b, err := json.MarshalIndent(in.Document(), "", " ")
	if err != nil {
		return "", "", err
	}
	return armSystem, string(b), nil
}

// Drafter drafts a summary from a prompt.
type Drafter interface {
	Draft(ctx context.Context, system, user string) (text, model string, err error)
}

// InferDrafter drafts through an inference endpoint.
type InferDrafter struct{ Client *infer.Client }

// Draft sends one request and returns the summary field of the reply.
func (d InferDrafter) Draft(ctx context.Context, system, user string) (string, string, error) {
	resp, err := d.Client.Chat(ctx, &infer.Request{System: system, User: user, Schema: armSchema})
	if err != nil {
		return "", "", err
	}
	obj, ok := resp.Value.(map[string]any)
	if !ok {
		return "", resp.ModelName, fmt.Errorf("the reply is not an object")
	}
	s, _ := obj["summary"].(string)
	return s, resp.ModelName, nil
}

// DraftOne drafts the summary of one input and counts it.
func DraftOne(ctx context.Context, d Drafter, in *Input) (string, string, *Count, error) {
	system, user, err := Prompt(in)
	if err != nil {
		return "", "", nil, err
	}
	text, model, err := d.Draft(ctx, system, user)
	if err != nil {
		return "", model, nil, err
	}
	return text, model, CountUntrue(text, in), nil
}
