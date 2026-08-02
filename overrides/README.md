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
  `add = true` compat fields), plus the `preference` tables described
  below. See its header comment for semantics.

## Preference tables

`[Resource.preference.<wire>]` records what an `auto|manual` mode field owns.
While such a field is `auto` the controller manages a block of siblings
itself: it accepts a payload that sets them, answers `rc: ok`, stores its own
values, and reports nothing. A caller finds out from the next read, from a
downstream diff, or not at all.

Nothing in the extracted schema describes this. Each field's validator
stands alone, and an `auto|manual` field looks like any other two-value
enum, so ownership can only be measured: write the same object twice, once
under each mode, and compare each write against what it asked for.
`TestIntegrationPreferenceOwnership` does that against a live controller and
prints the entry to paste, including the build it measured.

Don't hand-edit the `owns` lists. Re-run the sweep. The measurement finds
fields a field-by-field reading of the encoder cannot — `setting_preference`
owns twelve, four of which are not `*_enabled` toggles at all, which is why
earlier counts stopped at six.

### Nested modes

Not every mode sits on the resource. `Device.setting_preference` lives inside
`port_overrides`, and the gateway's inside `dns_verification`. Address those
with a dotted path, and **quote the key**:

```toml
[Device.preference."port_overrides.setting_preference"]
owns = ["stp_port_mode"]
measured = "10.4.57"
```

An unquoted dotted key is not a syntax error. TOML reads it as nested tables,
decodes cleanly, and produces an entry that governs nothing — the same silent
failure these tables document, one layer up.

`owns` stays relative to the object holding the mode. A mode governs its own
object, and `port_overrides` is an array: each element carries its own mode
governing that element, so there is no single sibling path to write.

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
