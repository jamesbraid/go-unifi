#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd)
content_store=${GO_UNIFI_CONTENT_STORE:-}
receipt_root=${1:-"$repository_root/.tmp/m0-rebuild"}
image_name=go-unifi-schema-builder:m0

if [ -z "$content_store" ]; then
    echo "GO_UNIFI_CONTENT_STORE must name the restricted artifact store" >&2
    exit 1
fi
if [ -n "$(git -C "$repository_root" status --porcelain)" ]; then
    echo "the hermetic rebuild requires a clean committed worktree" >&2
    exit 1
fi
daemon_arch=$(docker info --format '{{.Architecture}}')
if [ "$daemon_arch" != "x86_64" ] && [ "$daemon_arch" != "amd64" ]; then
    echo "the hermetic rebuild requires a native amd64 Docker daemon; found $daemon_arch" >&2
    exit 1
fi

scratch=$(mktemp -d "${TMPDIR:-/tmp}/go-unifi-m0.XXXXXX")
trap 'rm -rf "$scratch"' EXIT HUP INT TERM
mkdir -p "$scratch/source" "$receipt_root/run-1" "$receipt_root/run-2"
git -C "$repository_root" archive --format=tar HEAD | tar -x -C "$scratch/source"

docker buildx build --platform linux/amd64 --provenance=false --load \
    --metadata-file "$scratch/builder-metadata.json" \
    --file "$repository_root/build/m0/Dockerfile" \
    --tag "$image_name" "$repository_root"

expected_manifest=$(sed -n 's/.*"image_manifest_sha256": "\(sha256:[0-9a-f]*\)".*/\1/p' \
    "$repository_root/build/m0/builder.lock.json")
expected_config=$(sed -n 's/.*"image_config_sha256": "\(sha256:[0-9a-f]*\)".*/\1/p' \
    "$repository_root/build/m0/builder.lock.json")
actual_manifest=$(sed -n 's/.*"containerimage.digest": "\(sha256:[0-9a-f]*\)".*/\1/p' \
    "$scratch/builder-metadata.json")
actual_config=$(docker image inspect --format '{{.Id}}' "$image_name")
if [ -z "$expected_manifest" ] || [ "$actual_manifest" != "$expected_manifest" ]; then
    echo "builder manifest is ${actual_manifest:-<missing>}, lock requires ${expected_manifest:-<missing>}" >&2
    exit 1
fi
if [ -z "$expected_config" ] || [ "$actual_config" != "$expected_config" ]; then
    echo "builder config is ${actual_config:-<missing>}, lock requires ${expected_config:-<missing>}" >&2
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
  "builder_image_manifest_sha256": "${actual_manifest#sha256:}",
  "capture_lock_sha256": "$capture_lock_sha256",
  "first_output_sha256": "$output_sha256",
  "second_output_sha256": "$output_sha256"
}
EOF

echo "two hermetic rebuilds matched: $output_sha256"
echo "receipt: $receipt_root/receipt.json"
