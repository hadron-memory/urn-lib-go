package urn

import (
	"regexp"
	"strings"
)

// Grammar v2 — the FLAT, pool-rooted forms (hadron-server#694, decision
// D-2026-07-15-001). ADDITIVE and coexisting with the v1 surface. See
// urn-lib-js src/v2.ts for the full contract.
//
// v2 shape:  hrn:<type>:<root>[:<segment>...]  (single colon, NO sigil)

// V2URNTypes — the v2 type registry (SEEDED; finalized in #698). Note the
// renames vs v1 (memory->mem) and additions (apprun, noderev, appkey, ...);
// `data` is demoted to a #fragment and is NOT a type.
var V2URNTypes = []string{
	"org", "user", "mem", "agent", "app", "node", "edge", "asset", "secret",
	"apprun", "noderev", "appkey", "aiconf", "tool", "server", "userapikey",
	"agentschedule", "agentwebhook", "license", "subscription", "usage",
	"reference", "session", "platform",
}

var v2TypeSet = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range V2URNTypes {
		m[t] = true
	}
	return m
}()

// ParsedURNV2 is the result of ParseUrnV2. JSON tags match the corpus.
type ParsedURNV2 struct {
	Scheme   string   `json:"scheme"`
	Type     string   `json:"type"`
	Root     string   `json:"root"`
	Segments []string `json:"segments"`
}

var v2PrefixRe = regexp.MustCompile(`^(hrn|urn):([a-z][a-z0-9-]*):(.+)$`)

// ParseUrnV2 parses a STRICT grammar-v2 flat URN. Rejects v1 constructs (the ::
// hierarchy separator and the @/user: root sigil).
func ParseUrnV2(input string) (ParsedURNV2, error) {
	m := v2PrefixRe.FindStringSubmatch(input)
	if m == nil {
		return ParsedURNV2{}, &ParseError{Input: input, Reason: ReasonMalformedGrammar}
	}
	typ, rest := m[2], m[3]
	if !v2TypeSet[typ] {
		return ParsedURNV2{}, &ParseError{Input: input, Reason: ReasonUnknownType}
	}
	if strings.Contains(rest, "::") {
		return ParsedURNV2{}, &ParseError{Input: input, Reason: ReasonMalformedGrammar}
	}
	atoms := strings.Split(rest, ":")
	for _, atom := range atoms {
		if err := ValidateAtomShape(input, atom); err != nil {
			return ParsedURNV2{}, err
		}
	}
	segments := []string{}
	if len(atoms) > 1 {
		segments = atoms[1:]
	}
	return ParsedURNV2{Scheme: "hrn", Type: typ, Root: atoms[0], Segments: segments}, nil
}

// ComposeUrnV2 composes a canonical grammar-v2 flat URN.
func ComposeUrnV2(typ, root string, segments ...string) (string, error) {
	if !v2TypeSet[typ] {
		return "", &ParseError{Input: typ, Reason: ReasonUnknownType, OffendingSegment: typ}
	}
	if err := ValidateAtomShape(root, root); err != nil {
		return "", err
	}
	for _, s := range segments {
		if err := ValidateAtomShape(s, s); err != nil {
			return "", err
		}
	}
	all := append([]string{root}, segments...)
	return CanonicalScheme + ":" + typ + ":" + strings.Join(all, ":"), nil
}

// IsFlatV2 reports whether input is already a canonical grammar-v2 flat URN.
func IsFlatV2(input string) bool {
	p, err := ParseUrnV2(input)
	if err != nil {
		return false
	}
	all := append([]string{p.Root}, p.Segments...)
	return CanonicalScheme+":"+p.Type+":"+strings.Join(all, ":") == input
}
