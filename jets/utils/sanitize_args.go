package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

// ConfineFilePath joins fileName onto baseDir and verifies the cleaned result
// stays within baseDir, mitigating external control of file name or path (CWE-73).
func ConfineFilePath(baseDir, fileName string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("while resolving base dir %q: %w", baseDir, err)
	}
	joined := filepath.Join(absBase, fileName)
	if joined != absBase && !strings.HasPrefix(joined, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid file path %q: escapes directory %q", fileName, baseDir)
	}
	return joined, nil
}

// SanitizeS3Prefix validates and cleans an externally-provided S3 key prefix to
// mitigate external control of file name or path (CWE-73). It rejects control
// characters, strips any leading '/', and rejects path traversal ("..") sequences.
func SanitizeS3Prefix(prefix string) (string, error) {
	if prefix == "" {
		return "", nil
	}
	// Reject embedded control characters (e.g. CR/LF, NUL).
	for _, r := range prefix {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("invalid s3 prefix: contains control character")
		}
	}
	// Strip leading '/' and reject path traversal sequences.
	cleaned := strings.TrimLeft(prefix, "/")
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") ||
		strings.Contains(cleaned, "/../") || strings.HasSuffix(cleaned, "/..") {
		return "", fmt.Errorf("invalid s3 prefix: path traversal detected")
	}
	return cleaned, nil
}

// VerifyUrlPath validates and cleans an externally-provided URL path to
// mitigate external control of file name or path (CWE-73). It rejects control
// characters, strips any leading '/', and rejects path traversal ("..") sequences.
func VerifyUrlPath(path string) error {
	if path == "" {
		return nil
	}
	// Reject embedded control characters (e.g. CR/LF, NUL).
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid url path: contains control character")
		}
	}
	// Reject path traversal sequences.
	if path == ".." || strings.HasPrefix(path, "../") ||
		strings.Contains(path, "/../") || strings.HasSuffix(path, "/..") {
		return fmt.Errorf("invalid url path: path traversal detected")
	}
	return nil
}
