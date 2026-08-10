#!/usr/bin/env bash
set -euo pipefail

# Tag the next minor when a schema refresh lands on main.
#
# Ported from .github/workflows/auto-release.yaml. It is the pair to the capture
# loop: moving pull request creation to the forge without moving this means
# merges silently stop producing tags, which is the worst kind of regression
# because nothing reports it.
#
# The apidiff logic is NOT restated here. It lives in apidiff-check.sh and both
# callers use it. It previously existed twice, with a comment in this workflow
# acknowledging the duplication, and a port that copied it would have made four
# homes for one fact.
#
# The tag lands on the internal forge only. Publishing to GitHub is a separate
# deliberate act, because the module is consumed from there and a tag that
# exists only internally is not installable -- the wall that held up the
# v1.103.0 repoint until the tag was published.

: "${FORGEJO_BASE_URL:?FORGEJO_BASE_URL is required}"
: "${FORGEJO_USER:?FORGEJO_USER is required}"
: "${FORGEJO_TOKEN:?FORGEJO_TOKEN is required}"
# Credentials in the URL. A bare remote fails with "could not read Username ...
# No such device or address" -- the step's netrc does not cover it and git
# cannot prompt -- which is the same wall the branch push and the apidiff tag
# fetch both hit. Same repository, same defect, third location.
readonly remote="${FORGEJO_BASE_URL/#https:\/\//https://${FORGEJO_USER}:${FORGEJO_TOKEN}@}/infra/go-unifi.git"
readonly authed="${FORGEJO_BASE_URL/#https:\/\//https://${FORGEJO_USER}:${FORGEJO_TOKEN}@}/infra/go-unifi.git"

if [[ -n "$(git tag --points-at HEAD)" ]]; then
    echo "auto-release: HEAD is already tagged as $(git tag --points-at HEAD); nothing to do"
    exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
bash "$(dirname -- "${BASH_SOURCE[0]}")/apidiff-check.sh" "${work}"

if [[ "$(cat "${work}/breaking")" == true ]]; then
    echo "auto-release: NOT TAGGING. The public Go API breaks against $(cat "${work}/base")."
    echo "  A major bump needs a /v2 module path, which is a human decision."
    echo "  Accept the break and tag the next minor by hand, or do the /v2 work."
    cat "${work}/summary.md"
    exit 0
fi

# Retry against races with a manually pushed tag: refetch, recompute, retry.
for attempt in 1 2 3; do
    git fetch --tags --quiet "${remote}" || true
    latest="$(git tag --list 'v*' --sort=-v:refname | head -n1)"
    # No default. An empty tag list used to become v1.0.0, and the next lines
    # turn that into a real pushed tag -- so a clone that simply could not see
    # the tags (the fetch above is best-effort, and Woodpecker clones with
    # --no-tags) would have published v1.1.0 while v1.103.0 already existed,
    # walking the version backwards by 102 minors with --force behind it.
    #
    # There is no safe default for "the latest release". Absent is a refusal.
    if [[ -z ${latest} ]]; then
        echo "auto-release: no v* tag found, so the next version cannot be derived" >&2
        echo "  refusing rather than assuming a baseline: the previous default of" >&2
        echo "  v1.0.0 would tag v1.1.0 over a repository already past v1.100." >&2
        exit 1
    fi
    major="$(echo "${latest#v}" | cut -d. -f1)"
    minor="$(echo "${latest#v}" | cut -d. -f2)"
    tag="v${major}.$((minor + 1)).0"
    git tag --force "${tag}"
    if git push "${authed}" "refs/tags/${tag}"; then
        echo "auto-release: tagged ${tag} on the internal forge"
        echo "  NOT published. The module is consumed from GitHub, so this tag is"
        echo "  not installable until a separate deliberate step publishes it."
        exit 0
    fi
    git tag --delete "${tag}"
    echo "auto-release: tag push rejected on attempt ${attempt}; refetching" >&2
done
echo "auto-release: unable to push a release tag after 3 attempts" >&2
exit 1
