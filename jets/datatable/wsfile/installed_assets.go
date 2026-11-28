package wsfile

import (
	"log"
	"net/url"
	"os"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// The workspace IDE marks the files JetStore installs into a workspace
// (JM-389, Michel, 2026-11-27), and leaves them editable.
//
// **The install writes over them at every build** (workspace_assets.Install,
// run by the image build before the compile), so a change saved to one is a
// local override, and nothing in the editor said so. It still is: the save
// writes a row of jetsapi.workspace_changes, which every start applies over
// the newly installed file (SyncWorkspaceFiles, jets/workspace); Commit
// Workspace leaves it out of the commit, where the next install would refuse
// it (OwnedAssetOverrides, JM-032); and each start marks it in Workspace
// Changes when a JetStore release has changed the file under it
// (CheckOverrideBases, JM-033). What this adds is that the IDE says so before
// the save rather than after the release.
//
// **Making them read-only was decided and reversed the same day**: it would
// have taken away the one way to change a JetStore file in an installation
// without waiting for a release, which JM-032 and JM-033 exist to support.
//
// Which files they are is the manifests' to say (workspace_assets.InstalledFiles):
// the names the last install recorded. A file of the workspace's own beside
// the assets is recorded nowhere and is not marked.

// InstalledNote is what the IDE shows for a file JetStore installs: what the
// install does to it, and what a save of it becomes.
const InstalledNote = "Installed by JetStore, and replaced by every build. Saving it makes a " +
	"local override: it applies over the installed file at every start, Commit Workspace " +
	"leaves it out, and Workspace Changes marks it when a JetStore release changes the file. " +
	"Revert it there to return to JetStore's version."

// isInstalled reports whether the cleaned workspace path is one of the
// installed files, other than a manifest, which is the install's record rather
// than its output, and which nothing lists.
func isInstalled(installed map[string]workspace_assets.InstalledFile, cleaned string) bool {
	f, ok := installed[cleaned]
	return ok && !f.Manifest
}

// markInstalledAssets sets Installed and InstalledNote on every file node of
// nodes, at any depth, that is a file JetStore installed in the workspace whose
// root is root. ReadOnly is left as IsReadOnly set it. Manifests that cannot be
// read mark nothing.
func markInstalledAssets(root string, nodes []*WorkspaceNode) {
	installed, err := workspace_assets.InstalledFiles(os.DirFS(root))
	if err != nil {
		log.Printf("while reading which files JetStore installs in %q: %v", root, err)
		return
	}
	var mark func(nodes []*WorkspaceNode)
	mark = func(nodes []*WorkspaceNode) {
		for _, n := range nodes {
			if n == nil {
				continue
			}
			if n.Type == "file" {
				if filePath, err := url.QueryUnescape(n.PageMatchKey); err == nil &&
					isInstalled(installed, cleanWorkspacePath(filePath)) {
					n.Installed = true
					n.InstalledNote = InstalledNote
				}
			}
			if n.Children != nil {
				mark(*n.Children)
			}
		}
	}
	mark(nodes)
}
