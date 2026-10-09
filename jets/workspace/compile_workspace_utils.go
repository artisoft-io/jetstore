package workspace

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable/git"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/run_reports/tarextract"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Active workspace prefix and control file path
var workspaceHome, wprefix, workspaceControlPath, workspaceBuildPath, workspaceVersion, localRepoVersion string
var devMode bool
var lastWorkspaceSyncCheck *time.Time

func init() {
	localRepoVersion = os.Getenv("JETS_VERSION")
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
	workspaceControlPath = fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, wprefix)
	workspaceBuildPath = fmt.Sprintf("%s/%s/build", workspaceHome, wprefix)

	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
}

func WorkspacesHome() string {
	return workspaceHome
}

func WorkspacePrefix() string {
	return wprefix
}

func WorkspaceDbPath() string {
	return fmt.Sprintf("%s/%s/workspace.db", workspaceHome, wprefix)
}

func LookupDbPath() string {
	return fmt.Sprintf("%s/%s/lookup.db", workspaceHome, wprefix)
}

func DevMode() bool {
	return devMode
}

func WorkspaceBuildDir() string {
	return workspaceBuildPath
}

func WorkspaceControlFilePath() string {
	return workspaceControlPath
}

// This file contains functions to compile and sync the workspace
// between jetstore database and the local container
// WORKSPACE Workspace default currently in use
// WORKSPACES_HOME Home dir of workspaces

// Function to pull override workspace files from databse to the
// container workspace (local copy).
// Need this when:
//   - starting a task requiring local workspace (e.g. run_report to get latest report definition)
//   - starting apiserver to get latest override files (e.g. lookup csv files) to compile workspace
//   - starting rule server to get the latest lookup.db and workspace.db
func SyncWorkspaceFiles(dbpool *pgxpool.Pool, workspaceName, contentType string, skipSqliteFiles bool, skipTgzFiles bool) (bool, error) {
	// sync workspace files from db to locally
	if devMode {
		return false, nil
	}
	// Get all file_name that are modified
	compilationRequired := false
	if len(contentType) > 0 {
		log.Printf("Start synching overriten workspace file with content_type '%s' from database", contentType)
	} else {
		log.Printf("Start synching overriten workspace file from database")
	}

	// Mitigating external control of file name or path (CWE-73) in workspaceName.
	workspaceName, err := utils.ValidateWorkspaceName(workspaceName)
	if err != nil {
		return false, err
	}

	fileObjects, err := dbutils.QueryFileObject(dbpool, workspaceName, contentType)
	if err != nil {
		return false, err
	}
	for _, fo := range fileObjects {
		// When in skipSqliteFiles == true, do not override lookup.db and workspace.db
		// When in skipTgzFiles == true, do not override *.tgz files
		if (!skipSqliteFiles || !strings.HasSuffix(fo.FileName, ".db")) &&
			(!skipTgzFiles || !strings.HasSuffix(fo.FileName, ".tgz")) {

			// Mitigating external control of file name or path (CWE-73) in resolveWorkspacePath
			// Confine the DB-provided file name within the workspace directory (CWE-73).
			baseDir := filepath.Join(workspaceHome, workspaceName)
			localFileName, err := resolveWorkspacePath(baseDir, fo.FileName)
			if err != nil {
				return false, err
			}
			// create workspace.tgz file and dir structure
			fileDir := filepath.Dir(localFileName)
			if err = utils.EnsureWorkspaceDir(fileDir); err != nil {
				return false, fmt.Errorf("while creating file directory structure: %v", err)
			}
			// Put obj to local file system
			err = fo.WriteDbObject2LocalFile(dbpool, localFileName)
			if err != nil {
				return false, err
			}
			// If FileName ends with .tgz, extract files from archive
			switch {
			case strings.HasSuffix(fo.FileName, ".tgz"):
				err = extractTgz(localFileName, baseDir)
				if err != nil {
					return false, err
				}
			case strings.HasSuffix(fo.FileName, ".db"):
			default:
				log.Printf("*** compilation required due to override of file %s", fo.FileName)
				compilationRequired = true
			}
		} else {
			log.Println("Skipping file", fo.FileName)
		}
	}
	log.Println("Done synching overriten workspace file from database, compilationRequired?", compilationRequired)
	return compilationRequired, nil
}

