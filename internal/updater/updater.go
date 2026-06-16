// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Package updater checks GitHub Releases for a newer Tala WTE build and applies
// it in place. An update downloads the architecture-matched binary published by
// the release workflow, verifies it against the release checksums.txt, atomically
// replaces the running binary, and schedules a detached systemd restart so the
// service comes back up on the new code.
//
// The repository is public, so all GitHub calls are unauthenticated; no token is
// required or used.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/vtemlabs/tala-wte/internal/version"
)

// repoSlug is the owner/name this binary checks for releases. The release
// workflow publishes assets named tala-wte-linux-<arch> plus a checksums.txt.
const repoSlug = "vtemlabs/tala-wte"

// assetName is the release asset for the architecture this binary runs on.
// It mirrors the installer's naming (see cmd/server/install.go).
func assetName() string {
	return "tala-wte-linux-" + runtime.GOARCH
}

// httpClient is shared across calls with a sane timeout. Downloads can be large,
// so the timeout is generous rather than tight.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// ghRelease is the subset of the GitHub Releases API response we consume.
type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	HTMLURL    string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Status is the result of a CheckLatest call, surfaced to the UI.
type Status struct {
	Current         string `json:"current"`          // running version (no leading "v")
	Latest          string `json:"latest"`           // latest released version, "" if unknown
	UpdateAvailable bool   `json:"update_available"` // Latest is newer than Current
	Notes           string `json:"notes"`            // release body/changelog
	ReleaseURL      string `json:"release_url"`      // GitHub release page
	IsDev           bool   `json:"is_dev"`           // running an untagged local build
	Error           string `json:"error,omitempty"`  // non-fatal check failure, if any
}

// CheckLatest reports the running version and, when reachable, the latest
// published release and whether it is newer. A network failure is returned as a
// soft error inside Status (Error set) so the UI can still show the current
// version; only a programming-level failure returns a non-nil error.
func CheckLatest(ctx context.Context) (*Status, error) {
	st := &Status{
		Current: version.Version,
		IsDev:   version.IsDev(),
	}

	rel, err := latestRelease(ctx)
	if err != nil {
		// Soft-fail: surface the check error in Status so the UI still renders the
		// current version offline, rather than failing the whole request.
		st.Error = err.Error()
		return st, nil //nolint:nilerr // intentional: error is reported via Status.Error
	}

	st.Latest = strings.TrimPrefix(rel.TagName, "v")
	st.Notes = rel.Body
	st.ReleaseURL = rel.HTMLURL

	// Dev builds have no comparable version; report the latest but never prompt
	// an in-place update over a binary we did not release.
	if !st.IsDev {
		st.UpdateAvailable = compareSemver(st.Latest, st.Current) > 0
	}
	return st, nil
}

