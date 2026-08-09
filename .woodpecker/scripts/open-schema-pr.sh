#!/usr/bin/env bash
set -euo pipefail

# Open the schema-update pull request, once.
#
# This runs inside the campaign rather than the capture pipeline, which is what
# makes the gate fail closed: no campaign means no PR, and a rejected
# classification means no PR. A status check on an already-open PR fails OPEN
# when the gate never runs, which is what the arrangement this replaces does.
#
# Idempotency is not decoration. Runs on this pool get re-queued -- a killed
# campaign came back and failed on state its first attempt never cleaned up --
# and a retry here would otherwise open a second pull request for the same
# candidate. Two PRs carrying different evidence for one commit is worse than
# none, because a human then has to work out which one is real.
#
# Usage: open-schema-pr.sh <head-branch> <base-branch> <title> <body-file>

readonly head=${1:?head branch is required}
readonly base=${2:?base branch is required}
readonly title=${3:?title is required}
readonly body_file=${4:?body file is required}
: "${FORGEJO_BASE_URL:?FORGEJO_BASE_URL is required}"
: "${FORGEJO_USER:?FORGEJO_USER is required}"
: "${FORGEJO_TOKEN:?FORGEJO_TOKEN is required}"
readonly repo=${FORGEJO_REPO:-infra/go-unifi}
readonly api="${FORGEJO_BASE_URL}/api/v1/repos/${repo}"

forgejo() {
    curl --fail --silent --show-error \
        --user "${FORGEJO_USER}:${FORGEJO_TOKEN}" \
        --header 'Content-Type: application/json' "$@"
}

existing="$(forgejo "${api}/pulls?state=open&limit=50" |
    jq -r --arg head "${head}" --arg base "${base}" \
        'map(select(.head.ref == $head and .base.ref == $base)) | .[0].number // empty')"

body="$(jq -Rs . <"${body_file}")"

if [[ -n ${existing} ]]; then
    # Update rather than duplicate. The body carries the evidence, and on a
    # re-run the evidence is what changed.
    forgejo --request PATCH \
        --data "$(jq -n --arg t "${title}" --argjson b "${body}" '{title:$t, body:$b}')" \
        "${api}/pulls/${existing}" >/dev/null
    echo "pull request #${existing} already open for ${head} -> ${base}; updated it rather than opening a second"
    echo "${FORGEJO_BASE_URL}/${repo}/pulls/${existing}"
    exit 0
fi

number="$(forgejo --request POST \
    --data "$(jq -n --arg t "${title}" --argjson b "${body}" --arg h "${head}" --arg base "${base}" \
        '{title:$t, body:$b, head:$h, base:$base}')" \
    "${api}/pulls" | jq -r '.number')"

if [[ -z ${number} || ${number} == null ]]; then
    echo "pull request creation returned no number" >&2
    exit 1
fi
echo "opened pull request #${number} for ${head} -> ${base}"
echo "${FORGEJO_BASE_URL}/${repo}/pulls/${number}"