func extractTgz(sourceFileName, destBaseDir string) error {
	fileHd, err := os.Open(sourceFileName)
	if err != nil {
		return fmt.Errorf("failed to open tgz file %s for read: %v", sourceFileName, err)
	}
	defer fileHd.Close()
	err = tarextract.ExtractTarGz(fileHd, destBaseDir)
	if err != nil {
		return fmt.Errorf("failed to extract content from tgz file %s for read: %v", sourceFileName, err)
	}
	log.Printf("Extracted tgz file %s to %s", sourceFileName, destBaseDir)
	return nil
}

// resolveWorkspacePath joins fileName onto baseDir and verifies the result stays
// within baseDir, mitigating external control of file name or path (CWE-73).
// fileName originates from the database (fo.FileName) and may legitimately contain
// subdirectories, so we confine the cleaned path to baseDir rather than stripping it.
func resolveWorkspacePath(baseDir, fileName string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("while resolving workspace base dir: %v", err)
	}
	joined := filepath.Join(absBase, fileName)
	if joined != absBase && !strings.HasPrefix(joined, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid workspace file path %q: escapes workspace directory", fileName)
	}
	return joined, nil
}

// ensureLocalRepoSeeded makes an image-carried workspace usable at the path the
// rest of the code actually reads.
//
// Every workspace path in this package and in compute_pipes is built from
// WORKSPACES_HOME, and both packages read that variable in init(), so it cannot
// be repointed at run time. An image that bakes its compiled workspace in puts
// it at WORKSPACES_REPO instead, and something has to bridge the two:
//
//   - a container process gets the bridge from cbooter, which copies
//     WORKSPACES_REPO to WORKSPACES_HOME before exec'ing the binary
//     (jets/cmds/cbooter/main.go);
//   - a Lambda has no cbooter. Its entrypoint is the runtime interface and its
//     CMD is the handler, so nothing copied anything: the baked workspace sat
//     unread at /workspaces while WORKSPACES_HOME=/tmp/workspaces stayed empty
//     and the fetch the bake was meant to save ran on every cold start.
//
// This is that bridge, and it is on the *skip* path rather than at startup on
// purpose. It runs only once the version check has decided the image's own
// workspace is the one to use, so a run that is about to download a newer one
// does not pay for a copy it would immediately overwrite.
//
// No-op wherever WORKSPACES_REPO is unset (the zip lambdas carry no workspace),
// where it equals WORKSPACES_HOME, or where the destination already holds the
// workspace - which is every container, cbooter having got there first. Lambda's
// /tmp is writable and lives as long as the execution environment, so the copy
// happens once per cold start and warm invocations find it already there.
//
// It copies the whole directory, as cbooter does, rather than the subset the
// runtime is known to read. The workspace is a client artifact and what reads
// what is not this function's business to predict; jets_ws measures 110 MB, of
// which .git is a few, against an S3 fetch and untar of the compiled halves.
// skipTopLevel names top-level entries this caller does not need. **The list is
// the caller's because the contract is the caller's**: `SyncWorkspaceFiles`
// already says what each runtime fetches, by `contentType`, and the seed exists
// to stand in for that fetch. See ensureLocalRepoSeeded's own note.
func ensureLocalRepoSeeded(skipTopLevel ...string) error {
	repo := os.Getenv("WORKSPACES_REPO")
	if repo == "" || workspaceHome == "" || wprefix == "" || repo == workspaceHome {
		return nil
	}
	dest := filepath.Join(workspaceHome, wprefix)
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	src := filepath.Join(repo, wprefix)
	if _, err := os.Stat(src); err != nil {
		// The caller has just decided the image's workspace is authoritative and
		// there isn't one. Say so rather than falling through to a fetch that the
		// caller has already declined to make.
		return fmt.Errorf(
			"workspace version %s matches this image's, but the image carries no workspace at %s: %v",
			workspaceVersion, src, err)
	}
	skip := make(map[string]bool, len(skipTopLevel))
	for _, name := range skipTopLevel {
		skip[name] = true
	}
	log.Printf("Seeding %s from the image's %s ...", dest, src)
	start := time.Now()
	copied, skipped, err := copyWorkspaceExcept(dest, src, skip)
	if err != nil {
		return fmt.Errorf("while seeding %s from %s: %v", dest, src, err)
	}
	log.Printf("Seeded %s in %s (%.1f MB copied, %.1f MB skipped: %v)",
		dest, time.Since(start).Round(time.Millisecond),
		float64(copied)/1e6, float64(skipped)/1e6, skipTopLevel)
	return nil
}