// latestRelease fetches the newest non-draft release. We list releases rather
// than hit /releases/latest so a prerelease (beta) can still be discovered;
// the first non-draft entry is the most recent by GitHub's ordering.
func latestRelease(ctx context.Context) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", repoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	var releases []ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parsing GitHub response: %w", err)
	}
	for i := range releases {
		if !releases[i].Draft {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("no releases published yet")
}

// Apply downloads the latest release binary for this architecture, verifies it
// against the release checksums, atomically replaces the running binary, and
// schedules a detached service restart. It returns the version it installed.
func Apply(ctx context.Context) (string, error) {
	if version.IsDev() {
		return "", fmt.Errorf("in-place update is disabled for local dev builds")
	}

	rel, err := latestRelease(ctx)
	if err != nil {
		return "", err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if compareSemver(latest, version.Version) <= 0 {
		return "", fmt.Errorf("already on the latest version (%s)", version.Version)
	}

	// Locate the binary the service re-execs on restart. os.Executable resolves
	// to the file systemd's ExecStart points at, so replacing it is what takes
	// effect on the next start.
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating running binary: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolving running binary path: %w", err)
	}

	wantAsset := assetName()
	var binURL, sumURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case wantAsset:
			binURL = a.BrowserDownloadURL
		case "checksums.txt":
			sumURL = a.BrowserDownloadURL
		}
	}
	if binURL == "" {
		return "", fmt.Errorf("release %s has no %s asset for this architecture", latest, wantAsset)
	}

	// Stage the download alongside the target so the final rename stays on one
	// filesystem (rename across devices fails). A unique-ish temp name avoids
	// clobbering a partial download from a prior attempt.
	tmp := self + ".update"
	if err := download(ctx, binURL, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("downloading %s: %w", wantAsset, err)
	}

	// Verify against the published checksum when available. A missing
	// checksums.txt is treated as fatal: an unverifiable binary must not replace
	// the running service.
	if sumURL == "" {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("release %s has no checksums.txt; refusing to apply an unverified binary", latest)
	}
	want, err := fetchChecksum(ctx, sumURL, wantAsset)
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	got, err := sha256File(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if !strings.EqualFold(got, want) {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", wantAsset, want, got)
	}

	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("setting executable bit: %w", err)
	}

	// Atomic swap. The running process keeps the old inode open and is unharmed;
	// systemd execs the new file on the next start.
	if err := os.Rename(tmp, self); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("replacing binary: %w", err)
	}

	if err := scheduleRestart(); err != nil {
		// The new binary is in place; a failed restart scheduling just means the
		// operator must restart the service manually. Surface it, don't roll back.
		return latest, fmt.Errorf("update installed but auto-restart could not be scheduled (%w); restart tala-wte manually", err)
	}
	return latest, nil
}

// scheduleRestart fires `systemctl restart tala-wte.service` from a transient
// systemd timer a couple of seconds out. Running it via systemd-run (not a child
// of this process) means it survives our own shutdown, so the restart completes
// even though stopping the unit kills our cgroup. The short delay lets the HTTP
// response flush to the browser first.
func scheduleRestart() error {
	cmd := exec.Command("systemd-run",
		"--on-active=2s",
		"--timer-property=AccuracySec=100ms",
		"--unit=tala-wte-self-update",
		"--collect",
		"systemctl", "restart", "tala-wte.service",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemd-run: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// download streams url to dest.
func download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fetchChecksum downloads checksums.txt and returns the hex sha256 recorded for
// the named asset. The file follows the `sha256sum` format: "<hex>  <name>".
func fetchChecksum(ctx context.Context, url, asset string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading checksums: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// The name column may carry a leading "*" (binary mode) or "./".
		name := strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./")
		if name == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum listed for %s", asset)
}

// sha256File returns the lowercase hex sha256 of a file's contents.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// compareSemver compares two versions WITHOUT leading "v". It returns >0 if a is
// newer than b, <0 if older, 0 if equal. Numeric core fields (X.Y.Z) are
// compared as integers; a prerelease (e.g. "-beta.1") sorts BEFORE its matching
// stable release, and between prereleases the suffix is compared lexically then
// by trailing number. This is enough for the patch/minor/major/beta scheme the
// bump-version script produces.
func compareSemver(a, b string) int {
	ac, apre := splitPrerelease(a)
	bc, bpre := splitPrerelease(b)

	if c := compareCore(ac, bc); c != 0 {
		return c
	}
	// Cores equal: no-prerelease (stable) outranks a prerelease.
	switch {
	case apre == "" && bpre == "":
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	default:
		return comparePrerelease(apre, bpre)
	}
}

func splitPrerelease(v string) (core, pre string) {
	if i := strings.IndexByte(v, '-'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func compareCore(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bv, _ = strconv.Atoi(bp[i])
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// comparePrerelease orders identifiers like "beta.1" vs "beta.2". It compares
// dot-separated parts, numeric where both sides are numeric, lexical otherwise.
func comparePrerelease(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		if i >= len(ap) {
			return -1 // shorter prerelease set is lower
		}
		if i >= len(bp) {
			return 1
		}
		ai, aerr := strconv.Atoi(ap[i])
		bi, berr := strconv.Atoi(bp[i])
		if aerr == nil && berr == nil {
			if ai != bi {
				if ai > bi {
					return 1
				}
				return -1
			}
			continue
		}
		if ap[i] != bp[i] {
			if ap[i] > bp[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}
