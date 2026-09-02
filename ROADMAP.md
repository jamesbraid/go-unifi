# Roadmap: derive more, maintain less

Date: 2026-09-02

Status: proposed. Replaces the release-train roadmap, which completed with
v1.106.0 through v1.113.0. Coordinated with terraform-provider-unifi and
unifi-containers; each of the three projects ships exactly one tagged
release at the end of this plan, and nothing is tagged before then. Work
accumulates on main, which is always pushed to the mirror, so consumers pin
commits in the meantime.

## Aim

Every addition must retire more hand-maintained code than it adds. The last
cycle's defects shared one root: a human transcribed something the
controller already knows — six redaction substrings against 64 declared
sensitive fields, discard lists measured once and never re-measured,
ownership tables stamped with a controller version two releases old,
hand-written v2 API definitions missing fields the controller's own code
carries. Each item below replaces a class of transcription with a derivation
and deletes the transcription.

## 1. A measured-behaviour stage in the capture pipeline

The controller does things its published schema does not describe: it
silently coerces the twelve connection-tracking timeouts to per-field
floors, silently discards fields written alongside a preference of "auto",
accepts-then-ignores fields per firmware generation, and treats some fields
differently on create than on update. Today each discovery is a one-off
integration test with a hand-pasted baseline.

Build one probe framework that runs at capture time against the disposable
controller: write boundary and probe values, re-read what was stored, and
emit a versioned machine-readable artifact (beside the field definitions in
`schemas/`) recording coercion floors, discard sets, preference ownership,
and create-versus-update divergence. The generator turns that artifact into
exported constraints, the way validation patterns and sensitive fields are
exported today. Re-measured on every controller pin, so a behaviour change
surfaces in the capture diff instead of three nights of red CI.

Retires: the hand-pasted ownership blocks in `overrides/fields.toml`, the
`wantDiscarded` lists in the round-trip test, the separate clearing-,
null-write- and preference-probe scaffolding (consolidated into the
framework), and the provider's parked hand-carried timeout floors. Also
closes the write-twice gap (fields accepted once then dropped on update)
as a probe mode rather than another test.

## 2. Retire the hand-written v2 API definitions

Eight resource definitions under `overrides/resources/` are hand-maintained
and drift; scanning the controller's compiled classes found three missing
firewall-policy fields last cycle. Promote that scan into the capture: at
minimum a cross-check that fails when a hand-written definition disagrees
with the compiled model, at best generation of the definition itself.

Retires: as much of the eight JSON files as generation can carry, and the
class of "hand definition missing a field" outright.

## 3. Extend drift detection to devices

The wire-versus-model comparison covers four configuration collections and
not the Device object — the largest and most state-heavy in the API, which
is how `last_config_applied_successfully` went unmodelled. One table entry
to add; budgeted as a discovery task because it will surface unmodelled
state needing triage. Adds no new machinery.

## 4. Record Java field types at capture

Both this repo and the provider now reason about "which fields can arrive
as JSON numbers" from one manual read of the controller's compiled code,
and a controller release that retypes a field is silent. Record type
descriptors per field at capture time alongside the names, and the
number-or-word decode rule derives from data instead of a probe heuristic.

Retires: the manual jar-reading step, and the heuristic's probe corpus.

## 5. Dependency updates on the forge

The GitHub fork carries a Dependabot config that has never run once —
forks do not get Dependabot, so every action and module pin is manually
maintained while appearing covered. Run the updater on the canonical forge
instead, via Forgejo Actions (enabled for this repo per the runbook's
migration step — James's call, made). Tool choice at build time: dependabot
proper if it runs unmodified there, Renovate if dependabot turns out to be
GitHub-bound.

Retires: the dead `.github/dependabot.yml`, and the false sense of coverage.

## 6. Gate tagging in CI

A pushed tag currently publishes a release whatever any test says, on this
repo and the provider's alike; v1.111.0 shipped while nightly integration
had been red for two days. Add a preflight to the release workflow that
fails when the latest integration run on main is not green, with an
explicit override input for a red that is understood and accepted, and
fold the pre-tag checklist into it.

Retires: the human checklist as the only protection.

## 7. UOS image carries the current Network app (unifi-containers)

Owned by unifi-containers, consumed here. UniFi OS Server structurally
trails the standalone package, so the UOS test image bundles an older
Network app than the one the schemas target — half the integration matrix
green about the wrong controller until the pin moved. Real consoles update
the Network app in place (measured: the firmware API serves the current
Network package for the UOS platform string), so an image variant that
applies that update is faithful to deployed systems. Once both matrix arms
run the same Network version, the ownership-table ambiguity (platform
difference versus version difference) disappears and `uos_excludes`
becomes trustworthy again.

## Code accounting

Additions: one probe framework, one artifact schema, one class-scan check,
one capture field (types), one updater workflow, one release preflight.

Deletions: ownership TOML blocks, discard-list baselines, three separate
probe scaffolds, eight (or most of eight) hand-written v2 definitions, the
dead Dependabot config, the manual pre-tag checklist, the provider's
hand-transcribed floors and sensitive lists (their side), and the jar-read
step. Reviewed per pull request: any item that does not delete at least as
much as it adds gets rejected or re-scoped.

## Release plan

Order: unifi-containers first (image with updated Network app), because
item 1 re-measures against it and item 7 depends on it. Then this repo's
items land on main untagged. The provider derives against a pinned commit
throughout. When the work is proven, each project cuts exactly one tag —
one image release, one SDK tag, one provider tag — on James's word, and
nothing before.
