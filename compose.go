package urn

import (
	"fmt"
	"strings"
)

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

func stripPrefixOrThrow(u, expectedType string) (string, error) {
	if u == "" {
		return "", fmt.Errorf("composeInstalledAgentUrn: %s URN is empty", expectedType)
	}
	normalized := NormalizeScheme(u)
	pfx := CanonicalScheme + ":" + expectedType + ":"
	if strings.HasPrefix(normalized, pfx) {
		return normalized[len(pfx):], nil
	}
	if HasSchemePrefix(u) {
		return "", fmt.Errorf("composeInstalledAgentUrn: expected hrn:%s: prefix; got %q", expectedType, u)
	}
	return u, nil
}

func splitMixedGrammar(path string) []string {
	if strings.Contains(path, "::") {
		return strings.Split(path, "::")
	}
	return strings.Split(path, ":")
}

// ComposeInstalledAgentUrn composes the R2 canonical install URN for an Agent
// installed in an App: hrn:agent:<installing-org>::<app-slug>::<author-org>:<agent-slug>.
// Inputs MUST be 2-segment (org + slug) URNs. Install-by-self collapses via cat 1.
func ComposeInstalledAgentUrn(appURN, agentURN string) (string, error) {
	appPath, err := stripPrefixOrThrow(appURN, "app")
	if err != nil {
		return "", err
	}
	agentPath, err := stripPrefixOrThrow(agentURN, "agent")
	if err != nil {
		return "", err
	}
	appSegments := splitMixedGrammar(appPath)
	agentSegments := splitMixedGrammar(agentPath)
	if len(appSegments) != 2 {
		return "", fmt.Errorf("composeInstalledAgentUrn: appUrn must have exactly <org>::<slug> shape (2 segments); got %q (%d segments)", appURN, len(appSegments))
	}
	if len(agentSegments) != 2 {
		return "", fmt.Errorf("composeInstalledAgentUrn: agentUrn must have exactly <author-org>::<slug> shape (2 segments); got %q (%d segments)", agentURN, len(agentSegments))
	}
	raw := CanonicalScheme + ":agent:" + appSegments[0] + "::" + appSegments[1] + "::" + agentSegments[0] + ":" + agentSegments[1]
	return ToParserCanonical(raw)
}
