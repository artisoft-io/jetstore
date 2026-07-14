package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateWorkspaceName ensures the workspace name is a safe single path segment
// that cannot be interpreted as a command-line option or used for path traversal.
func ValidateWorkspaceName(workspaceName string) (string, error) {
	workspaceName = strings.TrimSpace(workspaceName)
	if workspaceName == "" {
		return "", fmt.Errorf("workspace name is required")
	}
	if workspaceName != filepath.Base(workspaceName) || workspaceName == "." || workspaceName == ".." {
		return "", fmt.Errorf("invalid workspace name: %q", workspaceName)
	}
	return workspaceName, nil
}
