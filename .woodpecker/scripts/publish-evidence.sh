#!/usr/bin/env bash
set -euo pipefail

# Put a campaign's evidence somewhere a human can reach it, including when the
# candidate was REJECTED -- which is the case this loop deliberately creates and
# the case where the evidence matters most. Release attachments cannot serve a
# rejected candidate, because there is no release. The generic package registry
# can, and it is the pattern the firmware cache already uses.
#
# Two properties this has to hold, both learned rather than assumed:
#
#   byte-identical. The attempt ledger is hash-chained and the attestation is
#   bound to controller identity, so an upload that transforms anything
#   silently destroys both. Every file is read back and compared by sha256.
#
#   discoverable. A package is harder to stumble across than an attachment, so
#   the location is printed. One line closes the gap.
#
# Usage: publish-evidence.sh <evidence-dir> <package-version>

readonly evidence_dir=${1:?evidence directory is required}
readonly package_version=${2:?package version is required}
: "${FORGEJO_BASE_URL:?FORGEJO_BASE_URL is required}"
: "${FORGEJO_USER:?FORGEJO_USER is required}"
: "${FORGEJO_TOKEN:?FORGEJO_TOKEN is required}"
readonly owner=${EVIDENCE_PACKAGE_OWNER:-infra}
readonly package=${EVIDENCE_PACKAGE_NAME:-go-unifi-campaign-evidence}
readonly base="${FORGEJO_BASE_URL}/api/packages/${owner}/generic/${package}/${package_version}"

if [[ ! -d ${evidence_dir} ]]; then
    echo "evidence directory not readable: ${evidence_dir}" >&2
    exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

published=0
for path in "${evidence_dir}"/*; do
    [[ -f ${path} ]] || continue
    name="$(basename "${path}")"
    source_hash="$(sha256sum "${path}" | cut -d' ' -f1)"

    # The body, not just the code. This discarded the response to /dev/null and
    # reported "HTTP 403" with no reason, which is the same mistake as a bare
    # status check anywhere else: the server explained itself and we threw it
    # away, leaving the next person to guess at permissions, quotas or names.
    body="${work}/.upload-response"
    status="$(curl --fail-with-body --silent --show-error --output "${body}" \
        --write-out '%{http_code}' --user "${FORGEJO_USER}:${FORGEJO_TOKEN}" \
        --upload-file "${path}" "${base}/${name}" || true)"

    # 409 means this exact version already carries the file. A re-queued
    # campaign must not be an error, and must not silently overwrite either:
    # the round-trip below still proves what is stored matches what we have.
    case "${status}" in
        201|409) ;;
        *)
            echo "upload ${name}: HTTP ${status}" >&2
            echo "  url: ${base}/${name}" >&2
            echo "  size: $(wc -c <"${path}") bytes" >&2
            if [[ -s ${body} ]]; then
                echo "  the registry said:" >&2
                sed 's/^/    /' "${body}" >&2
            else
                echo "  the registry returned no body" >&2
            fi
            exit 1
            ;;
    esac

    if ! curl --fail --silent --show-error --user "${FORGEJO_USER}:${FORGEJO_TOKEN}" \
        --output "${work}/${name}" "${base}/${name}"; then
        echo "read back ${name}: failed" >&2
        exit 1
    fi
    stored_hash="$(sha256sum "${work}/${name}" | cut -d' ' -f1)"
    if [[ ${stored_hash} != "${source_hash}" ]]; then
        echo "evidence ${name} did not survive the round trip" >&2
        echo "  local:  ${source_hash}" >&2
        echo "  stored: ${stored_hash}" >&2
        echo "  the ledger is hash-chained, so a transformed upload is a destroyed chain" >&2
        exit 1
    fi
    printf '  %s  %s (http %s)\n' "${source_hash}" "${name}" "${status}"
    published=$((published + 1))
done

if [[ ${published} -eq 0 ]]; then
    echo "no evidence files found in ${evidence_dir}" >&2
    exit 1
fi

echo "evidence published, ${published} file(s), each verified byte-identical:"
echo "  ${base}"
