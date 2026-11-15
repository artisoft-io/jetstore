package dblogin

// EnsureLogin is ensureLogin, for the shared-cluster tests (package
// dblogin_test), which race it on a group of their own rather than on Reader or
// IDE: those exist once any test has run, so a race on them cannot be staged.
var EnsureLogin = ensureLogin
