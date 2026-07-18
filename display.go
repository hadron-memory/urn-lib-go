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

// `mem` is added as the grammar-v2 memory type word (#697 emission flip): a
// server-emitted hrn:mem:<root>:<slug> renders under the `memory` display badge
// (mapped below) instead of falling through to "unknown".
var displayURNRe = regexp.MustCompile(`^(?:hrn|urn):(` + strings.Join(append(append([]string{}, DisplayURNTypes...), "mem"), "|") + `):(.+)$`)

// ParseDisplayUrn parses a displayable URN for chip rendering. `typeHint` is ""
// for none. Auto-detects a scheme-prefixed display kind; else applies the hint
// to a BARE value only; else falls back to "unknown".
func ParseDisplayUrn(value, typeHint string) ParsedDisplayURN {
	if m := displayURNRe.FindStringSubmatch(value); m != nil {
		rawType, bare := m[1], m[2]
		// Grammar-v2 memory (`mem`) renders under the `memory` badge, but the
		// reconstructed FullURN KEEPS the `mem` type word — rewriting it to
		// hrn:memory: would pair a v1 type word with the v2 single-colon body and
		// produce a malformed, non-round-tripping URN. So Type (the badge) and the
		// FullURN type word may diverge for a v2 memory URN, by design.
		t := rawType
		if rawType == "mem" {
			t = "memory"
		}
		return ParsedDisplayURN{Type: t, BareValue: bare, FullURN: CanonicalScheme + ":" + rawType + ":" + bare}
	}
	if typeHint != "" && !HasSchemePrefix(value) {
		return ParsedDisplayURN{Type: typeHint, BareValue: value, FullURN: CanonicalScheme + ":" + typeHint + ":" + value}
	}
	return ParsedDisplayURN{Type: "unknown", BareValue: value, FullURN: CanonicalScheme + ":unknown:" + value}
}
