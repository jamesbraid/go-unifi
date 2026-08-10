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
# The -sim variant. The registry carries three per version and only this one
# accepts the campaign credentials: the bare image is a fresh controller with
# no administrator, and -seeded seeds admin/unifi-containers-seeded, which is
# not what the pipeline holds. Both wrong choices fail identically and
# misleadingly -- a 400 from /api/login, retried until the readiness budget
# expires and reported as "controller did not become ready", when the
# controller was ready in seconds and simply refused the login.
#
# Confirmed rather than reasoned: resolving 10.4.57-sim reproduces both digests
# the committed profile pins, exactly. "sim" is the image variant; "seeded" in
# the profile name is the profile CLASS the campaign asks for (m4 passes
# -profile-class fresh_seeded). They are different words for different things
# and the collision is the whole trap.
readonly tag=${ONBOARD_IMAGE_TAG:-${version}-sim}
readonly expected_probe=${ONBOARD_EXPECTED_PROBE:-network-sim}

for tool in docker jq go curl sha256sum; do
    command -v "${tool}" >/dev/null || { echo "onboard: ${tool} is required" >&2; exit 1; }
done

echo "onboarding ${profile_name}"

# ---- RESOLVED ------------------------------------------------------------
# Ask the registry directly rather than going through a docker CLI plugin.
# `docker manifest inspect --verbose` is wrong here -- it returns a
# per-platform descriptor that looks like an index digest and is not one --
# and `docker buildx imagetools` is right but ships as a separate plugin that
# the pinned static docker tarball does not contain. The registry API needs
# only curl and jq, needs no daemon, and works on any architecture. It was
# checked against buildx on the same tag and returns the identical digest.
#
# The digest is VERIFIED, not accepted: an index digest is the sha256 of the
# manifest bytes, so we hash the body ourselves and require the registry's
# Docker-Content-Digest to agree. A registry that reported a digest which did
# not describe what it just served would otherwise be believed.
#
# An earlier version of this comment claimed ghcr's 10.4.57 had been
# republished since the profile was authored. That was wrong, and wrong in an
# avoidable way: it compared the bare tag against a profile that pins the -sim
# one. 10.4.57-sim resolves to the committed digests exactly and always has.
# Pinning a digest is still right -- a tag is a moving reference and this
# profile is a claim about specific bytes -- but no tag actually moved here.
readonly registry=${repository%%/*}
readonly repo_path=${repository#*/}

token=$(curl --fail --silent --show-error \
    "https://${registry}/token?scope=repository:${repo_path}:pull" | jq -r '.token // empty')
# Checked explicitly rather than with ${VAR:?message}. Measured: when the shell
# dies from a :? expansion error, an EXIT trap observes $? as 0 and the script
# exits 0 -- so every one of these checks reported success while doing nothing.
# The trap form does not matter; any EXIT trap does it. Explicit exit 1 works.
if [[ -z ${token} ]]; then
    echo "onboard: could not obtain a pull token for ${repository}" >&2
    exit 1
fi
readonly token

headers=$(mktemp)
body=$(mktemp)
if ! curl --fail --silent --show-error --dump-header "${headers}" --output "${body}" \
    --header "Authorization: Bearer ${token}" \
    --header 'Accept: application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json' \
    "https://${registry}/v2/${repo_path}/manifests/${tag}"; then
    echo "onboard: ${repository}:${tag} could not be fetched from the registry" >&2
    rm -f "${headers}" "${body}"
    exit 1
fi

index=$(tr -d '\r' <"${headers}" | awk 'tolower($1) == "docker-content-digest:" {print $2}')
computed="sha256:$(sha256sum "${body}" | cut -d' ' -f1)"
if [[ ${index} != "${computed}" ]]; then
    echo "onboard: the registry reported a digest that does not describe what it served" >&2
    echo "  reported: ${index:-<none>}" >&2
    echo "  computed: ${computed}" >&2
    rm -f "${headers}" "${body}"
    exit 1
fi

manifest=$(jq -r --arg arch "${architecture}" \
    '.manifests[]? | select(.platform.architecture == $arch and .platform.os == "linux") | .digest' \
    "${body}" | head -n1)
media=$(jq -r '.mediaType // "<none>"' "${body}")
rm -f "${headers}" "${body}"
if [[ -z ${manifest} ]]; then
    echo "onboard: ${repository}:${tag} advertises no linux/${architecture} manifest" >&2
    echo "  mediaType ${media}; a single-platform image cannot serve an ${architecture} profile" >&2
    exit 1
fi
# Verify the variant before starting anything. Picking the wrong one costs a
# full readiness budget and then lies about why: the controller comes up, the
# login is refused, and the tool reports "did not become ready". The image says
# which variant it is in READYZ_PROBE, so ask, and fail in a second with the
# real reason instead of in five minutes with a wrong one.
config_digest=$(curl --fail --silent --show-error --location \
    --header "Authorization: Bearer ${token}" \
    --header 'Accept: application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json' \
    "https://${registry}/v2/${repo_path}/manifests/${manifest}" | jq -r '.config.digest // empty')
if [[ -n ${config_digest} ]]; then
    probe=$(curl --fail --silent --show-error --location \
        --header "Authorization: Bearer ${token}" \
        "https://${registry}/v2/${repo_path}/blobs/${config_digest}" |
        jq -r '(.config.Env // [])[] | select(startswith("READYZ_PROBE=")) | sub("^READYZ_PROBE=";"")')
    if [[ -z ${probe} ]]; then
        echo "onboard: could not read READYZ_PROBE from ${repository}:${tag}" >&2
        echo "  refusing rather than continuing: this check exists because the wrong" >&2
        echo "  variant fails five minutes later with a misleading message" >&2
        exit 1
    fi
    if [[ ${probe} != "${expected_probe}" ]]; then
        echo "onboard: ${repository}:${tag} is the wrong image variant" >&2
        echo "  READYZ_PROBE reports ${probe}, and this profile needs ${expected_probe}" >&2
        echo "  the campaign credentials only exist in the ${expected_probe#network-} image;" >&2
        echo "  the others come up healthy and then refuse the login" >&2
        exit 1
    fi
    echo "  VERIFIED variant            ${probe:-unknown} (from the image, not the tag name)"
fi

readonly index manifest
echo "  RESOLVED repository         ${repository}"
echo "  RESOLVED image_index        ${index}  (digest verified against the bytes served)"
echo "  RESOLVED image_manifest     ${manifest}  (${architecture})"

# ---- MEASURED ------------------------------------------------------------
work=$(mktemp -d)
container=""
cleanup() {
    # Capture the status FIRST and re-raise it last. Without this the trap's
    # own final command becomes the script's exit status, so every failure
    # after the trap is installed -- a missing credential, a controller that
    # never became ready, a provisioner receipt that refused the container --
    # exited 0 and reported success while writing no profile. A step that goes
    # green having measured nothing is worse than one that fails.
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
