// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Package version holds the build-time version of the binary. The release
// workflow injects the real value from the git tag with
// -ldflags "-X github.com/vtemlabs/tala-wte/internal/version.Version=X.Y.Z".
// A locally built binary keeps the default below.
package version

// Version is the semantic version of this build, WITHOUT a leading "v"
// (e.g. "0.2.0" or "0.2.0-beta.1"). It is "dev" for untagged local builds.
var Version = "dev"

// IsDev reports whether this is an untagged local build. Update checks are
// suppressed for dev builds since there is no released version to compare to.
func IsDev() bool {
	return Version == "dev" || Version == ""
}
