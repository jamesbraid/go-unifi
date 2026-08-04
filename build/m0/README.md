# Hermetic schema rebuild

`rebuild.sh` runs two clean Linux/amd64 rebuilds and compares the complete
generated-output manifest. The builder contains Go 1.26.5 and the full module
cache from `go.sum`. Runtime networking is disabled.

The Docker daemon must run natively on amd64. Cross-architecture emulation is
not accepted as promotion evidence. The source worktree must also be clean so
both runs start from the same committed archive.

```sh
export GO_UNIFI_CONTENT_STORE=/restricted/go-unifi/artifacts
./build/m0/rebuild.sh
```

The content store is mounted read-only. Each container gets a fresh source copy,
Go build cache, home directory, and temporary directory. Within each run, the
pre-generation and post-generation manifests must match. The two post-generation
manifests must then match each other byte-for-byte.

The receipt under `.tmp/m0-rebuild/receipt.json` records the source commit,
builder manifest, capture-lock digest, platform, and output digest. Logs and
per-file manifests remain beside it for diagnosis.

`builder.lock.json` pins the base OCI index, selected amd64 manifest, Dockerfile
frontend, builder inputs, final image manifest, and image config. When any
builder input changes, rebuild the image with `--provenance=false`, verify two
identical image manifests, and update the lock in the same review.
