// Package dnsrecord defines the transport-neutral DNS record control-plane
// operation. It deliberately contains no API route or fallback transport.
package dnsrecord

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// OperationArtifact records the stable identity and safety semantics of the
// DNS normalized operation. Adapters must preserve this contract when mapping
// a Patch to a controller-specific write.
type OperationArtifact struct {
	Identity        string `json:"identity"`
	CatalogIdentity string `json:"catalog_identity"`
	Idempotency     string `json:"idempotency"`
	Retry           string `json:"retry"`
	Convergence     string `json:"convergence"`
	Concurrency     string `json:"concurrency"`
	Errors          string `json:"errors"`
}

var normalizedOperation = OperationArtifact{
	Identity:        "unifi.network.dns_record.operation.patch.v1",
	CatalogIdentity: "unifi.network.dns_record",
	Idempotency:     "Building against the same observed model and intent returns the same patch; rebuilding after the desired state is observed returns a no-op.",
	Retry:           "After any failed write, re-read the record and rebuild the patch before retrying; never retry through a fallback transport.",
	Convergence:     "Read, normalize, build, apply, and re-read until BuildPatch returns a no-op or a bounded caller policy stops.",
	Concurrency:     "Apply only to the same identity and expected revision; without a revision token, re-read immediately before writing and treat competing changes as a conflict.",
	Errors:          "Reject invalid identity or presence before writing; return transport and controller errors unchanged and do not fall back after a failed write.",
}

// NormalizedOperation returns the only admitted normalized operation in this
// package. It is DNS-only; transport adapters are intentionally outside this
// artifact. Each call returns a copy so callers cannot rewrite its identity.
func NormalizedOperation() OperationArtifact { return normalizedOperation }

// OperationDigest binds catalogs and scenario receipts to the complete
// normalized operation artifact.
func OperationDigest() string {
	document, err := json.Marshal(normalizedOperation)
	if err != nil {
		panic("encode static DNS normalized operation: " + err.Error())
	}
	sum := sha256.Sum256(document)
	return hex.EncodeToString(sum[:])
}

// Model is one normalized controller observation. Field presence is retained:
// omitted, explicitly cleared, and explicitly set values are not equivalent.
type Model struct {
	Identity string
	Revision string

	Enabled    Field[bool]
	Key        Field[string]
	Port       Field[int64]
	Priority   Field[int64]
	RecordType Field[string]
	TTL        Field[int64]
	Value      Field[string]
	Weight     Field[int64]
}
