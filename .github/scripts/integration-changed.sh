#!/usr/bin/env bash
# Decide whether an integration job must boot a controller. On schedule or
# manual dispatch there is no PR to diff, so always run. On a pull_request,
# run only when an integration-relevant path changed; a diff that cannot be
# computed falls back to running rather than silently skipping a required
# gate. Writes run=true|false to $GITHUB_OUTPUT for the calling step.
#
# Shared by both jobs in integration.yaml so the path set lives in one place.
set -euo pipefail

if [ "${GITHUB_EVENT_NAME:-}" != "pull_request" ]; then
  echo "run=true" >>"$GITHUB_OUTPUT"
  exit 0
fi

base="$(jq -r '.pull_request.base.sha // empty' "$GITHUB_EVENT_PATH")"
if [ -z "$base" ] || ! changed="$(git diff --name-only "${base}...HEAD")"; then
  echo "could not determine changed files; running suite to be safe"
  echo "run=true" >>"$GITHUB_OUTPUT"
  exit 0
fi

printf '%s\n' "$changed"
if printf '%s\n' "$changed" | grep -qE '^(schemas/|overrides/|unifi/|cmd/fields/|internal/controllertest/|go\.mod$|go\.sum$|\.github/workflows/|\.github/scripts/)'; then
  echo "run=true" >>"$GITHUB_OUTPUT"
else
  echo "no integration-relevant paths changed; skipping suite"
  echo "run=false" >>"$GITHUB_OUTPUT"
fi
