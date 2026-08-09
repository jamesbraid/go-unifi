#!/usr/bin/env bash
set -euo pipefail

# Public Go API comparison against the newest release tag.
#
# This lived in two places: generate.yaml and auto-release.yaml, the latter
# carrying a comment acknowledging the duplication. Porting by copying would
# have made four homes for one fact. Both callers now source the verdict here.
#
# The baseline is extracted from the local tag with git archive rather than
# fetched through the module proxy: gorelease cannot do that, and the proxy may
# not yet serve, or may have cached a 404 for, a tag pushed moments ago.
#
# cmd/ holds main packages, which are not importable API, so incompatibilities
# there are ignored.
#
# Writes three files into --output-dir:
#   breaking          "true" or "false"
#   base              the tag compared against, empty when none exists
#   summary.md        prose for a PR body or release note
# and exits 0 whether or not the API broke. Breaking is a fact to report, not a
# failure of this script -- the caller decides what it means.

readonly apidiff_module=golang.org/x/exp/cmd/apidiff@v0.0.0-20260709172345-9ea1abe57597
module=${APIDIFF_MODULE:-github.com/ubiquiti-community/go-unifi}
output_dir=${1:-}

if [[ -z ${output_dir} ]]; then
    echo "usage: apidiff-check.sh <output-dir>" >&2
    echo "  writes breaking, base and summary.md into it" >&2
    exit 2
fi
mkdir -p "${output_dir}"

base="$(git tag --list 'v*' --sort=-v:refname | head -n1)"
printf '%s' "${base}" >"${output_dir}/base"

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
breaking_file="${work}/breaking.txt"

if [[ -n ${base} ]]; then
    mkdir "${work}/base"
    git archive "${base}" | tar -x -C "${work}/base"
    (cd "${work}/base" && go run "${apidiff_module}" -m -w "${work}/base.export" "${module}")
    go run "${apidiff_module}" -m -incompatible "${work}/base.export" "${module}" >"${work}/incompatible.txt"
    grep -v '^- \./cmd/' "${work}/incompatible.txt" >"${breaking_file}" || true
else
    : >"${breaking_file}"
fi

{
    if [[ -s ${breaking_file} ]]; then
        printf '**Breaking API changes** against `%s` (apidiff):\n\n' "${base}"
        printf '```\n'
        head -n 100 "${breaking_file}"
        total="$(wc -l <"${breaking_file}")"
        if [[ ${total} -gt 100 ]]; then
            printf '... (%s incompatible changes total)\n' "${total}"
        fi
        printf '```\n\n'
        printf 'A maintainer must review. The auto-release job refuses to tag a\n'
        printf 'break, so tag by hand: an accepted minor, or a /v2 major.\n'
    else
        printf 'No breaking API changes against `%s`.\n' "${base:-any release tag}"
    fi
} >"${output_dir}/summary.md"

if [[ -s ${breaking_file} ]]; then
    printf 'true' >"${output_dir}/breaking"
    echo "apidiff: BREAKING against ${base}" >&2
    cat "${breaking_file}" >&2
else
    printf 'false' >"${output_dir}/breaking"
    echo "apidiff: no breaking changes against ${base:-<no release tags>}" >&2
fi
