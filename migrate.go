package urn

import "time"

// Per-row migration gate (spec 021 FR-032). Ported verbatim from hadron-server
// src/lib/urn.ts. NOTE: this is the online-migration dispatch keyed on a DB
// row's URNNormalizedAt — transient by design and arguably server-specific;
// kept here for surface completeness.

// UrnRow is the minimal row shape for ParseFor. A nil URNNormalizedAt means the
// row is pre-normalization (legacy shape).
type UrnRow struct {
	URN             string
	URNNormalizedAt *time.Time
}

// ParseForResult is the ParseFor envelope. JSON tags match the corpus.
type ParseForResult struct {
	Canonical string `json:"canonical"`
	IsLegacy  bool   `json:"isLegacy"`
}

// ParseFor dispatches per-row: a normalized row is parsed canonically and its
// stored URN returned unchanged (drift throws; scheme differences tolerated); a
// legacy row is validated with the legacy parser (unknown shape throws).
func ParseFor(row UrnRow) (ParseForResult, error) {
	if row.URNNormalizedAt != nil {
		parsed, err := ParseUrn(row.URN)
		if err != nil {
			return ParseForResult{}, err
		}
		if parsed.ParserCanonical != NormalizeScheme(row.URN) {
			return ParseForResult{}, &ParseError{Input: row.URN, Reason: ReasonMalformedGrammar}
		}
		return ParseForResult{Canonical: row.URN, IsLegacy: false}, nil
	}
	legacy := ParseUrnInput(row.URN)
	if legacy.Type == "unknown" {
		return ParseForResult{}, &ParseError{Input: row.URN, Reason: ReasonMalformedGrammar}
	}
	return ParseForResult{Canonical: row.URN, IsLegacy: true}, nil
}
