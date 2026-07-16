package urn

import (
	"regexp"
	"strings"
)

// Grammar v2 — the FLAT, pool-rooted forms (hadron-server#694, decision
// D-2026-07-15-001). ADDITIVE and coexisting with the v1 surface. See
// urn-lib-js src/v2.ts for the full contract.
//
// v2 shape:  hrn:<type>:<root>[:<segment>...][#<fragment>]  (single colon)
//
// Per-entity arity/container semantics (#696, decision D-2026-07-15-006):
//   secret   hrn:secret:<root>:<name>                 — org/user root + 1 name atom.
//   apprun   hrn:apprun:<root>:<app>:<run-id>         — fixed 2 segments.
//   noderev  hrn:noderev:<root>:<mem>:<loc...>:<rev>  — END-ANCHORED: last atom
//            is the revision id, first post-root atom is the memory, everything
//            between is the (variable-length, opaque) node loc.
//   node/edge  the loc is an OPAQUE terminal (never re-split into source:target).
//   node-data  <node-or-apprun-urn>#data              — #data fragment.

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

// v2Fragments — the fragment words valid on a v2 URN (v2 demotes node-data to a
// #data fragment of its parent node/apprun, #696).
var v2Fragments = map[string]bool{"data": true}

// v2FragmentParentTypes — the types a #data fragment may hang off.
var v2FragmentParentTypes = map[string]bool{"node": true, "apprun": true}

// ParsedURNV2 is the result of ParseUrnV2. JSON tags match the corpus.
type ParsedURNV2 struct {
	Scheme   string   `json:"scheme"`
	Type     string   `json:"type"`
	Root     string   `json:"root"`
	Segments []string `json:"segments"`
	// Fragment is present only when the URN carried a #<fragment> suffix.
	Fragment string `json:"fragment,omitempty"`
}

// ParsedNodeRevURNV2 is the END-ANCHORED decomposition of a hrn:noderev URN.
type ParsedNodeRevURNV2 struct {
	Scheme string `json:"scheme"`
	Type   string `json:"type"`
	Root   string `json:"root"`
	Memory string `json:"memory"`
	Loc    string `json:"loc"`
	Rev    string `json:"rev"`
}

var v2PrefixRe = regexp.MustCompile(`^(hrn|urn):([a-z][a-z0-9-]*):(.+)$`)

// validateV2Arity enforces the #696 per-type flat-segment arity. `segments` is
// everything AFTER the root atom. Only the fixed-shape types constrain arity;
// node/edge/mem/... keep the leaf-unbounded generic form.
func validateV2Arity(input, typ string, segments []string) error {
	switch typ {
	case "secret":
		if len(segments) != 1 {
			return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape}
		}
	case "apprun":
		if len(segments) != 2 {
			return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape}
		}
	case "noderev":
		if len(segments) < 3 {
			return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape}
		}
	case "node", "edge": // <root>:<mem>:<loc...> — a memory + at least one loc atom.
		if len(segments) < 2 {
			return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape}
		}
	}
	return nil
}

// ParseUrnV2 parses a STRICT grammar-v2 flat URN. Rejects v1 constructs (the ::
// hierarchy separator and the @/user: root sigil). Enforces the #696 per-entity
// arity and the #data fragment rules.
func ParseUrnV2(input string) (ParsedURNV2, error) {
	// Split off an optional trailing #<fragment> first (v2 node-data, #696).
	fragment := ""
	core := input
	if i := strings.IndexByte(input, '#'); i != -1 {
		fragment = input[i+1:]
		core = input[:i]
		if !v2Fragments[fragment] {
			return ParsedURNV2{}, &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: "#" + fragment}
		}
	}
	m := v2PrefixRe.FindStringSubmatch(core)
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
	if err := validateV2Arity(input, typ, segments); err != nil {
		return ParsedURNV2{}, err
	}
	if fragment != "" && !v2FragmentParentTypes[typ] {
		return ParsedURNV2{}, &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: "#" + fragment}
	}
	return ParsedURNV2{Scheme: "hrn", Type: typ, Root: atoms[0], Segments: segments, Fragment: fragment}, nil
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
	urn := CanonicalScheme + ":" + typ + ":" + strings.Join(all, ":")
	// Validate arity against the FULL composed URN so a thrown error's Input
	// reflects what the caller asked for (not a truncated prefix).
	if err := validateV2Arity(urn, typ, segments); err != nil {
		return "", err
	}
	return urn, nil
}

