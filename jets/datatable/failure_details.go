package datatable

// Decoding the state machine's failure object.
//
// A Step Functions error object carries two fields, `Error` and `Cause`. `Error` is
// where States.Timeout, States.TaskFailed and a Lambda's exception type live: a
// closed, machine-readable class the platform computes at no cost. Until this file
// the status-update lambda read `Cause` and never `Error`, so the one structured
// failure class JetStore is handed was discarded in favour of prose, and the record
// had no error class column anywhere else either.
//
// Two things are recovered here and both are free:
//
//   - FailureClass is the `Error` field verbatim. Verbatim rather than mapped: the
//     vocabulary is AWS's, it grows with the platform, and a mapping table here
//     would be a second thing to keep in step for no gain.
//   - FailureSource says which arm of the decoder produced FailureDetails. The
//     decoder has four arms and the text they produce is differently shaped -- a
//     Lambda's errorMessage, an ECS StoppedReason, a Cause that would not parse,
//     the whole object re-serialised -- and nothing recorded which one fired, so
//     the shape of the prose was undeclared.
//
// The decode is a pure function of the argument so that it can be tested without a
// lambda, a state machine or a database; the lambda calls it and stores the result.

import (
	"encoding/json"
	"fmt"
)

// FailureSource values. A closed vocabulary: every arm of DecodeFailureDetails
// returns exactly one of these, and FailureSourceNone is what a run that did not
// carry a failure object gets.
const (
	// FailureSourceNone: no failure object was passed, or it was of a type the
	// decoder does not know. FailureDetails is then empty.
	FailureSourceNone = "none"
	// FailureSourcePlainString: failureDetails arrived as a bare string.
	FailureSourcePlainString = "plain_string"
	// FailureSourceLambdaErrorMessage: Cause parsed as JSON and carried
	// errorMessage -- a Lambda function's own error.
	FailureSourceLambdaErrorMessage = "lambda_error_message"
	// FailureSourceEcsStoppedReason: StoppedReason was found, on the parsed Cause
	// or on the failure object itself -- an ECS task that stopped.
	FailureSourceEcsStoppedReason = "ecs_stopped_reason"
	// FailureSourceCauseText: Cause was present but is not JSON, or is JSON with
	// neither errorMessage nor StoppedReason. The text is kept as it stands.
	FailureSourceCauseText = "cause_text"
	// FailureSourceUnstructured: no Cause and no StoppedReason; the whole object
	// is re-serialised, which is what the decoder did before it said so.
	FailureSourceUnstructured = "unstructured"
)

// FailureInfo is what the record keeps about a failed run: the prose that was
// already stored, plus the class the platform computed and the arm that produced
// the prose.
type FailureInfo struct {
	Details string
	Class   string
	Source  string
}

// DecodeFailureDetails reads the state machine's failureDetails argument. It
// reproduces the decoder the status-update lambda carried, arm for arm and text for
// text, and adds the two fields the arms were throwing away.
func DecodeFailureDetails(failureDetails any) FailureInfo {
	switch v := failureDetails.(type) {
	case string:
		return FailureInfo{Details: v, Source: FailureSourcePlainString}

	case map[string]any:
		info := FailureInfo{}
		// The Error field is read whichever arm below fires: a timeout and a task
		// failure carry different Cause shapes and the same Error field.
		if errorClass, ok := v["Error"].(string); ok {
			info.Class = errorClass
		}
		cause, causeOk := v["Cause"].(string)
		if causeOk {
			// Looks like an error in an ecs task or lambda function;
			// see if the text is an embedded json document.
			var causeDetails map[string]any
			if err := json.Unmarshal([]byte(cause), &causeDetails); err == nil {
				if txt2, ok2 := causeDetails["errorMessage"].(string); ok2 {
					// got down to the error message, must have been a lambda
					info.Details = txt2
					info.Source = FailureSourceLambdaErrorMessage
					return info
				}
				if taskReason, ok3 := causeDetails["StoppedReason"].(string); ok3 {
					// Looks like an error in a task container
					info.Details = stoppedReasonText(taskReason, causeDetails)
					info.Source = FailureSourceEcsStoppedReason
					return info
				}
				// unknown error structure, keep the whole thing
				info.Details = cause
				info.Source = FailureSourceCauseText
				return info
			}
			// must have been a simple string
			info.Details = cause
			info.Source = FailureSourceCauseText
			return info
		}
		if reason, ok := v["StoppedReason"].(string); ok {
			// Looks like an error in a task container
			info.Details = stoppedReasonText(reason, v)
			info.Source = FailureSourceEcsStoppedReason
			return info
		}
		// failure details has an unknown structure
		b, _ := json.MarshalIndent(v, "", " ")
		info.Details = string(b)
		info.Source = FailureSourceUnstructured
		return info

	default:
		return FailureInfo{Source: FailureSourceNone}
	}
}

// stoppedReasonText is the ECS arm's message, unchanged: the reason, qualified by
// the task group when one is present.
func stoppedReasonText(reason string, from map[string]any) string {
	if group, ok := from["Group"].(string); ok {
		return fmt.Sprintf("%s from %s", reason, group)
	}
	return reason
}
