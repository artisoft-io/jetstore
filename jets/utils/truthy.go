package utils

import "strings"

// IsTruthy reports whether an environment variable's value asks for a feature to
// be turned on.
//
// The vocabulary is "1", "true", "yes" and "on", case-insensitive and trimmed.
// **Anything else is false, including an unset variable, an empty one and a
// typo** -- which is the whole point rather than a convenience. Deployment
// switches here are read from a container environment the CDK populates
// unconditionally: build_ui_service.go builds its task-definition environment
// with jsii.String(os.Getenv(...)) for every entry and filters nothing, so a
// variable the operator never set arrives *present and empty*. A presence test
// would therefore report "on" in every deployment the moment a switch joins that
// map, and a truthiness test that accepted any non-empty string would fire on a
// typo. Both failure modes are silent and site-wide.
//
// **This is the shared home for a rule that had two callers and one
// implementation.** It came from jets/userflow, where it exists because the same
// rule is implemented in validate.ts and the two must agree character for
// character; userflow.IsTruthy now delegates here so that correspondence has one
// place to change rather than two. See jets/userflow/validate.go.
func IsTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
