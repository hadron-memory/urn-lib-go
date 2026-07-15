package urn

import "regexp"

// Legacy (pre-spec-021) URN format/parse/validate surface. Ported verbatim from
// hadron-server src/lib/urn.ts. New code should prefer the canonical parser (a
// later increment); these exist for callers that predate it.

// LegacyParsedURN is the result of ParseUrnInput. JSON tags match the corpus.
type LegacyParsedURN struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

var legacyURNRe = regexp.MustCompile(`^(?:hrn|urn):(org|memory|agent|app|node|edge|user):(.+)$`)
var locRe = regexp.MustCompile(`^loc:(.+)$`)

// ParseUrnInput strips the hrn:/urn:<type>: or loc: prefix and returns the type
// and bare value. Unprefixed inputs are returned as type "unknown".
func ParseUrnInput(input string) LegacyParsedURN {
	if m := legacyURNRe.FindStringSubmatch(input); m != nil {
		return LegacyParsedURN{Type: m[1], Value: m[2]}
	}
	if m := locRe.FindStringSubmatch(input); m != nil {
		return LegacyParsedURN{Type: "loc", Value: m[1]}
	}
	return LegacyParsedURN{Type: "unknown", Value: input}
}

// FormatUrn formats a bare value as a typed canonical URN string.
func FormatUrn(typ, value string) string {
	return CanonicalScheme + ":" + typ + ":" + value
}

// ValidateUrnType returns nil if the parsed type matches expected (with the
// unknown-always-ok and loc-where-node-expected exceptions), otherwise a pointer
// to the mismatch message. A *string (nil = ok) mirrors the JS `string | null`.
func ValidateUrnType(parsed LegacyParsedURN, expected string) *string {
	if parsed.Type == "unknown" {
		return nil
	}
	if parsed.Type == expected {
		return nil
	}
	if parsed.Type == "loc" && expected == "node" {
		return nil
	}
	msg := "Expected " + expected + " URN (hrn:" + expected + ":...), got hrn:" + parsed.Type + ":..."
	return &msg
}
