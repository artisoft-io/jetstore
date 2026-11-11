package git

import "testing"

// The active workspace is refused with a message spelled correctly; it read
// "invaid request".
func TestDeleteWorkspaceRefusesTheActiveWorkspace(t *testing.T) {
	wg := &WorkspaceGit{WorkspaceName: "ws1", ActiveWorkspace: "ws1", WorkspacesHome: t.TempDir()}
	err := wg.DeleteWorkspace()
	if err == nil || err.Error() != "invalid request, cannot delete the active workspace" {
		t.Errorf("got %v", err)
	}
}
