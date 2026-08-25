// Package workspace_assets installs the JetStore-owned workspace assets into a
// client workspace, and refuses to do it silently over a local edit (A21.2,
// A21.3; plan section 3.9).
//
// The assets are committed under data_model/ and pipes_config/ — the data-model
// generator itself stays off the deployment path, so nothing here runs Python
// and nothing here reads the workspace it is installing into. They are embedded
// rather than copied out of the source tree so the install depends on the binary
// alone: `Dockerfile.cpipes_builder`'s `COPY jets` puts the files in the builder
// image, `go:embed` puts them in the binary, and `Dockerfile.compile_ws` needs to
// know no path but the workspace's.
//
// # Two directories, one mechanism
//
// data_model/ carries the platform base model and the generated agentic model.
// pipes_config/ carries the pipeline configurations JetStore owns — jets_loader,
// which every workspace had its own identical copy of, and the pipelines the
// platform itself runs. Both are installed the same way and guarded the same
// way; they differ in one detail only, which is that a .jr file can carry a
// comment header and a .pc.json cannot carry one that survives a round trip
// through a JSON editor. See TokenedAssets below.
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
//   - different, but matching the hash the last install recorded in that
//     directory's jets_assets_manifest.json — stale, not edited, so the new
//     version replaces it. This is how a JetStore change reaches a client;
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
// In data_model, a file with no ownership token in it is diagnosed differently
// from one that has the token but unexpected content: the first was never
// installed by JetStore, the second was and has been edited since. In
// pipes_config that distinction is not available — a `.pc.json` is machine-read
// configuration and carries no header — so the manifest is the only evidence
// there, and the diagnosis says so rather than claiming the file is not
// JetStore's. Both cases still refuse; only the wording differs.
//
// # The guard is checked across both directories before anything is written
//
// A conflict anywhere refuses the whole install, not the directory it was found
// in. A workspace with a good data model and an edited pipeline configuration is
// no more consistent than one with the reverse, and reporting half of it would
// invite a second build to discover the other half.
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

// The glob deliberately does not embed README.md: what is in a directory and
// what is installed into a workspace are not the same set, and each README is
// JetStore's own documentation of the convention (A21.4).
//
// user_flows names its four suffixes rather than taking *.json, and that is the
// same rule one step further on: the directory is where a generator writes, so
// the glob has to say what a document is instead of trusting whatever lands
// there. A .pc.json dropped beside a projection would otherwise install as a
// user flow.
//
//go:embed data_model/*.jr data_model/*.json pipes_config/*.json
//go:embed user_flows/*.uf.json user_flows/*.ua.json user_flows/*.form.json user_flows/*.apply.json
var assetFS embed.FS

// AssetGroup is one directory of JetStore-owned assets. Dir is both where the
// assets live here and where they are installed inside the workspace.
type AssetGroup struct {
	Dir string

	// TokenedAssets says every asset in this group carries OwnershipToken, so
	// its absence from a file at a reserved path is evidence that JetStore
	// never put that file there. It is true for data_model, whose assets are
	// .jr and hand-editable JSON written with a header, and false for
	// pipes_config: a .pc.json is read by the pipeline builder and by the
	// workspace IDE, and a token planted in it would be one more field for a
	// config author — or a config-generating model — to reproduce or lose.
	// Where it is false the manifest carries the whole of the evidence.
	//
	// It is false for user_flows too, and for a harder reason than
	// pipes_config's (task U.2, 2026-08-25). A .uf.json has nowhere to put a
	// token: userflow.schema.json, form.schema.json and action.schema.json all
	// set additionalProperties false at their root, so a field JetStore adds
	// for its own bookkeeping is refused by the save-path validator. The
	// browser is no help either, in the direction that reads as help — the Zod
	// objects are not strict, so a token would be stripped there rather than
	// reported, and a round trip through the editor would quietly remove the
	// evidence. The untokened case is therefore two cases wearing one flag —
	// one where a token would be unwelcome and one where it is not expressible
	// — and the manifest carries both.
	TokenedAssets bool

	// Doc is the remedy printed when this group has a conflict. The advice is
	// group-specific because the extension mechanisms are: a data model is
	// extended by importing it, a pipeline configuration by copying it under a
	// name of your own and registering that name, and a projected user flow by
	// copying the whole four-document set under a key of your own.
	Doc string
}