// copyWorkspaceExcept copies src to dest, leaving out the named top-level
// entries, and reports the bytes on each side of that decision.
//
// **It replaces `os.CopyFS`, which cannot leave anything out.** Copying
// everything measured 113.5 MB over 222 files and 24.4s on the native lambda's
// cold start.
//
// **The seed was delivering a superset of the fetch it replaces, and that is the
// argument for trimming it rather than a guess about what is read.**
// `SyncWorkspaceFiles` already declares what each runtime needs, by
// `contentType`: `SyncComputePipesWorkspace` fetches `workspace.tgz` and
// `sqlite`, `SyncRunReportsWorkspace` fetches `reports.tgz`. And
// `workspace.tgz` holds `workspace_control.json`, `build/**` and
// `pipes_config/**` — it does not hold `.git`, and it does not hold `lookups/`,
// whose CSVs are compiled into the `lookup.db` that `sqlite` brings instead. So
// a compute-pipes run that took the fetch never had those 53 MB, and a seeded
// one having them is the anomaly.
//
// **The exclusions are the caller's, not this function's**, for the same reason
// the content types are — and `run_reports` needs `lookups/` for a stronger
// reason than this comment first gave. It does sync them to S3
// (`run_reports/delegate/run_reports.go:287`), but it also **recompiles the
// workspace** immediately after, unless `SkipCompileWorkspace` is set (`:293`),
// and the compile is what actually consumes the CSVs:
// `PackageLookupTablesToSqlite` opens each one (`lookup_tables.go:87`) to
// rebuild `lookup.db`. So a report that updates a lookup table needs the source
// present, not merely copied onward. A list baked in here would be right for one
// caller and wrong for the other.
//
// **And that is why the compute-pipes exclusion is safe rather than merely
// untested.** The three callers of `CompileWorkspace` are the apiserver
// (`datatable/workspace_helper_functions.go:92`, `apiserver/server.go`),
// `run_reports` (`:295`) and the build-time CLI (`cmds/compile_workspace`).
// **`jets/compute_pipes` does not contain one**, so nothing on that path can
// ever need the CSVs to rebuild anything.
//
// **The default is to copy.** A new top-level entry is included unless somebody
// names it, which is the safe direction: an unnecessary copy costs seconds and a
// missing one breaks a pipeline. And the byte counts are logged rather than
// assumed, so if a skip stops paying, the log says so.
//
// Modes follow `os.CopyFS`'s rule — directories get WorkspaceDirMode, files get
// WorkspaceFileMode plus whatever execute bits the source carried, which is what
// keeps the workspace's 16 executable files executable.
func copyWorkspaceExcept(dest, src string, skip map[string]bool) (copied, skipped int64, err error) {
	fsys := os.DirFS(src)
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return os.MkdirAll(dest, utils.WorkspaceDirMode)
		}
		// Top-level only: a nested `lookups/` under some other directory is not
		// what the caller named, and silently matching it would be a surprise.
		if !strings.Contains(path, "/") && skip[path] {
			if size, sErr := treeSize(fsys, path); sErr == nil {
				skipped += size
			}
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dest, path)
		if d.IsDir() {
			return os.MkdirAll(target, utils.WorkspaceDirMode)
		}
		info, iErr := d.Info()
		if iErr != nil {
			return iErr
		}
		if !info.Mode().IsRegular() {
			// The workspace holds none today; refuse rather than copy something
			// whose meaning in the destination is not the same as here.
			return fmt.Errorf("cannot seed %s: not a regular file (%v)", path, info.Mode())
		}
		in, oErr := fsys.Open(path)
		if oErr != nil {
			return oErr
		}
		defer in.Close()
		perm := utils.WorkspaceFileMode | (info.Mode().Perm() & 0o111)
		out, cErr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if cErr != nil {
			return cErr
		}
		n, copyErr := io.Copy(out, in)
		copied += n
		if closeErr := out.Close(); closeErr != nil && copyErr == nil {
			copyErr = closeErr
		}
		return copyErr
	})
	return copied, skipped, err
}

