package main

import (
	"errors"
	"fmt"
)

const maxOCIControlJSONBytes int64 = 8 << 20

type ArchiveLimits struct {
	MaxImageTarBytes  int64
	MaxBlobBytes      int64
	MaxLayerBytes     int64
	MaxJarBytes       int64
	MaxJSONBytes      int64
	MaxNoticeBytes    int64
	MaxEntries        int
	MaxNoticeEntries  int
	MaxNestedArchives int
	MaxIndexDepth     int
}

func DefaultArchiveLimits() ArchiveLimits {
	return ArchiveLimits{
		MaxImageTarBytes:  2 << 30,
		MaxBlobBytes:      2 << 30,
		MaxLayerBytes:     4 << 30,
		MaxJarBytes:       512 << 20,
		MaxJSONBytes:      32 << 20,
		MaxNoticeBytes:    256 << 20,
		MaxEntries:        500_000,
		MaxNoticeEntries:  10_000,
		MaxNestedArchives: 1_024,
		MaxIndexDepth:     8,
	}
}

func (l ArchiveLimits) validate() error {
	if l.MaxImageTarBytes <= 0 || l.MaxBlobBytes <= 0 || l.MaxLayerBytes <= 0 || l.MaxJarBytes <= 0 || l.MaxJSONBytes <= 0 || l.MaxNoticeBytes <= 0 || l.MaxEntries <= 0 || l.MaxNoticeEntries <= 0 || l.MaxNestedArchives <= 0 || l.MaxIndexDepth <= 0 {
		return errors.New("all archive limits must be positive")
	}
	return nil
}

func limitPlusOne(limit int64) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("limit must be positive")
	}
	if limit == int64(^uint64(0)>>1) {
		return 0, fmt.Errorf("limit %d is too large", limit)
	}
	return limit + 1, nil
}
