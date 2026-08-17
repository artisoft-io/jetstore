// Package workspace_assets installs the JetStore-owned workspace assets into a
// client workspace, and refuses to do it silently over a local edit (A21.2,
// A21.3; plan section 3.9).
//
// The assets are generated in JetStore CI and committed under
// data_model/ — the generator itself stays off the deployment path, so nothing
// here runs Python and nothing here reads the workspace it is installing into.
// They are embedded rather than copied out of the source tree so the install
// depends on the binary alone: `Dockerfile.cpipes_builder`'s `COPY jets` puts
// the files in the builder image, `go:embed` puts them in the binary, and
// `Dockerfile.compile_ws` needs to know no path but the workspace's.
//
// # The guard
//
// A client workspace commits its copy of these files, because a knowledge
// engineer running compile_workspace locally needs them present (section 3.9.4).
// Committed and overwritten at build time is the whole arrangement: the copy is
// for ergonomics, the install is for authority. That leaves three states an
// existing file can be in, and they must not be treated alike:
//
//   - identical to what is being installed — the normal case, nothing to do;
//   - different, but matching the hash the last install recorded in
//     jets_assets_manifest.json — stale, not edited, so the new version replaces
//     it. This is how a JetStore model change reaches a client;
//   - different, and *not* what the last install left behind — someone edited a
//     file they do not own, or the path was theirs before JetStore claimed it.
//     The install fails, naming every such file, and the build stops.
//
// The third case is not hypothetical. `workspaces/usi_ws/data_model/jets_model.jr`
// carried a four-line `iWorkBook` extension appended in place, load-bearing in
// three rule trees; an install that overwrote it would have produced a compile
// failure on an undeclared resource somewhere far from the cause. That is the
// outcome this guard exists to prevent, and A21.5 is the migration that answers
// it — the extension moves to a client-owned file that imports the JetStore one.
//
// A file with no ownership token in it is diagnosed differently from one that
// has the token but unexpected content: the first was never installed by
// JetStore, the second was and has been edited since.
package workspace_assets

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The glob deliberately does not embed README.md: what is in the directory and
// what is installed into a workspace are not the same set, and the README is
// JetStore's own documentation of the convention (A21.4).
//
//go:embed data_model/*.jr data_model/*.json
var assetFS embed.FS

const (
	// AssetDir is both the directory the assets live in here and the directory
	// they are installed into inside the workspace.
	AssetDir = "data_model"

	// ManifestName records what the last install wrote. It belongs to the
	// workspace, not to JetStore, which is why it is not itself an asset: each
	// workspace's copy describes that workspace's last install and is committed
	// with the assets it describes.
	ManifestName = "jets_assets_manifest.json"

	// OwnershipToken is planted in every asset by the emitters' header (A21.4,
	// tools/jets_agentic/jets_agentic/header.py). Its absence from a file at a
	// reserved path means JetStore never put that file there.
	OwnershipToken = "jetstore-owned-asset"

	manifestComment = "Written by install_workspace_assets. Records the hash of each " +
		"JetStore-owned asset as installed, so the next install can tell a stale copy " +
		"(replaced) from a locally edited one (build fails). Commit it with the assets."
)

// Action is what the install did to one asset.
type Action string

const (
	Installed Action = "installed" // the workspace had no copy
	Updated   Action = "updated"   // the workspace had an older, unedited copy
	Unchanged Action = "unchanged" // already byte-identical
)

// Result reports the disposition of one asset.
type Result struct {
	Name   string
	Action Action
}

// Conflict is one file the install refuses to overwrite.
type Conflict struct {
	Name   string
	Path   string
	Reason string
}

// ConflictError carries every conflict found, not just the first: a client
// finding out that one of their files was never theirs should find out about
// all of them in one build, not one per build.
type ConflictError struct {
	WorkspaceDir string
	Conflicts    []Conflict
}

func (e *ConflictError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to install %d JetStore-owned asset(s) over local changes in %s:\n",
		len(e.Conflicts), e.WorkspaceDir)
	for _, c := range e.Conflicts {
		fmt.Fprintf(&b, "  %s: %s\n", filepath.Join(AssetDir, c.Name), c.Reason)
	}
	b.WriteString(
		"\nThese paths are owned by JetStore and are overwritten on every build, so a\n" +
			"change made in place is lost and is not what the compiled workspace contains.\n" +
			"Put your declarations in your own file that imports the JetStore one:\n\n" +
			"    # data_model/<your_workspace>_model.jr\n" +
			"    import \"data_model/jets_model.jr\"\n" +
			"    ... your resources, classes and triples ...\n\n" +
			"then point your rule files at your file instead. Imports are resolved\n" +
			"textually before the parse, so the result is identical to editing in place.\n" +
			"If the difference is not yours — a copy predating this mechanism — install\n" +
			"once with -force to adopt the JetStore version and commit the result.")
	return b.String()
}