// treeSize totals the regular files under one entry, for the skipped-bytes log.
func treeSize(fsys fs.FS, root string) (int64, error) {
	var total int64
	err := fs.WalkDir(fsys, root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if info, iErr := d.Info(); iErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// Sync the workspace files for run report lambdas if a new version of the workspace exist since the last call.
// Return true if a sync was performed.
// Note: run report lambdas do not need the full workspace, only the reports definitions.
// hence only sync the reports.tgz file.
// Note: in dev mode, do not sync from database.
// Note: sync is only performed if more than 1 min since last check to avoid too many db calls.
// Note: No sync if db version is same as local repo version (meaning workspace taken from local repo).
func SyncRunReportsWorkspace(dbpool *pgxpool.Pool) (bool, error) {
	if devMode {
		return false, nil
	}
	// See if it worth to do a check
	if lastWorkspaceSyncCheck != nil && time.Since(*lastWorkspaceSyncCheck) < time.Duration(1)*time.Minute {
		// No need to check since it was check less than a min ago
		return false, nil
	}
	now := time.Now()
	lastWorkspaceSyncCheck = &now

	// Get the latest workspace version
	// Check the workspace release in database vs current release
	version, err := GetWorkspaceVersion(dbpool)
	if err != nil {
		return false, err
	}
	didSync := false
	if version != workspaceVersion {
		workspaceVersion = version
		if len(localRepoVersion) > 0 && localRepoVersion == workspaceVersion {
			// No need to sync since workspace taken from local repo
			log.Printf("Skipping sync of reports.tgz since workspace version %s is same as local repo version", workspaceVersion)
			return false, ensureLocalRepoSeeded()
		}
		// Get the reports
		_, err = SyncWorkspaceFiles(dbpool, wprefix, "reports.tgz", true, false)
		if err != nil {
			return false, fmt.Errorf("error while synching reports.tgz file from db: %v", err)
		}
		didSync = true
	} else {
		log.Printf("🙌 No need to sync run reports workspace, version %s is same as last synced version 🙌", workspaceVersion)
	}
	return didSync, nil
}

// Sync the workspace files for cpipes lambdas if a new version of the workspace exist since the last call.
// Return true if a sync was performed.
// Synch both workspace.tgz and sqlite files.
// Note: in dev mode, do not sync from database.
// Note: sync is only performed if more than 1 min since last check to avoid too many db calls.
// Note: No sync if db version is same as local repo version (meaning workspace taken from local repo).
func SyncComputePipesWorkspace(dbpool *pgxpool.Pool) (bool, error) {
	if devMode {
		return false, nil
	}
	// See if it worth to do a check
	if lastWorkspaceSyncCheck != nil && time.Since(*lastWorkspaceSyncCheck) < time.Duration(1)*time.Minute {
		// No need to check since it was check less than a min ago
		return false, nil
	}
	now := time.Now()
	lastWorkspaceSyncCheck = &now

	// Get the latest workspace version
	// Check the workspace release in database vs current release
	version, err := GetWorkspaceVersion(dbpool)
	if err != nil {
		return false, err
	}
	didSync := false
	if version != workspaceVersion {
		workspaceVersion = version
		if len(localRepoVersion) > 0 && localRepoVersion == workspaceVersion {
			// No need to sync since workspace taken from local repo
			log.Printf("Skipping sync of workspace.tgz and sqlite since workspace version %s is same as local repo version", workspaceVersion)
			// The two entries the fetch this stands in for would not have
			// delivered: `.git` is version-control metadata, and `lookups/` is
			// the CSV source compiled into the `lookup.db` that `sqlite` brings.
			// 53 MB of 113 MB, measured. Safe here because nothing in
			// `jets/compute_pipes` calls `CompileWorkspace` — the compile is the
			// only thing that reads the CSVs, and it lives on the apiserver and
			// `run_reports` paths, which pass no exclusions.
			return false, ensureLocalRepoSeeded(".git", "lookups")
		}
		// Get the compiled rules
		_, err = SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), "workspace.tgz", true, false)
		if err != nil {
			return false, fmt.Errorf("error while synching workspace file from db: %v", err)
		}
		// Get the compiled lookups
		_, err = SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), "sqlite", false, true)
		if err != nil {
			return false, fmt.Errorf("error while synching workspace file from db: %v", err)
		}
		didSync = true
	} else {
		log.Printf("🙌 No need to sync compute pipes workspace, version %s is same as last synced version 🙌", workspaceVersion)
	}
	return didSync, nil
}

