package utils

import "strings"

// SanitizeArgs sanitizes the command line arguments to prevent command injection
// It returns a new slice of arguments that are safe to use in exec.Command
func SanitizeArgs(args []string) []string {
	sanitized := make([]string, len(args))
	for i, arg := range args {
		// Remove any potentially dangerous characters from the argument
		sanitized[i] = sanitizeArg(arg)
	}
	return sanitized
}

// sanitizeArg removes potentially dangerous characters from a single argument
func sanitizeArg(arg string) string {
	// Replace any occurrences of shell metacharacters with an underscore
	replacer := strings.NewReplacer(
		"`", "_",
		"$", "_",
		";", "_",
		"&", "_",
		"|", "_",
		"<", "_",
		">", "_",
		"\n", "_",
		"\r", "_",
		"\t", "_",
	)
	return replacer.Replace(arg)
}