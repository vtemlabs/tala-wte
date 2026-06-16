#!/usr/bin/env bash
#
# bump-version.sh - Tala WTE release tagger.
#
# Tala WTE is a single Go binary whose version is stamped at build time from the
# git tag (see .github/workflows/release.yml, which injects it with -ldflags into
# internal/version.Version). There is no version file to edit: the tag IS the
# version. Pushing a vX.Y.Z tag is what triggers the release workflow to build the
# linux amd64 + arm64 binaries and publish them as a GitHub Release.
#
# This script is the safe way to create that tag. It validates the working tree,
# computes the next version off the latest tag, writes an annotated tag with a
# short changelog, and pushes it.
#
# Usage:
#   ./scripts/bump-version.sh [patch|minor|major|X.Y.Z] [beta] [flags]
#
#   patch|minor|major   bump the latest release tag by that level (default patch)
#   X.Y.Z               set an explicit version (required for the FIRST release,
#                       before any tag exists)
#   beta                cut/iterate a prerelease on the BETA channel:
#                         X.Y.Z-beta.1 -> X.Y.Z-beta.2 -> ...
#                       Run the same bump WITHOUT 'beta' to promote to stable.
#
#   --no-push | --local commit + tag locally only (do NOT push, no release)
#   --dry-run | -n      show what would happen, change nothing
#   --yes | -y          skip the confirmation prompt
#   -h | --help         show this help
#
# Push happens by default: you confirm once, then it tags and pushes (which fires
# the release workflow). The working tree must be clean so the tag reflects real
# state.

set -euo pipefail

# Resolve repo root from git (not $PWD) so the script works from any directory.
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "error: not inside a git repository." >&2
  exit 1
}
cd "$REPO_ROOT"

C_RED=$'\033[0;31m'; C_GREEN=$'\033[0;32m'; C_YELLOW=$'\033[1;33m'; C_BLUE=$'\033[0;34m'; C_OFF=$'\033[0m'
info() { echo -e "${C_BLUE}[bump]${C_OFF} $*"; }
ok()   { echo -e "${C_GREEN}[bump]${C_OFF} $*"; }
warn() { echo -e "${C_YELLOW}[bump]${C_OFF} $*" >&2; }
die()  { echo -e "${C_RED}[bump]${C_OFF} $*" >&2; exit 1; }
usage() { sed -n '3,40p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

readonly TAG_PREFIX="v"   # release line: vX.Y.Z

# -----------------------------------------------------------------------------
# Argument parsing
# -----------------------------------------------------------------------------
LEVEL="patch"; EXPLICIT=""; IS_BETA=0; DO_PUSH=1; DRY_RUN=0; ASSUME_YES=0
for arg in "$@"; do
  case "$arg" in
    patch|minor|major)        LEVEL="$arg" ;;
    beta)                     IS_BETA=1 ;;
    v[0-9]*.[0-9]*.[0-9]*)    EXPLICIT="${arg#v}" ;;
    [0-9]*.[0-9]*.[0-9]*)     EXPLICIT="$arg" ;;
    --push)                   DO_PUSH=1 ;;
    --no-push|--local)        DO_PUSH=0 ;;
    --dry-run|-n)             DRY_RUN=1 ;;
    --yes|-y)                 ASSUME_YES=1 ;;
    -h|--help)                usage 0 ;;
    *) echo "unknown argument: $arg" >&2; usage 1 ;;
  esac
done

semver_ok() { [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; }

# -----------------------------------------------------------------------------
# Preflight
# -----------------------------------------------------------------------------
git fetch --tags --quiet origin 2>/dev/null \
  || warn "could not fetch tags from origin - computed version may be stale."

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$CURRENT_BRANCH" != "main" && "$CURRENT_BRANCH" != "master" ]]; then
  warn "on branch '$CURRENT_BRANCH' (not main/master)."
  if [ "$ASSUME_YES" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    read -r -p "tag anyway? [y/N] " reply
    [[ "$reply" =~ ^[Yy]$ ]] || die "aborted."
  fi
fi

# -----------------------------------------------------------------------------
# Compute the target version
# -----------------------------------------------------------------------------
# Latest STABLE version (plain X.Y.Z, no -beta). Betas never advance the stable
# base, so an open beta line bumps cleanly to the next patch.
# `|| true`: with no tags yet (first release) grep matches nothing and exits 1,
# which set -o pipefail would otherwise treat as fatal.
LATEST_STABLE="$(git tag --list "${TAG_PREFIX}[0-9]*.[0-9]*.[0-9]*" \
  | sed "s/^${TAG_PREFIX}//" \
  | { grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' || true; } \
  | sort -V | tail -n 1)"
# Latest tag of ANY kind (incl. betas) - used for the changelog range.
LATEST_ANY="$(git tag --list "${TAG_PREFIX}*" | sort -V | tail -n 1)"

if [ -n "$EXPLICIT" ]; then
  semver_ok "$EXPLICIT" || die "invalid explicit version '$EXPLICIT' (expected X.Y.Z)"
  BASE="$EXPLICIT"
elif [ -n "$LATEST_STABLE" ]; then
  IFS=. read -r M N P <<<"$LATEST_STABLE"
  case "$LEVEL" in
    patch) P=$((P + 1)) ;;
    minor) N=$((N + 1)); P=0 ;;
    major) M=$((M + 1)); N=0; P=0 ;;
  esac
  BASE="$M.$N.$P"
