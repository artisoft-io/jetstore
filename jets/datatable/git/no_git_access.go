package git

import (
	"log"
	"os"
	"sync"

	"github.com/artisoft-io/jetstore/jets/utils"
)

// NoGitAccessEnvVar turns every git operation in this package into a logged
// no-op. Truthy is "1", "true", "yes" or "on" (utils.IsTruthy); anything else,
// **including an unset variable and including the empty string**, leaves git on
// and this package behaving exactly as it did before this switch existed.
//
// # Why a new variable rather than an existing one
//
// The operator-facing name for "this site has no path to a source-control host"
// is JETS_GIT_ACCESS, and the runtime cannot read it: it is consumed once at
// synth time by NewGitAccessSecurityGroup to add egress rules
// (cdk/jetstore_one/stack/jetstore_github.go), and is not in the task
// definition's environment. WORKSPACE_URI *is* in the environment and was the
// obvious candidate, but it means "lock the workspace uri" rather than "enable
// git" -- getWorkspaceUri falls back to the row's workspace_uri when it is unset
// (jets/datatable/workspace_data_table_action.go) -- so overloading it would
// have silently disabled git for any deployment that drives the uri from the UI.
//
// # Why the value is tested and not the presence
//
// build_ui_service.go builds the task-definition environment with
// jsii.String(os.Getenv(...)) for every entry and filters nothing, so a variable
// the operator never set arrives **present and empty**. os.LookupEnv would
// therefore report "set" in every deployment the moment this joins that map, and
// git would be off everywhere. utils.IsTruthy is the guard and the empty string
// is tested explicitly in no_git_access_test.go, because that is the value every
// existing deployment gets on the day the CDK change ships.
const NoGitAccessEnvVar = "JETS_NO_GIT_ACCESS"

var (
	noGitAccessOnce sync.Once
	noGitAccess     bool
)

// NoGitAccess reports whether git operations are turned off for this process.
//
// **Read once, not per call.** A deployment does not change its mind mid-process,
// and one read is one decision every caller shares rather than a value that could
// differ between two buttons pressed a second apart.
func NoGitAccess() bool {
	noGitAccessOnce.Do(func() {
		noGitAccess = utils.IsTruthy(os.Getenv(NoGitAccessEnvVar))
	})
	return noGitAccess
}

// NoGitAccessNotice is what a skipped operation reports, in the log and in the
// workspace_registry.last_git_log column the UI's "View Last Log" dialog shows.
//
// **It names the variable deliberately.** A reader of a last_git_log in a
// deployment they did not configure needs to know which setting produced the
// silence, and this is the only place that says so outside the startup log.
const NoGitAccessNotice = "No git operation performed: " + NoGitAccessEnvVar +
	" is set. Git integration is off for this deployment."

// LogGitAccessMode states once, at startup, which way the switch went.
//
// **This is part of the design rather than a nicety.** The whole purpose of the
// switch is to be silent afterwards, and a silent switch that was misread looks
// exactly like a silent switch that was read correctly. Saying it once, loudly,
// is the countermeasure for a signal that is otherwise indistinguishable from an
// ordinary one.
func LogGitAccessMode() {
	if NoGitAccess() {
		log.Printf("ENV %s: %q -- git integration is OFF; workspace git operations will be skipped",
			NoGitAccessEnvVar, os.Getenv(NoGitAccessEnvVar))
		return
	}
	log.Printf("ENV %s: %q -- git integration is ON", NoGitAccessEnvVar, os.Getenv(NoGitAccessEnvVar))
}

// resetNoGitAccessForTest re-arms the sync.Once so a test can exercise both
// modes. Tests only; nothing in production changes the value after startup.
func resetNoGitAccessForTest() {
	noGitAccessOnce = sync.Once{}
	noGitAccess = false
}
