package datatable

import "testing"

// The decoder's arms, each with the class the platform supplies alongside.
//
// Every case here is shaped the way Step Functions actually delivers it: the error
// object carries Error and Cause side by side, and the Cause is a JSON *string*
// rather than an object. The two things being asserted are that the prose is
// unchanged from what the lambda produced before this decoder existed, and that the
// class and the arm are now recorded beside it.
func TestDecodeFailureDetails(t *testing.T) {
	tests := []struct {
		name       string
		in         any
		wantText   string
		wantClass  string
		wantSource string
	}{
		{
			name:       "no failure object",
			in:         nil,
			wantText:   "",
			wantClass:  "",
			wantSource: FailureSourceNone,
		},
		{
			name:       "plain string",
			in:         "something went wrong",
			wantText:   "something went wrong",
			wantClass:  "",
			wantSource: FailureSourcePlainString,
		},
		{
			name: "lambda error message, with the exception type as the class",
			in: map[string]any{
				"Error": "Function.ResponseSizeTooLarge",
				"Cause": `{"errorType":"Function.ResponseSizeTooLarge","errorMessage":"response payload too large"}`,
			},
			wantText:   "response payload too large",
			wantClass:  "Function.ResponseSizeTooLarge",
			wantSource: FailureSourceLambdaErrorMessage,
		},
		{
			name: "ecs stopped reason inside a cause, qualified by its group",
			in: map[string]any{
				"Error": "States.TaskFailed",
				"Cause": `{"StoppedReason":"OutOfMemoryError: Container killed due to memory usage","Group":"family:cpipes-node"}`,
			},
			wantText:   "OutOfMemoryError: Container killed due to memory usage from family:cpipes-node",
			wantClass:  "States.TaskFailed",
			wantSource: FailureSourceEcsStoppedReason,
		},
		{
			name: "ecs stopped reason on the object itself, no group",
			in: map[string]any{
				"StoppedReason": "Essential container in task exited",
			},
			wantText:   "Essential container in task exited",
			wantClass:  "",
			wantSource: FailureSourceEcsStoppedReason,
		},
		{
			name: "cause that is not json",
			in: map[string]any{
				"Error": "States.Timeout",
				"Cause": "the state machine timed out",
			},
			wantText:   "the state machine timed out",
			wantClass:  "States.Timeout",
			wantSource: FailureSourceCauseText,
		},
		{
			name: "cause that is json but names neither field",
			in: map[string]any{
				"Cause": `{"whatever":"else"}`,
			},
			wantText:   `{"whatever":"else"}`,
			wantClass:  "",
			wantSource: FailureSourceCauseText,
		},
		{
			name: "no cause and no stopped reason",
			in: map[string]any{
				"Error": "States.Timeout",
			},
			wantText: "{\n \"Error\": \"States.Timeout\"\n}",
			// The class is recovered even though the prose is the re-serialised
			// object -- which is the whole point: the least informative arm is the
			// one that most needs a class.
			wantClass:  "States.Timeout",
			wantSource: FailureSourceUnstructured,
		},
		{
			name:       "a type the decoder does not know",
			in:         42,
			wantText:   "",
			wantClass:  "",
			wantSource: FailureSourceNone,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeFailureDetails(tc.in)
			if got.Details != tc.wantText {
				t.Errorf("Details: got %q, want %q", got.Details, tc.wantText)
			}
			if got.Class != tc.wantClass {
				t.Errorf("Class: got %q, want %q", got.Class, tc.wantClass)
			}
			if got.Source != tc.wantSource {
				t.Errorf("Source: got %q, want %q", got.Source, tc.wantSource)
			}
		})
	}
}

// The Error field is read on every arm, not only on the ones that carry no prose.
// Written as its own test because it is the claim I-259 is about, and a table row
// asserting it once is easy to read past.
func TestDecodeFailureDetailsReadsErrorOnEveryArm(t *testing.T) {
	causes := []string{
		`{"errorMessage":"a lambda"}`,
		`{"StoppedReason":"a task"}`,
		`not json at all`,
	}
	for _, cause := range causes {
		got := DecodeFailureDetails(map[string]any{"Error": "States.TaskFailed", "Cause": cause})
		if got.Class != "States.TaskFailed" {
			t.Errorf("cause %q: Class got %q, want States.TaskFailed", cause, got.Class)
		}
		if got.Source == FailureSourceNone {
			t.Errorf("cause %q: Source should name an arm", cause)
		}
	}
}
