package urn

import (
	"regexp"
	"strings"
)

// Display-chip parser/registry (spec-010). Ported from urn-lib-js src/display.ts
// (originally hadron-portal's parse-urn.ts). DELIBERATELY separate from the
// canonical parser: tolerant for rendering, splits a displayable URN into
// {type, bareValue, fullUrn}, accepts legacy urn: input, always emits canonical
// hrn:, and falls back to "unknown" for a missing/unregistered visible kind so
// UI-registration gaps stay visible (hadron-portal#393).

// DisplayURNTypes — the visible URN kinds rendered as a typed chip. Narrower
// than the canonical registry on purpose: a new visible kind must be added here
// too, or ParseDisplayUrn renders it as hrn:unknown:...
var DisplayURNTypes = []string{"org", "memory", "agent", "app", "node", "user", "apprun"}

// ParsedDisplayURN is the result of ParseDisplayUrn. JSON tags match the corpus.
type ParsedDisplayURN struct {
	Type      string `json:"type"`
	BareValue string `json:"bareValue"`
	FullURN   string `json:"fullUrn"`
}

var displayURNRe = regexp.MustCompile(`^(?:hrn|urn):(` + strings.Join(DisplayURNTypes, "|") + `):(.+)$`)

// ParseDisplayUrn parses a displayable URN for chip rendering. `typeHint` is ""
// for none. Auto-detects a scheme-prefixed display kind; else applies the hint
// to a BARE value only; else falls back to "unknown".
func ParseDisplayUrn(value, typeHint string) ParsedDisplayURN {
	if m := displayURNRe.FindStringSubmatch(value); m != nil {
		t, bare := m[1], m[2]
		return ParsedDisplayURN{Type: t, BareValue: bare, FullURN: CanonicalScheme + ":" + t + ":" + bare}
	}
	if typeHint != "" && !HasSchemePrefix(value) {
		return ParsedDisplayURN{Type: typeHint, BareValue: value, FullURN: CanonicalScheme + ":" + typeHint + ":" + value}
	}
	return ParsedDisplayURN{Type: "unknown", BareValue: value, FullURN: CanonicalScheme + ":unknown:" + value}
}
