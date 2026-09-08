// Two-tag release contract, existence-only variant (docs/ROADMAP.md M5,
// docs/REPOSITORY_SPLIT_AND_SPEC_SYNC.md section 4.4, canonical repo).
//
// An operator creates and pushes canonical spec tag spec-v<VERSION> (VERSION from
// internal/spec/VERSION) to polytypo/polytypo before pushing this repo's own release tag
// v<X.Y.Z> (only the latter triggers .github/workflows/release.yml). This proves that tag exists
// in the canonical repository — existence only, not commit-SHA equality: this repository was
// never part of polytypo/polytypo's git history (it is a from-spec port, not a filtered
// extraction), so its commits share no ancestry with the canonical repository's, and no commit
// SHA here could ever legitimately equal one there. Existence of the canonical tag is the
// strongest claim this repository's own history can support.
//
// Reads the GitHub REST API unauthenticated (polytypo/polytypo is public), never a local git
// operation against the canonical repo (it is not this checkout). Exits non-zero with a GitHub
// Actions ::error:: annotation on any failure. Go has no manifest-version field analogous to
// package.json/pyproject.toml (a module's version is purely its git tag), so unlike the JS and
// Python ports there is no separate "verify tag matches manifest version" check to run here —
// the pushed tag itself is definitionally correct.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const canonicalRepo = "polytypo/polytypo"

var strictSemver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

func parseStrictSpecVersion(raw string) (string, error) {
	version := strings.TrimSpace(raw)
	if version == "" {
		return "", fmt.Errorf("internal/spec/VERSION is empty (after trimming whitespace)")
	}
	if !strictSemver.MatchString(version) {
		return "", fmt.Errorf(
			"internal/spec/VERSION content %q is not a strict MAJOR.MINOR.PATCH release version "+
				"-- pre-release suffixes, build-metadata suffixes, leading zeros, and any other "+
				"form are rejected by this project's spec-tag policy", version)
	}
	return version, nil
}

func canonicalTagExists(tagName string) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/git/refs/tags/%s", canonicalRepo, tagName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GitHub API returned %d resolving tag %q in %s", resp.StatusCode, tagName, canonicalRepo)
	}
	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, err
	}
	return true, nil
}

func fail(format string, args ...any) {
	fmt.Printf("::error::"+format+"\n", args...)
	os.Exit(1)
}

func main() {
	raw, err := os.ReadFile("internal/spec/VERSION")
	if err != nil {
		fail("reading internal/spec/VERSION: %v", err)
	}

	specVersion, err := parseStrictSpecVersion(string(raw))
	if err != nil {
		fail("%v", err)
	}

	tagName := "spec-v" + specVersion
	fmt.Printf("internal/spec/VERSION:       %s\n", specVersion)
	fmt.Printf("expected canonical spec tag: %s\n", tagName)

	exists, err := canonicalTagExists(tagName)
	if err != nil {
		fail("%v", err)
	}
	if !exists {
		fail("Tag %q does not exist in %s. It must be created and pushed by an operator to %s "+
			"before this repository's release tag.", tagName, canonicalRepo, canonicalRepo)
	}

	fmt.Printf("ok: canonical spec tag %q exists in %s.\n", tagName, canonicalRepo)
}