// IsFlatV2 reports whether input is already a canonical grammar-v2 flat URN.
func IsFlatV2(input string) bool {
	p, err := ParseUrnV2(input)
	if err != nil {
		return false
	}
	all := append([]string{p.Root}, p.Segments...)
	out := CanonicalScheme + ":" + p.Type + ":" + strings.Join(all, ":")
	if p.Fragment != "" {
		out += "#" + p.Fragment
	}
	return out == input
}

// ─── Per-entity typed helpers (#696) ────────────────────────────────────────

// ComposeSecretUrnV2 composes hrn:secret:<root>:<name> (org/user root + name).
func ComposeSecretUrnV2(root, name string) (string, error) {
	return ComposeUrnV2("secret", root, name)
}

// ComposeAppRunUrnV2 composes hrn:apprun:<root>:<app>:<run-id>.
func ComposeAppRunUrnV2(root, app, runID string) (string, error) {
	return ComposeUrnV2("apprun", root, app, runID)
}

// ComposeNodeRevUrnV2 composes hrn:noderev:<root>:<mem>:<loc...>:<rev>. `loc`
// may be a multi-atom node loc (colon-joined); it is split into its atoms so the
// terminal rev stays end-anchored.
func ComposeNodeRevUrnV2(root, mem, loc, rev string) (string, error) {
	locAtoms := strings.Split(loc, ":")
	segments := make([]string, 0, 2+len(locAtoms))
	segments = append(segments, mem)
	segments = append(segments, locAtoms...)
	segments = append(segments, rev)
	return ComposeUrnV2("noderev", root, segments...)
}

// ComposeDataFragmentV2 appends the #data fragment to a v2 node or apprun URN.
func ComposeDataFragmentV2(parentURN string) (string, error) {
	p, err := ParseUrnV2(parentURN)
	if err != nil {
		return "", err
	}
	if !v2FragmentParentTypes[p.Type] || p.Fragment != "" {
		return "", &ParseError{Input: parentURN, Reason: ReasonInvalidSegmentShape, OffendingSegment: parentURN}
	}
	// Recompose from the parsed pieces so the result is always canonical hrn: —
	// a legacy urn:-scheme parent must not leak its scheme into the emission.
	all := append([]string{p.Root}, p.Segments...)
	return CanonicalScheme + ":" + p.Type + ":" + strings.Join(all, ":") + "#data", nil
}

// ParseNodeRevUrnV2 parses and END-ANCHORED-decomposes a hrn:noderev URN (#696).
// The LAST atom is the revision id; the FIRST post-root atom is the memory slug;
// everything between is the variable-length, opaque node loc.
func ParseNodeRevUrnV2(input string) (ParsedNodeRevURNV2, error) {
	p, err := ParseUrnV2(input)
	if err != nil {
		return ParsedNodeRevURNV2{}, err
	}
	if p.Type != "noderev" {
		return ParsedNodeRevURNV2{}, &ParseError{Input: input, Reason: ReasonInvalidSegmentShape}
	}
	memory := p.Segments[0]
	rev := p.Segments[len(p.Segments)-1]
	loc := strings.Join(p.Segments[1:len(p.Segments)-1], ":")
	return ParsedNodeRevURNV2{Scheme: "hrn", Type: "noderev", Root: p.Root, Memory: memory, Loc: loc, Rev: rev}, nil
}
