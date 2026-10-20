// Package version holds build-identifying constants used across the tool,
// most importantly in the HTTP User-Agent so the sites we contact can see
// who we are and how to reach the project.
package version

// Version is the tool version. The release workflow overrides it at build time
// with -ldflags "-X .../internal/version.Version=1.2.3", so it must be a var,
// not a const (the linker can only set variables).
var Version = "0.1.0-dev"

// RepoURL is the public code repository, advertised in the User-Agent.
const RepoURL = "https://github.com/pannamatena/job-radar"

// UserAgent is the value sent in the User-Agent header on every outbound
// request, e.g. "job-radar/0.1.0-dev (+https://github.com/pannamatena/job-radar)".
func UserAgent() string {
	return "job-radar/" + Version + " (+" + RepoURL + ")"
}
