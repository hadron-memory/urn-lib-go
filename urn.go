// Package urn is the Go implementation of the Hadron URN library, paired with
// @hadron-memory/urn-lib-js. Both run the SAME conformance corpus
// (fixtures/corpus.json) — the corpus is the contract, so the two cannot drift.
//
// This is increment 1: the pure "scheme + registry + normalize + slug" slice,
// ported verbatim (v1 parity) from hadron-server/src/lib/urn.ts. parse/format/
// compose and the grammar-v2 flat forms land in later increments.
package urn

import (
	"regexp"
	"strings"
)

// Scheme prefixes (issue #239). hrn: is canonical; legacy urn: is accepted on
// input forever. Every emission path composes hrn:.
const (
	CanonicalScheme = "hrn"
	LegacyScheme    = "urn"
)

// MaxAtomLen is the maximum length of a single URN atom / slug, in bytes (spec
// cor:urn:010:01, FR-017). Exported as the single source of truth so lib
// internals and server-side minters stop re-typing the bare literal 64 (#715).
const MaxAtomLen = 64

var schemePrefixRe = regexp.MustCompile(`^(?:hrn|urn):`)

// HasSchemePrefix reports whether input leads with a scheme prefix.
func HasSchemePrefix(input string) bool {
	return schemePrefixRe.MatchString(input)
}

// NormalizeScheme rewrites a leading legacy urn: scheme to canonical hrn:.
// Bare and already-canonical inputs pass through.
func NormalizeScheme(input string) string {
	if strings.HasPrefix(input, "urn:") {
		return "hrn:" + input[len("urn:"):]
	}
	return input
}

// NormalizeUrnForLookup collapses the canonical :: hierarchy form to the bare
// single-colon lookup form.
func NormalizeUrnForLookup(bareURN string) string {
	if strings.Contains(bareURN, "::") {
		return strings.ReplaceAll(bareURN, "::", ":")
	}
	return bareURN
}

// LegacyMemoryUrnToCanonical converts a bare legacy single-colon memory URN to
// canonical :: hierarchy, preserving the marker:id boundary for role-marker
// memories. Input MUST be the bare legacy form.
func LegacyMemoryUrnToCanonical(bareLegacyURN string) string {
	parts := strings.Split(bareLegacyURN, ":")
	markerIdx := -1
	for i := len(parts) - 1; i >= 0; i-- {
		if roleMarkerSet[parts[i]] {
			markerIdx = i
			break
		}
	}
	if markerIdx < 0 {
		return strings.Join(parts, "::")
	}
	beforeMarker := strings.Join(parts[:markerIdx], "::")
	marker := parts[markerIdx]
	afterMarker := strings.Join(parts[markerIdx+1:], ":")
	if afterMarker != "" {
		return beforeMarker + "::" + marker + ":" + afterMarker
	}
	return beforeMarker + "::" + marker
}

// AgentSlugFromUrn extracts an agent's slug path, stripping the leading org
// segment. Robust to both legacy single-colon and canonical double-colon forms.
func AgentSlugFromUrn(agentURN string) string {
	parts := strings.Split(NormalizeUrnForLookup(agentURN), ":")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], ":")
}

var atomRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)

// ValidateAtomShape checks the FR-016 charset + FR-017 length only. Deliberately
// case-lenient (the read/parse path); the lowercase rule is enforced at create
// in ValidateUserSlug.
func ValidateAtomShape(input, atom string) error {
	if len(atom) == 0 {
		return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: atom}
	}
	if len(atom) > MaxAtomLen {
		return &ParseError{Input: input, Reason: ReasonSlugTooLong, OffendingSegment: atom}
	}
	if !atomRe.MatchString(atom) {
		return &ParseError{Input: input, Reason: ReasonInvalidCharset, OffendingSegment: atom}
	}
	return nil
}

// ValidateUserSlug validates a slug at create/rename: charset + length +
// reserved-word rejection + the lowercase-canonical rule (#575).
func ValidateUserSlug(slug string) error {
	if err := ValidateAtomShape(slug, slug); err != nil {
		return err
	}
	if reservedSlugSet[strings.ToLower(slug)] {
		return &ParseError{Input: slug, Reason: ReasonReservedWordSlug, OffendingSegment: slug}
	}
	if strings.ToLower(slug) != slug {
		return &ParseError{Input: slug, Reason: ReasonSlugNotLowercase, OffendingSegment: slug}
	}
	return nil
}

// ValidateOrgSlug validates an org slug: it must be bare (no scheme prefix, no
// colon), reuse the shared slug rules, and — for a NEW org root (#692) — be a
// dotted domain. Create/rename rule only; the parse/read path stays lenient.
func ValidateOrgSlug(slug string) error {
	if HasSchemePrefix(slug) || strings.Contains(slug, ":") {
		return &ParseError{Input: slug, Reason: ReasonOrgUrnNotBare, OffendingSegment: slug}
	}
	if err := ValidateUserSlug(slug); err != nil {
		return err
	}
	if !strings.Contains(slug, ".") {
		return &ParseError{Input: slug, Reason: ReasonOrgRootNotDotted, OffendingSegment: slug}
	}
	return nil
}

// ValidateUserHandle validates a user handle at create/rename (#692): a
// ValidateUserSlug slug that must additionally be dot-free, so it stays
// charset-disjoint from a dotted org root in the shared principal pool.
// Registration policy, not a parse rule.
func ValidateUserHandle(handle string) error {
	if err := ValidateUserSlug(handle); err != nil {
		return err
	}
	if strings.Contains(handle, ".") {
		return &ParseError{Input: handle, Reason: ReasonHandleHasDot, OffendingSegment: handle}
	}
	return nil
}

var nonAtomRe = regexp.MustCompile(`[^a-z0-9._-]+`)

// DeriveSlugFromName derives a valid, lowercase slug atom from a free-form name.
// Slugification only — it does NOT reject reserved words. Returns ReasonEmptyDerivedSlug
// when nothing usable survives.
func DeriveSlugFromName(name string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = nonAtomRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "._-")
	if len(slug) > MaxAtomLen {
		slug = strings.TrimRight(slug[:MaxAtomLen], "._-")
	}
	if slug == "" {
		return "", &ParseError{Input: name, Reason: ReasonEmptyDerivedSlug, OffendingSegment: name}
	}
	return slug, nil
}

// IsValidSlug is the boolean form of ValidateUserSlug: true when slug is a legal
// NEW entity slug (charset + length + reserved-word + lowercase), false on any
// violation. Convenience predicate for callers that don't want an error (#715).
func IsValidSlug(slug string) bool {
	return ValidateUserSlug(slug) == nil
}
