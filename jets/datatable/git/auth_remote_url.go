package git

import (
	"fmt"
	"net/url"
)

// authRemoteURL builds an authenticated https remote URL with the credentials
// properly percent-encoded so that special characters cannot alter the URL.
func authRemoteURL(gitUser, gitToken, gitRepo string) string {
	return fmt.Sprintf("https://%s@%s", url.UserPassword(gitUser, gitToken).String(), gitRepo)
}
