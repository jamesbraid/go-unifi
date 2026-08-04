#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd)
content_store=${GO_UNIFI_CONTENT_STORE:-}
receipt_root=${1:-"$repository_root/.tmp/m0-rebuild"}
image_name=go-unifi-schema-builder:m0-$$
builder_name=go-unifi-m0-$$
lock_file=$repository_root/build/m0/builder.lock.json

if [ -z "$content_store" ]; then
    echo "GO_UNIFI_CONTENT_STORE must name the restricted artifact store" >&2
    exit 1
fi
if [ -n "$(git -C "$repository_root" status --porcelain)" ]; then
    echo "the hermetic rebuild requires a clean committed worktree" >&2
    exit 1
fi
for required_command in docker git jq tar; do
    if ! command -v "$required_command" >/dev/null 2>&1; then
        echo "$required_command is required for the hermetic rebuild" >&2
        exit 1
    fi
done
daemon_arch=$(docker info --format '{{.Architecture}}')
if [ "$daemon_arch" != "x86_64" ] && [ "$daemon_arch" != "amd64" ]; then
    echo "the hermetic rebuild requires a native amd64 Docker daemon; found $daemon_arch" >&2
    exit 1
fi

scratch=$(mktemp -d "${TMPDIR:-/tmp}/go-unifi-m0.XXXXXX")
cleanup() {
    result=$?
    trap - EXIT HUP INT TERM
    docker image rm "$image_name" >/dev/null 2>&1 || true
    docker buildx rm --force "$builder_name" >/dev/null 2>&1 || true
    rm -rf "$scratch"
    exit "$result"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$scratch/source" "$receipt_root/run-1" "$receipt_root/run-2"
git -C "$repository_root" archive --format=tar HEAD | tar -x -C "$scratch/source"

expected_buildkit_image=$(jq -er '.buildkit_image' "$lock_file")
expected_buildkit_index=$(jq -er '.buildkit_index_sha256' "$lock_file")
expected_buildkit_manifest=$(jq -er '.buildkit_platform_manifest_sha256' "$lock_file")
expected_manifest=$(jq -er '.image_manifest_sha256' "$lock_file")
expected_config=$(jq -er '.image_config_sha256' "$lock_file")

docker buildx create --name "$builder_name" --driver docker-container \
    --driver-opt "image=$expected_buildkit_image@$expected_buildkit_index" --use >/dev/null
docker buildx inspect --bootstrap "$builder_name" >/dev/null

for build in 1 2; do
    docker buildx build --builder "$builder_name" --platform linux/amd64 \
        --provenance=false --output "type=oci,dest=$scratch/builder-$build.tar" \
        --file "$repository_root/build/m0/Dockerfile" "$repository_root"
done

oci_manifest() {
    tar -xOf "$1" index.json | jq -er \
        'if (.manifests | length) == 1 and (.manifests[0].digest | test("^sha256:[0-9a-f]{64}$")) then .manifests[0].digest else error("expected one sha256 manifest") end'
}
oci_config() {
    archive=$1
    manifest=$2
    tar -xOf "$archive" "blobs/sha256/${manifest#sha256:}" | jq -er \
        'if (.config.digest | test("^sha256:[0-9a-f]{64}$")) then .config.digest else error("expected one sha256 config") end'
}

actual_manifest=$(oci_manifest "$scratch/builder-1.tar")
second_manifest=$(oci_manifest "$scratch/builder-2.tar")
actual_config=$(oci_config "$scratch/builder-1.tar" "$actual_manifest")
second_config=$(oci_config "$scratch/builder-2.tar" "$second_manifest")
if [ "$actual_manifest" != "$second_manifest" ] || [ "$actual_config" != "$second_config" ]; then
    echo "two builder builds produced different OCI descriptors" >&2
    exit 1
fi
if [ "$actual_manifest" != "$expected_manifest" ]; then
    echo "builder manifest is $actual_manifest, lock requires $expected_manifest" >&2
    exit 1
fi
if [ "$actual_config" != "$expected_config" ]; then
    echo "builder config is $actual_config, lock requires $expected_config" >&2
    exit 1
fi

docker buildx build --builder "$builder_name" --platform linux/amd64 \
    --provenance=false --load \
    --file "$repository_root/build/m0/Dockerfile" \
    --tag "$image_name" "$repository_root"
loaded_config=$(docker image inspect --format '{{.Id}}' "$image_name")
if [ "$loaded_config" != "$expected_config" ]; then
    echo "loaded builder config is $loaded_config, lock requires $expected_config" >&2
    exit 1
fi

for run in 1 2; do
    docker run --rm --platform linux/amd64 --network none --read-only \
        --tmpfs /tmp:rw,nosuid,size=4g \
        --mount "type=bind,src=$scratch/source,dst=/source,readonly" \
        --mount "type=bind,src=$content_store,dst=/content,readonly" \
        --mount "type=bind,src=$receipt_root/run-$run,dst=/out" \
        "$image_name"
done

if ! cmp -s "$receipt_root/run-1/after.json" "$receipt_root/run-2/after.json"; then
    diff -u "$receipt_root/run-1/after.json" "$receipt_root/run-2/after.json" || true
    echo "two clean rebuilds produced different output manifests" >&2
    exit 1
fi

source_commit=$(git -C "$repository_root" rev-parse HEAD)
capture_lock_sha256=$(openssl dgst -sha256 -r "$repository_root/schemas/capture.lock.json" | awk '{print $1}')
output_sha256=$(sed -n 's/.*"output_sha256": "\([0-9a-f]*\)".*/\1/p' \
    "$receipt_root/run-1/after.json")
cat >"$receipt_root/receipt.json" <<EOF
{
  "format_version": 1,
  "source_commit": "$source_commit",
  "platform": "linux/amd64",
  "buildkit_image": "$expected_buildkit_image",
  "buildkit_index_sha256": "$expected_buildkit_index",
  "buildkit_amd64_manifest_sha256": "$expected_buildkit_manifest",
  "builder_image_manifest_sha256": "$actual_manifest",
  "builder_image_config_sha256": "$actual_config",
  "capture_lock_sha256": "$capture_lock_sha256",
  "first_output_sha256": "$output_sha256",
  "second_output_sha256": "$output_sha256"
}
EOF

echo "two hermetic rebuilds matched: $output_sha256"
echo "receipt: $receipt_root/receipt.json"
