package workspace_assets

import (
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
// It is what tells a stale override from a current one: an override
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