// AssetGroups are the directories installed, in install order. Adding one is
// adding a row here plus a line to the embed glob above — and, if its assets
// carry no header, a note in the group's README saying the manifest is the only
// evidence the guard has.
//
// Three as of 2026-08-25, and the third is the first whose assets are written
// by a generator rather than by a person (task U.2). That is the condition I-61
// named as the moment its second asymmetry would matter: `jets-agentic generate
// --check` accounts for data_model alone, and there is now a second generator
// writing into a second asset directory with nothing checking that what is
// committed is what the generator would emit. The check is owed to
// cpipes-contract rather than to this file, and is not written here.
var AssetGroups = []AssetGroup{
	{
		Dir:           "data_model",
		TokenedAssets: true,
		Doc: "Put your declarations in your own file that imports the JetStore one:\n\n" +
			"    # data_model/<your_workspace>_model.jr\n" +
			"    import \"data_model/jets_model.jr\"\n" +
			"    ... your resources, classes and triples ...\n\n" +
			"then point your rule files at your file instead. Imports are resolved\n" +
			"textually before the parse, so the result is identical to editing in place.",
	},
	{
		Dir:           "pipes_config",
		TokenedAssets: false,
		Doc: "A pipeline configuration JetStore owns is registered in process_config by\n" +
			"JetStore (jets/jets_init_db.sql) and is replaced on every build. To run a\n" +
			"variant, copy it under a name of your own:\n\n" +
			"    cp pipes_config/jets_loader.pc.json pipes_config/<your_workspace>_loader.pc.json\n\n" +
			"and register that file in your workspace's own process_config row. A copy\n" +
			"is not kept in step with the JetStore original; that is the trade.",
	},
	{
		Dir:           "user_flows",
		TokenedAssets: false,
		Doc: "The wizard for a JetStore template is generated from the template, by\n" +
			"`cpipes-contract templates --project jets/workspace_assets/user_flows`, and\n" +
			"is replaced on every build. Editing one here is editing the output of a\n" +
			"generator, so the change is lost at the next regeneration and the config the\n" +
			"wizard writes stops matching what the template expands to.\n\n" +
			"To run a variant, author your own flow beside it:\n\n" +
			"    cp user_flows/qc_metrics.uf.json user_flows/<your_workspace>_qc.uf.json\n\n" +
			"and its .ua.json, .form.json and .apply.json under the same key — the four\n" +
			"are read together and a set with three of them does not load. Your key is\n" +
			"yours; the escape name in the .ua.json is the build's and must stay\n" +
			"cpipesTemplateApply.",
	},
}

const (
	// ManifestName records what the last install wrote into one directory. It
	// belongs to the workspace, not to JetStore, which is why it is not itself
	// an asset: each workspace's copy describes that workspace's last install
	// and is committed with the assets it describes. There is one per group.
	ManifestName = "jets_assets_manifest.json"

	// OwnershipToken is planted in every data_model asset by the emitters'
	// header (A21.4, tools/jets_agentic/jets_agentic/header.py). Its absence
	// from a file at a reserved path in a TokenedAssets group means JetStore
	// never put that file there.
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
	Dir    string
	Name   string
	Action Action
}

