# Compatibility policy

UniFi's internal APIs change fast, often, and without a backwards-
compatibility contract — this SDK targets them anyway. This document is the
contract *we* offer instead.

## One supported controller train per release

Each SDK release is generated from exactly one UniFi Network version
(`unifi.UnifiVersion`; named in every release's notes) and tested against
it — unit and schema tests, plus a live-controller integration gate that
runs on every schema change. "Supported" means "tested at that level" —
other controller versions usually work for
overlapping features, but that is best-effort, not a promise.

**Running an older controller? Pin an older SDK tag.** The tag history is
the multi-version story: every release names its controller train, and old
tags keep working against old controllers. Maintenance backports are cut
lazily, on demonstrated need, not proactively.

## How upstream drift is absorbed

Four kinds of drift, four different answers:

- **Upstream drops a *field* the controller still tolerates** → we retain
  it, generated from an explicit pin in `overrides/fields.toml` with a
  deprecation note (e.g. `Network.MdnsEnabled`, `Device.X`/`Device.Y`).
  Pins are reviewed each controller train and sunset when they stop being
  harmless.
- **Upstream removes an *endpoint or resource*** → we remove it in the same
  release, verified dead against a live controller first. Shipping API
  surface that errors on every current controller is worse than an honest
  removal (10.4.57 removed HeatMap, HeatMapPoint, Map, Tag, VirtualDevice
  and the EvaluationScore setting this way).
- **Upstream changes a *wire shape*** → we follow the schema and add a
  tolerant decode where old payloads may still occur (e.g. igmp_snooping
  `querier_addresses` accepts both the pre-10.x string form and the 10.x
  object form).
- **Upstream *relocates* a setting or field** → we follow the move and name
  the successor here. The old name is gone from the schema, so no pin brings
  it back. The mapping is the only thing that helps. Both sides below are Go
  identifiers, because the thing that sends you here is a build that stopped
  compiling. 10.4.57 moved three settings, and the table carries every
  identifier those moves renamed, child types included:

  | Removed | Replacement |
  | --- | --- |
  | `settings.RoamingAssistant` (`Enabled`, `Rssi`) and `Device.RadioTable[].AssistedRoamingEnabled` / `.AssistedRoamingRssi` | `WLAN.RoamingAssistantNaEnabled` / `.RoamingAssistantNaRssi`, and `WLAN.RoamingAssistant6EEnabled` / `.RoamingAssistant6ERssi` for 6 GHz |
  | `settings.Ips.Suppression` (`*SettingIpsSuppression`) | `settings.IpsSuppression`, a setting in its own right (key `ips_suppression`) |
  | `settings.SettingIpsAlerts` / `SettingIpsTracking` / `SettingIpsWhitelist` | `settings.SettingIpsSuppressionAlerts` / `SettingIpsSuppressionTracking` / `SettingIpsSuppressionWhitelist` |
  | `settings.Usg.GeoIPFilteringEnabled` / `.GeoIPFilteringBlock` / `.GeoIPFilteringCountries` / `.GeoIPFilteringTrafficDirection` | `settings.UsgGeo.IPFiltering` (`*SettingUsgGeoIPFiltering`), whose members are `Enabled`, `Action`, `Countries`, `TrafficDirection` |

  Two of those are more than a rename. Roaming assistance stopped being one
  site setting plus a per-radio device field and became two per-band WLAN
  ones, so a caller carrying it on `Device.RadioTable` has to decide which
  band it meant. And `geo_ip_filtering_block` became `Action` while keeping
  its `block|allow` values — same values, new field name, new setting.

## Versioning honesty

Semantic versioning here tracks the **Go API**, with one deliberate,
long-standing deviation: **controller-forced breaking changes ship in minor
releases**, prominently documented. Strictly-semver majors would burn a
major version per controller train (constant `/vN` import-path churn),
which serves consumers worse than documented breaking minors — the same
trade most vendor-API SDKs make. In exchange:

- Every regeneration is checked with `apidiff` against the previous
  release; breaking changes can never auto-merge or auto-release — a human
  reviews the full diff and cuts the tag deliberately, and the release
  notes carry the complete incompatibility list.
- If you need Go-module-strict guarantees, **pin an exact version** and
  upgrade deliberately; do not rely on `go get -u` being safe across
  controller trains.
- Major versions (`/v2`, ...) are reserved for deliberate redesigns of this
  SDK itself, not for upstream's schema churn.