func GetWorkspaceVersion(dbpool *pgxpool.Pool) (string, error) {
	var version string
	stmt := "SELECT MAX(version) FROM jetsapi.workspace_version"
	err := dbpool.QueryRow(context.Background(), stmt).Scan(&version)
	if err != nil {
		return "", fmt.Errorf("while checking latest workspace version: %v", err)
	}
	return version, nil
}

// UpdateWorkspaceVersionDb records the compiled workspace version.
//
// **It received workspaceName and discarded it until 2026-09-04**, which is why a
// run bound to a version could not be resolved to a source: the version answered
// *which compiled artefact* and never *which workspace*, and the table's unique
// constraint was on the version string alone across every workspace at once. The
// row now names the workspace and the commit its source tree was at.
//
// **The commit is best-effort and null is a real answer.** A workspace may be
// synced without a .git directory, and a compile must not fail because the record
// cannot say what it compiled -- the same reasoning setWorkspaceBinding applies to
// a run that starts before any workspace has been compiled
// (`setWorkspaceBinding`, `jets/datatable/workspace_binding.go:41`).
func UpdateWorkspaceVersionDb(dbpool *pgxpool.Pool, workspaceName, version string) error {

	if version == "" {
		log.Println("Error: attempting to write empty version to table workspace_version, skipping")
		return nil
	}
	// The workspace name is what makes the version resolvable; write null rather
	// than "" when it is absent, so a reader can tell "not recorded" from a name.
	var name any
	if workspaceName != "" {
		name = workspaceName
	}
	var commit any
	if workspaceName != "" {
		if sha, err := git.HeadCommit(workspaceHome, workspaceName); err == nil && isWorkspaceCommitSha(sha) {
			commit = sha
		} else {
			// **The tree could not answer, so the image that carries the tree is
			// asked instead, and this is the one part of the no-git work that is
			// about the agentic layer rather than about a screen.** `rca.mapping`
			// classifies `CauseTransformationDefect` as `Coarse` rather than `None`
			// partly on the strength of a run resolving to a workspace commit
			// (`CauseTransformationDefect`, `jets/agentic/rca/mapping.go:128`). In a
			// deployment with no repository this column would otherwise be null on
			// each row this function writes -- not stale, not unreliable, absent --
			// and a triage classification would be weakened by a deployment choice
			// that has nothing to do with triage. The build already knows the sha
			// (`ARG WORKSPACE_GIT_SHA`, `dockerfiles/Dockerfile.ui_service_ws:35`,
			// promoted to an ENV at `:36`); until this, nothing in Go read it.
			//
			// **What the fallback is worth, said plainly, because the column will
			// be read as though it still means what it used to mean.** A sha
			// recorded from an image build argument is as trustworthy as the build
			// script that set it, and until this change the identifier appeared in
			// that one Dockerfile and nowhere else in the tree (grepped
			// 2026-09-07) -- its only other consumer is the /VERSION.txt line at
			// `:40`, and no script, Makefile or CDK file here passes a value for
			// the ARG, so a site that wants this column populated has to arrange
			// that in the image build, which lives outside this repository. It also
			// names the commit the *image* was built from, which is the commit the
			// workspace tree is at only while the tree arrives inside the image --
			// true of the recursive copy cbooter makes
			// (`jets/cmds/cbooter/main.go:129`), and not a property this function
			// can check. So it records a sha or a null and claims nothing further
			// about provenance.
			//
			// **It is deliberately not scoped to the git-off case.** A tree synced
			// without a .git directory in a deployment that has git is the same
			// question with the same answer, and `HeadCommit`'s own comment already
			// names that shape as the reason it is best-effort.
			buildSha, ignored := workspaceCommitFromBuildEnv()
			switch {
			case buildSha != "":
				commit = buildSha
				log.Printf("Notice: workspace %s HEAD commit is not available, recording version %s with the commit carried by %s instead",
					workspaceName, version, workspaceGitShaEnvVar)
			case ignored != "":
				// A build argument holding prose, a short sha or a tag is a
				// misconfiguration rather than an absence, and it is invisible from
				// the column it failed to fill -- both cases write a null. Saying
				// which one happened is the difference between "this deployment
				// records no commit" and "this deployment meant to and its build
				// script is wrong".
				log.Printf("Notice: %s is set to %q, which is not a 40-character lowercase hex sha, recording version %s for workspace %s without a commit",
					workspaceGitShaEnvVar, ignored, version, workspaceName)
			case git.NoGitAccess():
				// The expected case, reported as a configuration rather than as a
				// fault: git is off and the image carries no sha, so there is no
				// commit to record and nothing has gone wrong. `HeadCommit`'s error
				// is deliberately not echoed here -- it is an accurate description
				// of a deployment doing what it was told, and an error line printed
				// on every compile for the one thing the operator configured is the
				// alarm that HeadCommit's short-circuit removed at source, arriving
				// one level up.
				log.Printf("Notice: git is off for this deployment and %s carries no sha, recording version %s for workspace %s without a commit",
					workspaceGitShaEnvVar, version, workspaceName)
			default:
				log.Printf("Notice: workspace %s HEAD commit is not available, recording version %s without one: %v",
					workspaceName, version, err)
			}
		}
	}
	// insert the new workspace version in jetsapi db
	log.Println("Updating workspace version in database to", version, "for workspace", workspaceName)
	stmt := `INSERT INTO jetsapi.workspace_version (version, workspace_name, workspace_commit)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := dbpool.Exec(context.Background(), stmt, version, name, commit)
	if err != nil {
		return fmt.Errorf("while inserting workspace version into workspace_version table: %v", err)
	}

	return nil
}

// workspaceGitShaEnvVar names the image build argument that carries the commit the
// workspace tree was built from.
//
// **It has existed and gone unread.** `dockerfiles/Dockerfile.ui_service_ws`
// declares it as an ARG and promotes it to an ENV (`:35`, `:36`) so that
// /VERSION.txt can print it (`:40`); reading it here uses something the build was
// already told rather than asking the build for anything new.
//
// **It reaches the apiserver's image and not the others.** `Dockerfile.compile_ws`
// and `Dockerfile.cpipes_ws` declare no such argument (checked 2026-09-07), so a
// compile driven from one of those images finds the variable absent and writes a
// null exactly as it did before. That is a limit of the fallback rather than a
// defect in it: ui_service_ws is the image the UI compiles a workspace from, and it
// is the image a deployment with no source-control host runs.
const workspaceGitShaEnvVar = "WORKSPACE_GIT_SHA"

// isWorkspaceCommitSha reports whether s is what this package will accept into
// workspace_version.workspace_commit: forty lowercase hex digits, the full object
// name git itself prints.
//
// **It restates the git package's `shaPattern` rather than sharing it**
// (`shaPattern`, `jets/datatable/git/workspace_git.go`), which is unexported.
// Exporting it to share one regular expression would widen that package's surface
// for the sake of four characters; the two do not have to be the same object, they
// have to agree about what the column holds. Written as a loop rather than as a
// second `^[0-9a-f]{40}$` because the rule is short enough to state exactly, and
// one comparison per compile does not need a pattern compiled at init.
//
// **Uppercase is refused rather than folded to lower**, so the column has one
// spelling of a commit and a reader comparing two rows can compare strings. A value
// that is nearly a sha -- a short one, a tag, a branch name -- is prose as far as
// this column is concerned, which is the whole point of validating an environment
// variable that no build script in this repository sets (grepped 2026-09-07).
func isWorkspaceCommitSha(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// workspaceCommitFromBuildEnv reads workspaceGitShaEnvVar and returns the sha it
// carries, or an empty sha and the raw value when it carries something else.
//
// **The second return exists so a misconfigured build can be said out loud.** An
// absent variable and a variable holding "unknown" both end in a null commit, and
// from the column they are indistinguishable; the caller logs them differently
// because only one of them is somebody's mistake.
//
// **The value is trimmed before it is judged.** It travels from a shell through a
// Docker build argument into an environment variable, and a trailing newline
// picked up on that route is a transport artefact rather than a different sha.
// Trimming is not leniency about the format: what is left is matched whole.
func workspaceCommitFromBuildEnv() (sha string, ignored string) {
	raw := strings.TrimSpace(os.Getenv(workspaceGitShaEnvVar))
	switch {
	case raw == "":
		return "", ""
	case !isWorkspaceCommitSha(raw):
		return "", raw
	}
	return raw, ""
}
