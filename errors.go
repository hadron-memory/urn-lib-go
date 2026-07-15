package urn

import "fmt"

// Reason is a machine-stable failure code. These ARE part of the fixture
// contract (shared with urn-lib-js); the human message text is not.
type Reason string

const (
	ReasonUnknownType          Reason = "unknown-type"
	ReasonMalformedGrammar     Reason = "malformed-grammar"
	ReasonEmptySegment         Reason = "empty-segment"
	ReasonTrailingDoubleColon  Reason = "trailing-double-colon"
	ReasonReservedWordSlug     Reason = "reserved-word-slug"
	ReasonInvalidCharset       Reason = "invalid-charset"
	ReasonSlugNotLowercase     Reason = "slug-not-lowercase"
	ReasonSlugTooLong          Reason = "slug-too-long"
	ReasonLocSegmentRejected   Reason = "loc-segment-rejected"
	ReasonInvalidSegmentShape  Reason = "invalid-segment-shape"
	ReasonEmptyBareValue       Reason = "empty-bare-value"
	ReasonAlreadyPrefixedValue Reason = "already-prefixed-bare-value"
	ReasonOrgUrnNotBare        Reason = "org-urn-not-bare"
	ReasonEmptyDerivedSlug     Reason = "empty-derived-slug"
)

// ParseError is returned by every validation/parse entry point on any
// acceptance violation.
type ParseError struct {
	Input            string
	Reason           Reason
	OffendingSegment string
}

func (e *ParseError) Error() string {
	at := ""
	if e.OffendingSegment != "" && e.OffendingSegment != e.Input {
		at = fmt.Sprintf(" at %q", e.OffendingSegment)
	}
	return fmt.Sprintf("URN parse error [%s]%s: %q", e.Reason, at, e.Input)
}
