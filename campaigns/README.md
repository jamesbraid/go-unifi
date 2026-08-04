# Compatibility campaigns

Compatibility campaigns compare candidate scout evidence with the distinct,
tracked, last-admitted catalog. They emit redacted, canonical attestations for
review. A campaign cannot admit an operation, change provider behavior, or
publish a support claim.

Run the deterministic offline campaign with:

```sh
go generate ./scout ./campaigns
```

The profile matrix records fresh seeded evidence and keeps the persisted
single-hop and long-lived multi-hop classes explicitly uncovered until their
digest-pinned fixtures exist. The mutation attestation proves that an
unreviewed controller field produces a capture-invalid candidate. Every
attempt records builder, controller, and runtime provenance. Offline evidence
is explicitly fixture-scoped. Live evidence must come from the runner and
provisioner.
