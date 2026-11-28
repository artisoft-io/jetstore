package workspace_assets

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// InstalledHash reports the hash the last install recorded for the workspace
// file fileName, a workspace-relative path such as `user_flows/flow.uf.json`,
// in the workspace at workspaceDir. owned is true when the file is directly in
// one of AssetGroups' directories and that directory's manifest
// (ManifestName) has an entry for it: the file is a JetStore-owned asset, and
// hash is the SHA-256, in hex, of the version JetStore installed there.
//
// **The manifest is the evidence, not the embed.** A workspace JetStore never
// installed into has no manifest, and a JetStore fix cannot reach it through an
// install, so nothing an override of one of its files masks is JetStore's to
// report. And the manifest says what *this* workspace holds, which is what an
// override replaces at a start, whichever release wrote it.
//
// It is what tells a stale override from a current one (JM-033): an override
// of an owned asset records hash when it is saved, and a later hash that
// differs means JetStore has changed the asset under it since
// (wsfile.CheckOverrideBases). It reads the manifest only; an unreadable
// manifest is treated as absent, as Install treats it.
func InstalledHash(workspaceDir, fileName string) (hash string, owned bool, err error) {
	dir, name := path.Split(path.Clean(strings.ReplaceAll(fileName, "\\", "/")))
	dir = strings.TrimSuffix(dir, "/")
	if name == "" || name == ManifestName {
		return "", false, nil
	}
	for _, g := range AssetGroups {
		if g.Dir != dir {
			continue
		}
		m, err := readManifest(filepath.Join(workspaceDir, g.Dir))
		if err != nil {
			return "", false, err
		}
		hash, owned = m.Assets[name]
		return hash, owned && hash != "", nil
	}
	return "", false, nil
}

// InstalledFile is a workspace file the next install writes: an asset the
// last install recorded, or a group's manifest.
type InstalledFile struct {
	// Group is the asset group whose directory holds the file.
	Group AssetGroup
	// Manifest is true for the group's ManifestName, which every install
	// writes, recorded or not.
	Manifest bool
}

// InstalledFiles returns the files of the workspace whose root is workspace
// that the next install of JetStore's assets writes over, by
// workspace-relative path (`user_flows/flow.uf.json`): every name a group's
// manifest records, as InstalledHash reads it, and every group's manifest.
// The workspace IDE marks them as installed by JetStore (JM-389, 2026-11-27,
// wsfile.markInstalledAssets). They stay editable: a save is a local override,
// as for any other file.
//
// **The manifest is the evidence, as for InstalledHash**, and not the embed:
// a manifest records what the install wrote into this workspace, and the next
// install replaces each name it records with the version it ships, or deletes
// it when JetStore has retired it. A file of the workspace's own beside the
// assets is in no manifest. A recorded file the workspace's repository has
// changed is not written over: the install refuses it and the build stops.
//
// A workspace JetStore never installed into has no manifest, and nothing in
// it is returned but the manifests' own paths.
func InstalledFiles(workspace fs.FS) (map[string]InstalledFile, error) {
	files := map[string]InstalledFile{}
	for _, g := range AssetGroups {
		files[g.Dir+"/"+ManifestName] = InstalledFile{Group: g, Manifest: true}
		m, err := readManifestFS(workspace, g.Dir)
		if err != nil {
			return nil, err
		}
		for name, hash := range m.Assets {
			if hash == "" || name == ManifestName || name == "." || name == ".." || name != path.Base(name) {
				continue
			}
			files[g.Dir+"/"+name] = InstalledFile{Group: g}
		}
	}
	return files, nil
}

// readManifestFS is readManifest for the group directory dir of the workspace
// workspace.
func readManifestFS(workspace fs.FS, dir string) (manifest, error) {
	data, err := fs.ReadFile(workspace, path.Join(dir, ManifestName))
	return decodeManifest(data, err)
}
