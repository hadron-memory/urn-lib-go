package urn

import "strings"

// Canonical URN composition (specs 027 / 037). Ported verbatim from
// hadron-server src/lib/urn.ts. Emits canonical hrn: form.

// FormatCanonicalUrn composes and canonicalizes a full URN from an entity type
// and a bare-stored value (legacy single-colon or canonical :: hierarchy).
// Always returns the canonical hrn:<type>: form.
func FormatCanonicalUrn(typ, bareValue string) (string, error) {
	if strings.TrimSpace(bareValue) == "" {
		return "", &ParseError{Input: "", Reason: ReasonEmptyBareValue}
	}
	if HasSchemePrefix(bareValue) {
		return "", &ParseError{Input: bareValue, Reason: ReasonAlreadyPrefixedValue, OffendingSegment: bareValue}
	}
	// Promote legacy single-colon hierarchy to canonical :: only when NO ::
	// is present (once any canonical separator exists, remaining single colons
	// are intentional — e.g. the R2 author segment).
	normalized := bareValue
	if !strings.Contains(bareValue, "::") {
		normalized = strings.ReplaceAll(bareValue, ":", "::")
	}
	return ToParserCanonical(CanonicalScheme + ":" + typ + ":" + normalized)
}

func composeMemoryScopedUrn(typ, memURN, loc string) (string, error) {
	memCanonical, err := FormatCanonicalUrn("memory", memURN)
	if err != nil {
		return "", err
	}
	bareMemCanonical := memCanonical[len(CanonicalScheme+":memory:"):]
	return CanonicalScheme + ":" + typ + ":" + bareMemCanonical + "::" + loc, nil
}

// ComposeNodeUrn builds a canonical hrn:node:<mem>::<loc> URN.
func ComposeNodeUrn(memURN, loc string) (string, error) {
	return composeMemoryScopedUrn("node", memURN, loc)
}

// ComposeEdgeUrn builds a canonical hrn:edge:<mem>::<loc> URN.
func ComposeEdgeUrn(memURN, loc string) (string, error) {
	return composeMemoryScopedUrn("edge", memURN, loc)
}
