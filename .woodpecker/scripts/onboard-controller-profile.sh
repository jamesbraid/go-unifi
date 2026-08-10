#!/usr/bin/env bash
set -euo pipefail

# Produce a scout profile for a controller version that has never been
# onboarded.
#
# This exists because the loop had a cycle in it: the scout profile gates the
# campaign, and internal/scout/catalog.go asserts the profile's
# controller_fingerprint equals one derived from a live provisioner receipt, so
# a version could not be onboarded without first running it. That was invisible
# while 10.4.57 was the only version anyone had, and it recurs for every version
# after this one. So this is a repeatable step rather than a script to run once
# and delete.
#
# Two kinds of value, kept distinct in the output because they carry different
# weight:
#
#   RESOLVED  looked up from the registry. Wrong only if the registry is wrong.
#   MEASURED  produced by starting the controller and asking it. The fingerprint
#             comes from cmd/provisioner-receipt, never from a formula here --
#             recomputing a canonical digest in bash would be a second
#             implementation of an identity that scout hard-errors on.
#
# Usage: onboard-controller-profile.sh <version> [repository]
#   env: UNIFI_API, UNIFI_USERNAME, UNIFI_PASSWORD for the started controller

readonly version=${1:?controller version is required, e.g. 10.5.67}
readonly repository=${2:-ghcr.io/jamesbraid/unifi-network}
readonly architecture=${ONBOARD_ARCHITECTURE:-amd64}
readonly profile_name="network-${version}-seeded"
readonly out=${ONBOARD_OUTPUT:-scout/profiles/${profile_name}.json}
readonly tag=${ONBOARD_IMAGE_TAG:-${version}}

for tool in docker jq go; do
    command -v "${tool}" >/dev/null || { echo "onboard: ${tool} is required" >&2; exit 1; }
done

echo "onboarding ${profile_name}"

# ---- RESOLVED ------------------------------------------------------------
# buildx imagetools, not `docker manifest inspect`. Validated against the one
# profile we already trust: for 10.4.57 this reproduces both committed digests
# exactly, while `manifest inspect --verbose` returns a per-platform descriptor
# that looks like an index digest and is not one.
index=$(docker buildx imagetools inspect "${repository}:${tag}" --format '{{.Manifest.Digest}}' 2>/dev/null || true)
# Checked explicitly rather than with ${VAR:?message}. Measured: when the shell
# dies from a :? expansion error, an EXIT trap observes $? as 0 and the script
# exits 0 -- so every one of these checks reported success while doing nothing.
# The trap form does not matter; any EXIT trap does it. Explicit exit 1 works.
if [[ -z ${index} ]]; then
    echo "onboard: could not resolve the image index digest for ${repository}:${tag}" >&2
    echo "  the profile pins a digest, so a tag alone is not enough" >&2
    exit 1
fi
manifest=$(docker manifest inspect "${repository}@${index}" 2>/dev/null |
    jq -r --arg arch "${architecture}" \
        '.manifests[]? | select(.platform.architecture == $arch and .platform.os == "linux") | .digest' | head -n1)
if [[ -z ${manifest} ]]; then
    echo "onboard: ${repository}@${index} advertises no linux/${architecture} manifest" >&2
    exit 1
fi
readonly index manifest
echo "  RESOLVED repository         ${repository}"
echo "  RESOLVED image_index        ${index}"
echo "  RESOLVED image_manifest     ${manifest}  (${architecture})"

# ---- MEASURED ------------------------------------------------------------
work=$(mktemp -d)
container=""
cleanup() {
    local status=$?
    [[ -n ${container} ]] && docker rm -f "${container}" >/dev/null 2>&1
    rm -rf "${work}"
    exit "${status}"
}
trap cleanup EXIT

if [[ -z ${UNIFI_USERNAME:-} ]]; then
    echo "UNIFI_USERNAME is required to measure the controller" >&2
    exit 1
fi
if [[ -z ${UNIFI_PASSWORD:-} ]]; then
    echo "UNIFI_PASSWORD is required to measure the controller" >&2
    exit 1
fi
# Two topologies. In Woodpecker the controller joins the step's network
# namespace, which is the arrangement run-live.sh proved works on this pool.
# Locally there is no step container to join, so publish the port instead.
# The architecture is NOT negotiable: the profile is consumed on amd64
# builders, and cmd/provisioner-receipt refuses a container whose architecture
# disagrees with the locked one -- which is how an arm64 workstation finds out
# it cannot author an amd64 profile.
docker_args=(--detach --rm --platform "linux/${architecture}")
if [[ -n ${CI:-} && -n ${HOSTNAME:-} ]]; then
    docker_args+=(--network "container:${HOSTNAME}")
else
    docker_args+=(--publish 127.0.0.1:8443:8443)
fi
container=$(docker run "${docker_args[@]}" "${repository}@${index}")
echo "  started ${container:0:12} from the pinned index digest (linux/${architecture})"

export UNIFI_API="${UNIFI_API:-https://127.0.0.1:8443}"
export UNIFI_INSECURE="${UNIFI_INSECURE:-true}"
go run ./cmd/provisioner-receipt \
    --container-id "${container}" \
    --image "${repository}@${index}" \
    --profile-name "${profile_name}" \
    --architecture "${architecture}" \
    --expected-version "${version}" \
    --expected-image-index "${index}" \
    --expected-image-manifest "${manifest}" \
    --output "${work}/receipt.json" \
    --fingerprint-output "${work}/fingerprint"

fingerprint=$(cat "${work}/fingerprint")
readonly fingerprint
echo "  MEASURED controller_version $(jq -r '.version' "${work}/receipt.json")"
echo "  MEASURED instance_identity  $(jq -r '.instance_identity_sha256' "${work}/receipt.json")"
echo "  MEASURED controller_fingerprint ${fingerprint}"

# ---- COMPOSE -------------------------------------------------------------
mkdir -p "$(dirname "${out}")"
jq -n \
    --arg name "${profile_name}" --arg version "${version}" \
    --arg architecture "${architecture}" --arg repository "${repository}" \
    --arg index "${index}" --arg manifest "${manifest}" --arg fingerprint "${fingerprint}" \
    '{name:$name, product:"unifi-network", version:$version, architecture:$architecture,
      image_repository:$repository, image_index_sha256:$index,
      image_manifest_sha256:$manifest, controller_fingerprint:$fingerprint}' >"${out}"

echo "wrote ${out}"
echo "  the fingerprint is measured, the digests are resolved, and nothing here was typed."
echo "  Next: capture this version so the lock agrees, or run-live.sh will refuse the pairing."
