#!/usr/bin/env bash
#
# Port of scripts/writeGitVersion.mts.
#
# The TypeScript original rewrote package.json in place, turning "3.1.0" into
# "3.1.0-<short sha>" so CI could publish a throwaway prerelease tarball.
#
# Go modules have no package.json: a module's version is the git tag it is
# fetched by, and pseudo-versions (v0.0.0-<utc timestamp>-<short sha>) are
# derived by the toolchain, never written into a file. There is therefore
# nothing to mutate. This script prints the pseudo-version that `go get` would
# resolve for the given revision, so CI logs record exactly which commit a job
# built, matching what the upstream script achieved.
#
# Usage: scripts/version.sh [<git rev>]   (defaults to HEAD)

set -euo pipefail

rev="${1:-HEAD}"

short_sha="$(git rev-parse --short=12 "${rev}")"
commit_time="$(TZ=UTC0 git show --quiet --date='format-local:%Y%m%d%H%M%S' --format=%cd "${rev}")"

base_version="$(git describe --tags --abbrev=0 "${rev}" 2>/dev/null || echo v0.0.0)"

printf '%s-%s-%s\n' "${base_version}" "${commit_time}" "${short_sha}"
