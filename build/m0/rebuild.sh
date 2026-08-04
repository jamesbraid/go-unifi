#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd)
content_store=${GO_UNIFI_CONTENT_STORE:-}
receipt_root=${1:-"$repository_root/.tmp/m0-rebuild"}
image_name=go-unifi-schema-builder:m0-$$
first_builder_name=go-unifi-m0-$$-1
second_builder_name=go-unifi-m0-$$-2
lock_file=$repository_root/build/m0/builder.lock.json

if [ -z "$content_store" ]; then
    echo "GO_UNIFI_CONTENT_STORE must name the restricted artifact store" >&2
    exit 1
fi
for required_command in docker git jq tar openssl; do
    if ! command -v "$required_command" >/dev/null 2>&1; then
        echo "$required_command is required for the hermetic rebuild" >&2
        exit 1
    fi
done
scratch=$(mktemp -d "${TMPDIR:-/tmp}/go-unifi-m0.XXXXXX")
cleanup() {
    result=$?
    trap - EXIT HUP INT TERM
    docker image rm "$image_name" >/dev/null 2>&1 || true
    docker buildx rm --force "$first_builder_name" >/dev/null 2>&1 || true
    docker buildx rm --force "$second_builder_name" >/dev/null 2>&1 || true
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
expected_buildkit_version=$(jq -er '.buildkit_version' "$lock_file")
expected_buildkit_index=$(jq -er '.buildkit_index_sha256' "$lock_file")
expected_buildkit_manifest=$(jq -er '.buildkit_platform_manifest_sha256' "$lock_file")
expected_source_date_epoch=$(jq -er '.source_date_epoch' "$lock_file")
expected_compatibility_version=$(jq -er '.buildkit_compatibility_version' "$lock_file")
expected_manifest=$(jq -er '.image_manifest_sha256' "$lock_file")
expected_config=$(jq -er '.image_config_sha256' "$lock_file")

digest_file() {
    openssl dgst -sha256 -r "$1" | awk '{print $1}'
}

verify_recorded_digest() {
    name=$1
    lock_path=$2
    filename=$3
    expected=$(jq -er "$lock_path" "$lock_file")
    actual=$(digest_file "$filename")
    if [ "$actual" != "$expected" ]; then
        echo "$name digest is $actual, lock requires $expected" >&2
        exit 1
    fi
}

verify_recorded_digest Dockerfile '.dockerfile_sha256' "$repository_root/build/m0/Dockerfile"
verify_recorded_digest container-rebuild '.container_rebuild_sha256' "$repository_root/build/m0/container-rebuild"
verify_recorded_digest go.mod '.go_mod_sha256' "$repository_root/go.mod"
verify_recorded_digest go.sum '.go_sum_sha256' "$repository_root/go.sum"
verify_recorded_digest rebuild.sh '.rebuild_script_sha256' "$repository_root/build/m0/rebuild.sh"

if [ -n "$(git -C "$repository_root" status --porcelain)" ]; then
    echo "the hermetic rebuild requires a clean committed worktree" >&2
    exit 1
fi

daemon_arch=$(docker info --format '{{.Architecture}}')
if [ "$daemon_arch" != "x86_64" ] && [ "$daemon_arch" != "amd64" ]; then
    echo "the hermetic rebuild requires a native amd64 Docker daemon; found $daemon_arch" >&2
    exit 1
fi

docker buildx create --name "$first_builder_name" --driver docker-container \
    --driver-opt "image=$expected_buildkit_image@$expected_buildkit_index" --use >/dev/null
docker buildx inspect --bootstrap "$first_builder_name" >/dev/null

docker buildx build --builder "$first_builder_name" --platform linux/amd64 \
    --provenance=false --build-arg "SOURCE_DATE_EPOCH=$expected_source_date_epoch" \
    --output "type=oci,dest=$scratch/builder-1.tar,rewrite-timestamp=true,compatibility-version=$expected_compatibility_version" \
    --file "$repository_root/build/m0/Dockerfile" "$repository_root"
docker buildx rm --force "$first_builder_name" >/dev/null

docker buildx create --name "$second_builder_name" --driver docker-container \
    --driver-opt "image=$expected_buildkit_image@$expected_buildkit_index" --use >/dev/null
docker buildx inspect --bootstrap "$second_builder_name" >/dev/null
docker buildx build --builder "$second_builder_name" --platform linux/amd64 \
    --provenance=false --build-arg "SOURCE_DATE_EPOCH=$expected_source_date_epoch" \
    --output "type=oci,dest=$scratch/builder-2.tar,rewrite-timestamp=true,compatibility-version=$expected_compatibility_version" \
    --file "$repository_root/build/m0/Dockerfile" "$repository_root"

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

docker buildx build --builder "$second_builder_name" --platform linux/amd64 \
    --provenance=false --build-arg "SOURCE_DATE_EPOCH=$expected_source_date_epoch" \
    --output "type=docker,rewrite-timestamp=true" \
    --file "$repository_root/build/m0/Dockerfile" \
    --tag "$image_name" "$repository_root"
loaded_config=$(docker image inspect --format '{{.Id}}' "$image_name")
if [ "$loaded_config" != "$expected_config" ]; then
    echo "loaded builder config is $loaded_config, lock requires $expected_config" >&2
    exit 1
fi

for run in 1 2; do
    docker run --rm --platform linux/amd64 --network none --read-only \
        --tmpfs /tmp:rw,exec,nosuid,size=4g \
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
canonical_repository=$(sed -n 's/^module //p' "$repository_root/go.mod")
capture_lock_sha256=$(digest_file "$repository_root/schemas/capture.lock.json")
jq -n -S \
    --arg canonical_repository "$canonical_repository" \
    --arg source_commit "$source_commit" \
    --arg network_mode none \
    --arg builder_manifest "$actual_manifest" \
    --arg builder_config "$actual_config" \
    --arg capture_lock_sha256 "$capture_lock_sha256" \
    --slurpfile builder_lock "$lock_file" \
    --slurpfile input_lock "$repository_root/schemas/capture.lock.json" \
    --slurpfile run_1 "$receipt_root/run-1/after.json" \
    --slurpfile run_2 "$receipt_root/run-2/after.json" \
    '{
      format_version: 2,
      canonical_repository: $canonical_repository,
      source_commit: $source_commit,
      builder: {
        lock: $builder_lock[0],
        manifest_sha256: $builder_manifest,
        config_sha256: $builder_config
      },
      input_lock: {
        capture: $input_lock[0],
        sha256: $capture_lock_sha256
      },
      network: {mode: $network_mode, disabled: true},
      negative_gates: {
        dockerfile_digest: "passed",
        container_rebuild_digest: "passed",
        module_digests: "passed",
        rebuild_script_digest: "passed"
      },
      output_manifests: {run_1: $run_1[0], run_2: $run_2[0]}
    }' >"$receipt_root/receipt.json"

output_sha256=$(jq -er '.output_manifests.run_1.output_sha256' "$receipt_root/receipt.json")

echo "two hermetic rebuilds matched: $output_sha256"
echo "receipt: $receipt_root/receipt.json"
