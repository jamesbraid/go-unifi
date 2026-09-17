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
  deprecation note (e.g. `Network.MdnsEnabled`).
  Pins are reviewed each controller train and sunset when they stop being
  harmless. A pin requires a live measurement that the controller still
  honors the field; when it does not, the removal stands. PortProfile's
  `tagged_networkconf_ids` (dropped upstream in 2023) is the measured
  counter-example: 10.4.57 strips it from every create and update shape —
  port profiles express tagged VLANs only as `tagged_vlan_mgmt` +
  `excluded_networkconf_ids` — so the SDK does not model it, and
  `TestIntegrationPortProfileTaggedNetworks` holds the measurement in
  place. (Device `port_overrides` still honors the same wire name; the two
  write paths differ.)
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
  | `settings.RoamingAssistant` (`Enabled`, `Rssi`) — one value for the site | `WLAN.RoamingAssistantNaEnabled` / `.RoamingAssistantNaRssi`, plus `WLAN.RoamingAssistant6EEnabled` / `.RoamingAssistant6ERssi` for 6 GHz. Set them on each WLAN that had the site behaviour |
  | `DeviceRadioTable.AssistedRoamingEnabled` / `.AssistedRoamingRssi` — one value per AP radio | **None with the same scope.** See below |
  | `settings.Ips.Suppression` (`*SettingIpsSuppression`) | `settings.IpsSuppression`, a setting in its own right (key `ips_suppression`) |
  | `settings.SettingIpsAlerts` / `SettingIpsTracking` / `SettingIpsWhitelist` | `settings.SettingIpsSuppressionAlerts` / `SettingIpsSuppressionTracking` / `SettingIpsSuppressionWhitelist` |
  | `settings.Usg.GeoIPFilteringEnabled` / `.GeoIPFilteringBlock` / `.GeoIPFilteringCountries` / `.GeoIPFilteringTrafficDirection` | `settings.UsgGeo.IPFiltering` (`*SettingUsgGeoIPFiltering`), whose members are `Enabled`, `Action`, `Countries`, `TrafficDirection` |

  Roaming assistance is the one to read carefully, because two different
  things were removed and only one of them has a real successor.

  The site setting moved to the WLAN. That narrows its scope — one value
  per site becomes one per WLAN — but the behaviour is expressible: set it
  on every WLAN that relied on the site default.

  The per-radio device fields did not move anywhere. `DeviceRadioTable`
  held a value per AP radio, and nothing in the current schema does. The
  WLAN fields are the nearest equivalent and are not a substitute: they
  apply to every AP broadcasting that WLAN, so **a deployment that set
  different assisted-roaming values on individual APs cannot express that
  any more**, on any band. Band narrows it further — `Radio` still takes
  `ng|na|ad|6e` while the only successors are `Na` (5 GHz) and `6E`, so a
  2.4 GHz or 60 GHz configuration has nothing to move to at all. Porting
  either onto the `Na`/`6E` fields does not preserve it; it applies a
  policy to APs and bands it was never written for.

  `geo_ip_filtering_block` is the milder case: it became `Action` while
  keeping its `block|allow` values — same values, new field name, new
  setting.

## Removals we chose, not upstream

The four cases above are upstream's doing. We also drop our own surface
when nothing uses it: a type left behind by a reverted design, a helper
that never needed to be part of the contract, a parameter the
implementation ignores. A retained pin that has stopped working belongs
here too — we chose to add it, so dropping it is our call, not upstream's.
These are still breaking changes and ship like any other here, in a
documented minor under the review discipline below.

They need one thing upstream drift does not. For unused surface the whole
justification is that nothing calls it, so every known consumer is searched
before the removal lands, and the commit names what was searched. For a
sunset pin the justification is the opposite direction: the controller
itself no longer honors the field, and the commit names the measurement
that showed it.

Gone this way so far, with the successor where there is one:

- `Options` — never read by anything. `Config` configures a client.
- `FindOwnerHost`, `FindHostByHardwareID` — unexported. Each has exactly
  one caller, inside the SDK, and neither belonged in the contract.
- `(*ApiClient).GetCloudConsoleID` — no successor. The console ID is
  selected once at construction, from `Config.CloudConnector` and
  `Config.HardwareID`.
- `types.MAC`, `types.MACString`, `types.MACStrings` — no successor and no
  loss. They existed to normalise MACs on decode, and that was reverted so
  a field holds what the controller sent. `types.NormalizeMAC` survives.
  It is what the lookup helpers call.
- `ListNetwork`'s trailing `params ...[]struct{key, val string}` — the body
  ignored it, and its unexported fields made a non-empty argument
  impossible to build from outside the package. Call
  `ListNetwork(ctx, site)`.
- `Device.X`, `Device.Y`, `Device.MapID`, `Device.HeightInMeters` — the
  map-placement properties, pinned when 10.x dropped them from the schema
  along with the maps feature. No successor: placing a device on a floor
  plan is not something this API does any more. Measured on 10.6.101 —
  a `rest/device` write carrying all four is accepted with rc: ok and
  stores none of them, and a freshly adopted device returns none of them
  either.
- `WLAN.WLANGroupID` — pinned when Network 6 removed it. The successor is
  `WLAN.ApGroupIDs`; AP groups are how a WLAN is scoped now. Measured on
  10.6.101 — a WLAN create that never mentions it succeeds, and a create
  that names a real WLAN group is accepted with rc: ok and comes back
  without the key.
- `settings.Usg.MdnsEnabled` — pinned when Network 7 removed it. The
  successor is the site-level `mdns` setting. Measured on 10.6.101 —
  `get/setting/usg` returns 34 keys and none of them is `mdns_enabled`,
  and a `set/setting/usg` write of it is accepted with rc: ok and does
  not come back. `Network.MdnsEnabled` is a different field and stays:
  the same measurement finds the controller storing and returning it on
  a network.
- `Cmd`, `(*ApiClient).ExecuteCmd` — no successor. A generic escape hatch for
  posting an arbitrary site command, added once and never called by
  anything in this SDK, its tests, or `cmd/`. Every command the SDK
  actually issues (adopt, delete-device, add/delete/update-site, firewall
  reorder) builds its own typed request and goes through the unexported
  `siteCommand`, which decodes the command manager's answer and reports
  when a command was accepted but not acted on. `ExecuteCmd` discarded the
  response body outright (`var respBody struct{}`), so it could never have
  told a performed command from an ignored one — the exact failure mode
  `siteCommand`'s callers now guard against. Call the wrapper for the
  command you need instead; if none exists, that is a gap to fill with a
  typed one, not a reason to keep an untyped one around.
- `ClientInfoDeviceInfo` — renamed to `ClientInfoUnifiDeviceInfo`. The type
  is unchanged; only its name moved. `ClientInfo` is now generated from the
  document the controller actually returns rather than hand-written, and the
  generator names a nested type after the wire key that carries it — here
  `unifi_device_info`, which the hand-written name had shortened. The field
  on `ClientInfo` keeps its own name (`UnifiDeviceInfo`); a caller that only
  reaches the nested value through that field needs no change, and one that
  names the type does.

## Versioning honesty

Semantic versioning here tracks the **Go API**, with a deliberate,
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