// Names lists the assets that get installed, sorted.
func Names() ([]string, error) {
	entries, err := assetFS.ReadDir(AssetDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Asset returns one embedded asset's bytes.
func Asset(name string) ([]byte, error) {
	return assetFS.ReadFile(filepath.Join(AssetDir, name))
}

type manifest struct {
	Comment string            `json:"_comment"`
	Assets  map[string]string `json:"assets"`
}

// Options tunes an install. The zero value is what the build runs.
type Options struct {
	// DryRun reports what would happen and writes nothing.
	DryRun bool
	// Force adopts the JetStore version over a conflicting file. It exists for
	// the one-time adoption of a workspace whose copies predate the manifest,
	// and is not for the build: a build that needs it is a build that is about
	// to lose someone's work.
	Force bool
}

// Install writes every asset into workspaceDir/data_model, refusing to
// overwrite a file that is neither JetStore's nor what the last install left.
// On conflict nothing is written at all — the guard is checked over the whole
// set before the first byte lands, so a failed install does not leave a
// workspace half-updated.
func Install(workspaceDir string, opts Options) ([]Result, error) {
	names, err := Names()
	if err != nil {
		return nil, err
	}
	targetDir := filepath.Join(workspaceDir, AssetDir)
	if info, err := os.Stat(workspaceDir); err != nil {
		return nil, fmt.Errorf("workspace %s: %w", workspaceDir, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("workspace %s is not a directory", workspaceDir)
	}

	prev, err := readManifest(targetDir)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(names))
	pending := make(map[string][]byte, len(names))
	next := manifest{Comment: manifestComment, Assets: map[string]string{}}
	var conflicts []Conflict

	for _, name := range names {
		want, err := Asset(name)
		if err != nil {
			return nil, err
		}
		wantSum := sum(want)
		next.Assets[name] = wantSum
		path := filepath.Join(targetDir, name)

		got, err := os.ReadFile(path)
		switch {
		case os.IsNotExist(err):
			pending[name] = want
			results = append(results, Result{name, Installed})
			continue
		case err != nil:
			return nil, err
		}

		gotSum := sum(got)
		switch {
		case gotSum == wantSum:
			results = append(results, Result{name, Unchanged})
		case opts.Force || prev.Assets[name] == gotSum:
			pending[name] = want
			results = append(results, Result{name, Updated})
		default:
			conflicts = append(conflicts, Conflict{name, path, reason(got, prev.Assets[name] != "")})
		}
	}

	if len(conflicts) > 0 {
		return results, &ConflictError{WorkspaceDir: workspaceDir, Conflicts: conflicts}
	}
	if opts.DryRun {
		return results, nil
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, err
	}
	for name, data := range pending {
		if err := os.WriteFile(filepath.Join(targetDir, name), data, 0644); err != nil {
			return nil, err
		}
	}
	return results, writeManifest(targetDir, next)
}

// reason names which of the two conflicts this is, because the fix differs.
func reason(got []byte, hadManifestEntry bool) string {
	if !bytes.Contains(got, []byte(OwnershipToken)) {
		return "a file JetStore did not install (it carries no " + OwnershipToken +
			" header) sits at a path JetStore owns"
	}
	if hadManifestEntry {
		return "a JetStore-installed file has been modified since it was installed"
	}
	return "a JetStore-owned file differs from the one being installed, and no " +
		ManifestName + " entry says what the last install left here"
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func readManifest(targetDir string) (manifest, error) {
	m := manifest{Assets: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(targetDir, ManifestName))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		// A manifest we cannot read is not a reason to guess: treat it as
		// absent, which makes every difference a conflict rather than an
		// overwrite.
		return manifest{Assets: map[string]string{}}, nil
	}
	if m.Assets == nil {
		m.Assets = map[string]string{}
	}
	return m, nil
}

func writeManifest(targetDir string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, ManifestName), append(data, '\n'), 0644)
}
