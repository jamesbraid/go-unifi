# Overrides

Human-maintained schema inputs, layered on top of the definitions extracted
from the controller at build time (see `schemas/README.md` for the full
override-layer story):

- `resources/*.json` — whole-resource wire-format definitions for API
  objects the controller's own schema files don't describe (the internal
  v2 endpoints). Same format as the extracted files: wire field name →
  validation string.
- `fields.toml` — declarative per-resource REST paths and per-field
  overrides (shape pins, renames, wire retags, envelope removals, and
  `add = true` compat fields). See its header comment for semantics.

## Provenance

`ApGroups.json` and `NetworkMembersGroup.json` are imported from
[ubiquiti-community/unifi-api](https://github.com/ubiquiti-community/unifi-api)
(`cmd/fields/custom/`, MPL-2.0 — same license as this repository), which
maintains the same style of hand-written v2 definitions. Their
`NetworkMembersGroup.json` carries an explicit `id` field; here the base
envelope's `_id` is retagged to `id` via `fields.toml` instead, so the file
holds only the resource's own fields.

`PowerSupervisor.json` from the same source is deliberately NOT imported:
`unifi/power_supervisor.go` is hand-tuned (semantic doc comments, computed
read-only fields, `PowerSupervisorSource` naming) and generating it today
would rename its nested types and drop that tuning for no functional gain.
Revisit if the wire format drifts.

All of these are reverse-engineered observations of the controller's
internal v2 API — nothing here is extracted from Ubiquiti software. A
live-controller drift probe (part of the controller-testing work)
keeps them honest.