else
  die "no ${TAG_PREFIX}X.Y.Z tag exists yet - pass an explicit version for the first release, e.g. ./scripts/bump-version.sh 0.1.0"
fi

if [ "$IS_BETA" -eq 1 ]; then
  # Next beta iteration for this base: max existing -beta.N + 1.
  MAX_BETA=0
  while read -r t; do
    [ -z "$t" ] && continue
    n="${t##*-beta.}"
    if [[ "$n" =~ ^[0-9]+$ ]] && (( n > MAX_BETA )); then MAX_BETA=$n; fi
  done < <(git tag --list "${TAG_PREFIX}${BASE}-beta.*")
  NEW_VER="${BASE}-beta.$((MAX_BETA + 1))"
else
  NEW_VER="$BASE"
fi

TAG="${TAG_PREFIX}${NEW_VER}"

# Guards: never reuse a tag, never go backwards on the stable line.
[ -z "$(git tag -l "$TAG")" ] || die "tag $TAG already exists - pick another version or delete the old tag."
if [ "$IS_BETA" -eq 0 ] && [ -n "$LATEST_STABLE" ] \
   && [ "$(printf '%s\n%s\n' "$LATEST_STABLE" "$BASE" | sort -V | tail -n 1)" != "$BASE" ]; then
  die "target $BASE is older than latest stable $LATEST_STABLE (refusing to downgrade)."
fi

# -----------------------------------------------------------------------------
# Summary + confirmation
# -----------------------------------------------------------------------------
HEAD_SHA="$(git rev-parse --short HEAD)"
HEAD_SUBJECT="$(git log -1 --pretty=%s)"
CURRENT_DISPLAY="<none>"
[ -n "$LATEST_STABLE" ] && CURRENT_DISPLAY="${TAG_PREFIX}${LATEST_STABLE}"

echo ""
info "repo        : vtemlabs/tala-wte"
info "current     : ${CURRENT_DISPLAY}"
info "new tag     : ${C_GREEN}${TAG}${C_OFF}"
info "target      : ${HEAD_SHA}  ${HEAD_SUBJECT}"
info "channel     : $([ "$IS_BETA" -eq 1 ] && echo BETA || echo stable)"
if [ -n "$LATEST_ANY" ]; then
  echo ""
  info "commits since ${LATEST_ANY}:"
  git log --oneline "${LATEST_ANY}..HEAD" | sed 's/^/  /'
fi
echo ""
if [ "$DO_PUSH" -eq 1 ]; then
  info "action      : tag ${TAG} AND push to origin (triggers release build)"
else
  info "action      : tag ${TAG} locally (push manually to release)"
fi
echo ""

if [ "$DRY_RUN" -eq 1 ]; then
  info "dry run - nothing changed."
  exit 0
fi

# Clean tree required: a tag on a dirty tree embeds uncommitted state.
git diff --quiet && git diff --cached --quiet \
  || { git status --short >&2; die "working tree not clean - commit or stash first."; }

if [ "$ASSUME_YES" -eq 0 ]; then
  read -r -p "$(echo -e "${C_YELLOW}proceed? [y/N] ${C_OFF}")" reply
  [[ "$reply" =~ ^[Yy]$ ]] || die "aborted."
fi

# -----------------------------------------------------------------------------
# Tag + push
# -----------------------------------------------------------------------------
# Annotated tag with a short changelog so the message shows on the release page.
if [ -n "$LATEST_ANY" ]; then
  CHANGELOG="$(git log "${LATEST_ANY}..HEAD" --pretty=format:"- %s" | head -10)"
  git tag -a "$TAG" -m "Release ${TAG}

Changes since ${LATEST_ANY}:
${CHANGELOG}"
else
  git tag -a "$TAG" -m "Release ${TAG}"
fi
ok "created tag ${TAG} at ${HEAD_SHA}"

if [ "$DO_PUSH" -eq 0 ]; then
  info "not pushed. to publish + trigger the release build:"
  echo "  git push origin ${TAG}"
  exit 0
fi

git push origin "$TAG"
ok "pushed ${TAG} to origin - release workflow should start shortly."
info "watch: https://github.com/vtemlabs/tala-wte/actions"
