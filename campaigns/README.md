# Compatibility campaigns

Compatibility campaigns compare candidate scout evidence with the distinct,
tracked, last-admitted catalog. They emit redacted, canonical attestations for
review. A campaign cannot admit an operation, change provider behavior, or
publish a support claim.

Run the deterministic offline campaign with:

```sh
go generate ./scout ./campaigns
```

The admitted DNS catalog is accepted only with the checked admission receipt,
which binds its canonical digest, catalog identity, operation identity, review
revision, and provenance. Campaign automation reads both files and never
rewrites them.

The profile matrix records fresh seeded evidence and keeps the persisted
single-hop and long-lived multi-hop classes explicitly uncovered until their
digest-pinned fixtures exist. The mutation attestation proves that an
unreviewed controller field produces a capture-invalid candidate.

Each invocation requires an execution receipt and an append-only attempt
ledger. The command appends `started` before validation and then appends either
`succeeded` or `failed`. Failed attempts remain in the ledger and produce no
attestation. Offline receipts stay fixture-scoped. Runner receipts bind the
checked workflow digest, source revision, pipeline identity, configured image,
and observed Go toolchain.

Set `CAMPAIGN_LIVE=1` on the isolated Skunkworks job to run
`campaigns/run-live.sh`, supplying it as step environment or a secret rather
than a pipeline substitution: Woodpecker resolves `${...}` in commands before
the shell runs, so the guard is written `$${CAMPAIGN_LIVE:-0}` to reach the
shell at all. Until it was escaped this instruction could not succeed, whoever
followed it.

The step must also mount the Docker socket — installing a client is not enough,
and without a daemon the script exits 125 at its first `docker run`. The runner
must provide `UNIFI_NETWORK_IMAGE` as the locked index-digest reference, plus
`UNIFI_USERNAME` and `UNIFI_PASSWORD` as secrets. The script starts a
disposable Network container, uses the in-repo provisioner to inspect its image
and read its version and UUID, then runs live scout and a candidate-only
campaign. It retains the provisioner receipt, runner receipt, scout outputs,
attempt ledger, and attestation under `.campaign-results/<pipeline>`, and logs
their digests. `UNIFI_USERNAME` and `UNIFI_PASSWORD` default to the disposable
target's seeded credentials supplied by the Skunkworks job.
