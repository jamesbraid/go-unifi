module github.com/ubiquiti-community/go-unifi

go 1.25.0

toolchain go1.26.5

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/hashicorp/go-retryablehttp v0.7.8
	github.com/hashicorp/go-version v1.9.0
	github.com/hashicorp/terraform-plugin-codegen-spec v0.2.0
	github.com/iancoleman/strcase v0.3.0
	github.com/klauspost/compress v1.19.0
	github.com/moby/moby/api v1.54.2
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.43.0
	github.com/ulikunitz/xz v0.5.15
	pault.ag/go/debian v0.21.0
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Masterminds/sprig/v3 v3.2.3 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.4.1 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/cli v1.1.7 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/terraform-plugin-codegen-framework v0.4.1 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	// unifi-emu backs the controllertest device simulator (imported only
	// from integration-tagged files). Two lifecycle states:
	//
	// While unpublished (now): the placeholder version below resolves
	// locally through a gitignored go.work (`go work use <checkout>`,
	// plus a replace of this exact version — unifi-emu has its own
	// requires, so the graph consults the required version's go.mod even
	// in workspace mode). The // indirect marker is deliberate despite
	// the direct import: pruning then skips the unresolvable
	// placeholder's go.mod, keeping untagged `go build ./...` / `go test
	// ./...` green without the workspace. `go mod tidy` walks all tags,
	// so it needs the workspace, and it will re-mark this direct; keep
	// the marker when committing.
	//
	// After publishing: bump the require to the real tag and drop the
	// marker; the go.work setup becomes optional (local hacking only).
	github.com/jamesbraid/unifi-emu v0.0.0-00010101000000-000000000000 // indirect
	github.com/kjk/lzma v0.0.0-20161016003348-3fd93898850d // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.2.0 // indirect
	github.com/moby/moby/client v0.4.0 // indirect
	github.com/moby/patternmatcher v0.6.1 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/posener/complete v1.2.3 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/shirou/gopsutil/v4 v4.26.5 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.41.0 // indirect
	go.opentelemetry.io/otel/metric v1.41.0 // indirect
	go.opentelemetry.io/otel/trace v1.41.0 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	pault.ag/go/topsort v0.1.1 // indirect
)

tool (
	github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework
	golang.org/x/tools/cmd/stringer
)