// Conflict is one file the install refuses to overwrite.
type Conflict struct {
	Dir    string
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
	seen := map[string]bool{}
	for _, c := range e.Conflicts {
		fmt.Fprintf(&b, "  %s: %s\n", filepath.Join(c.Dir, c.Name), c.Reason)
		seen[c.Dir] = true
	}
	b.WriteString(
		"\nThese paths are owned by JetStore and are overwritten on every build, so a\n" +
			"change made in place is lost and is not what the compiled workspace contains.\n")
	// Only the groups that actually conflicted get their remedy printed; the
	// other one's advice would be noise at the moment someone is reading this.
	for _, g := range AssetGroups {
		if seen[g.Dir] {
			b.WriteString("\n" + g.Doc + "\n")
		}
	}
	b.WriteString(
		"\nIf the difference is not yours — a copy predating this mechanism — install\n" +
			"once with -force to adopt the JetStore version and commit the result.")
	return b.String()
}

// Names lists the assets in one group, sorted.
func Names(dir string) ([]string, error) {
	entries, err := assetFS.ReadDir(dir)
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
func Asset(dir, name string) ([]byte, error) {
	return assetFS.ReadFile(filepath.Join(dir, name))
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

// Install writes every asset into its directory under workspaceDir, refusing to
// overwrite a file that is neither JetStore's nor what the last install left.
// On conflict nothing is written at all — the guard is checked over every group
// before the first byte lands, so a failed install does not leave a workspace
// half-updated in one directory or updated in one and not the other.
func Install(workspaceDir string, opts Options) ([]Result, error) {
	if info, err := os.Stat(workspaceDir); err != nil {
		return nil, fmt.Errorf("workspace %s: %w", workspaceDir, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("workspace %s is not a directory", workspaceDir)
	}

	var (
		results   []Result
		conflicts []Conflict
		// pending is what would be written, per group, once every group has
		// been found clean.
		pending   = map[string]map[string][]byte{}
		manifests = map[string]manifest{}
	)

	for _, g := range AssetGroups {
		names, err := Names(g.Dir)
		if err != nil {
			return nil, err
		}
		targetDir := filepath.Join(workspaceDir, g.Dir)
		prev, err := readManifest(targetDir)
		if err != nil {
			return nil, err
		}
		groupPending := map[string][]byte{}
		next := manifest{Comment: manifestComment, Assets: map[string]string{}}

		for _, name := range names {
			want, err := Asset(g.Dir, name)
			if err != nil {
				return nil, err
			}
			wantSum := sum(want)
			next.Assets[name] = wantSum
			path := filepath.Join(targetDir, name)

			got, err := os.ReadFile(path)
			switch {
			case os.IsNotExist(err):
				groupPending[name] = want
				results = append(results, Result{g.Dir, name, Installed})
				continue
			case err != nil:
				return nil, err
			}

			gotSum := sum(got)
			switch {
			case gotSum == wantSum:
				results = append(results, Result{g.Dir, name, Unchanged})
			case opts.Force || prev.Assets[name] == gotSum:
				groupPending[name] = want
				results = append(results, Result{g.Dir, name, Updated})
			default:
				conflicts = append(conflicts, Conflict{
					Dir: g.Dir, Name: name, Path: path,
					Reason: reason(got, prev.Assets[name] != "", g.TokenedAssets),
				})
			}
		}
		pending[g.Dir] = groupPending
		manifests[g.Dir] = next
	}

	if len(conflicts) > 0 {
		return results, &ConflictError{WorkspaceDir: workspaceDir, Conflicts: conflicts}
	}
	if opts.DryRun {
		return results, nil
	}
	for _, g := range AssetGroups {
		targetDir := filepath.Join(workspaceDir, g.Dir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, err
		}
		for name, data := range pending[g.Dir] {
			if err := os.WriteFile(filepath.Join(targetDir, name), data, 0644); err != nil {
				return nil, err
			}
		}
		if err := writeManifest(targetDir, manifests[g.Dir]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// reason names which conflict this is, because the fix differs. In a group
// whose assets carry no ownership token there are only two states to tell
// apart, not three, and saying "JetStore did not install this" on the evidence
// of a missing header JetStore never writes would be a false diagnosis.
func reason(got []byte, hadManifestEntry bool, tokened bool) string {
	if tokened && !bytes.Contains(got, []byte(OwnershipToken)) {
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
