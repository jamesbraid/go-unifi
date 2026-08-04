package campaign

import (
	"encoding/json"
	"fmt"

	"github.com/ubiquiti-community/go-unifi/control/dnsrecord"
)

const (
	trustedDNSAdmissionReceiptID       = "unifi.network.dns_record.admission.1"
	trustedDNSAdmissionReceiptChecksum = "e8622a993b2c131b9531e78fe2c23a5f28a1e65ef33ef13168b460bad998e06d"
	trustedDNSAdmissionSourceCommit    = "4d2b7eba2474eaf05bb17275b92839c78e6c27e8"
	trustedDNSAdmissionArtifact        = "catalogs/network-10.4.57/dns_record.admitted-catalog.json"
)

type admissionReceipt struct {
	FormatVersion         int                 `json:"format_version"`
	ReceiptID             string              `json:"receipt_id"`
	BaselineCatalogSHA256 string              `json:"baseline_catalog_sha256"`
	CatalogIdentity       string              `json:"catalog_identity"`
	OperationIdentity     string              `json:"operation_identity"`
	OperationDigest       string              `json:"operation_digest"`
	AdmissionRevision     int                 `json:"admission_revision"`
	Decision              string              `json:"decision"`
	Provenance            admissionProvenance `json:"provenance"`
	ChecksumSHA256        string              `json:"checksum_sha256"`
}

type admissionProvenance struct {
	Kind         string `json:"kind"`
	SourceCommit string `json:"source_commit"`
	Artifact     string `json:"artifact"`
}

type admissionReceiptPayload struct {
	FormatVersion         int                 `json:"format_version"`
	ReceiptID             string              `json:"receipt_id"`
	BaselineCatalogSHA256 string              `json:"baseline_catalog_sha256"`
	CatalogIdentity       string              `json:"catalog_identity"`
	OperationIdentity     string              `json:"operation_identity"`
	OperationDigest       string              `json:"operation_digest"`
	AdmissionRevision     int                 `json:"admission_revision"`
	Decision              string              `json:"decision"`
	Provenance            admissionProvenance `json:"provenance"`
}

func validateAdmissionReceipt(baselineCanonical []byte, baseline catalogDocument, document []byte) error {
	var receipt admissionReceipt
	if _, err := decodeCanonical(document, &receipt); err != nil {
		return fmt.Errorf("decode admission receipt: %w", err)
	}
	checksum, err := admissionReceiptChecksum(receipt)
	if err != nil {
		return err
	}
	if receipt.ChecksumSHA256 != checksum {
		return fmt.Errorf("admission receipt checksum does not match payload")
	}
	if receipt.ReceiptID != trustedDNSAdmissionReceiptID || receipt.ChecksumSHA256 != trustedDNSAdmissionReceiptChecksum {
		return fmt.Errorf("admission receipt is not the trusted admission")
	}
	operation := dnsrecord.NormalizedOperation()
	if receipt.FormatVersion != 1 || receipt.AdmissionRevision != 1 || receipt.Decision == "" ||
		receipt.Provenance.Kind != "operator_review" || receipt.Provenance.SourceCommit != trustedDNSAdmissionSourceCommit ||
		receipt.Provenance.Artifact != trustedDNSAdmissionArtifact {
		return fmt.Errorf("trusted admission receipt is incomplete")
	}
	if receipt.BaselineCatalogSHA256 != digest(baselineCanonical) {
		return fmt.Errorf("admission receipt baseline digest does not match catalog")
	}
	if receipt.CatalogIdentity != baseline.CatalogID || receipt.OperationIdentity != operation.Identity ||
		receipt.OperationDigest != dnsrecord.OperationDigest() || receipt.OperationDigest != baseline.Admission.OperationDigest {
		return fmt.Errorf("admission receipt catalog or operation identity does not match baseline")
	}
	return nil
}

func admissionReceiptChecksum(receipt admissionReceipt) (string, error) {
	payload := admissionReceiptPayload{
		FormatVersion:         receipt.FormatVersion,
		ReceiptID:             receipt.ReceiptID,
		BaselineCatalogSHA256: receipt.BaselineCatalogSHA256,
		CatalogIdentity:       receipt.CatalogIdentity,
		OperationIdentity:     receipt.OperationIdentity,
		OperationDigest:       receipt.OperationDigest,
		AdmissionRevision:     receipt.AdmissionRevision,
		Decision:              receipt.Decision,
		Provenance:            receipt.Provenance,
	}
	document, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode admission receipt payload: %w", err)
	}
	return digest(document), nil
}
