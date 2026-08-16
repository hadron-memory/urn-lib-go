package urn

import (
	"fmt"
	"regexp"
	"strings"
)

// API-boundary URN qualification (spec 022). Ported verbatim from hadron-server
// src/lib/urn.ts. AssertFullyQualifiedUrn / SplitNodeUrn return a
// *NotQualifiedError (distinct from *ParseError) — the boundary gate.

var minHierarchySegments = map[string]int{
	"org": 1, "memory": 2, "agent": 2, "app": 2, "node": 3, "edge": 3, "user": 1,
	// secret (#679): owner-dependent depth; the gate checks a minimum, so 2
	// admits both org/user-owned (2) and app/memory-owned (3).
	"secret": 2,
}

var minSegmentsHint = map[string]string{
	"org":    `1 hierarchy segment (e.g., "acme.com")`,
	"memory": `2 hierarchy segments (org::memory, e.g., "acme.com::mmdata")`,
	"agent":  `2 hierarchy segments (org::agent-slug, e.g., "acme.com::coding-agent")`,
	"app":    `2 hierarchy segments (org::app-slug, e.g., "acme.com::dev-app")`,
	"node":   `3 hierarchy segments (org::memory::loc, e.g., "acme.com::mmdata::review:sort-imports")`,
	"edge":   `3 hierarchy segments (org::memory::loc, e.g., "acme.com::mmdata::intro:next")`,
	"user":   `1 hierarchy segment (the handle, e.g., "holger")`,
	"secret": `2+ hierarchy segments (owner root :: [app|memory:slug ::] name, e.g., "acme.com::stripe-key" or "acme.com::app:internal-ops::stripe-key")`,
}

var nodeRoleAliases = map[string]bool{
	"abstract": true, "partial": true, "parent": true, "plan": true, "prompt": true,
	"record": true, "task": true, "review": true, "chat": true, "chat-message": true,
	"config": true, "conversation": true, "event": true, "goal": true, "stage": true,
	"condition": true, "data": true,
}

// NotQualifiedError is returned when a non-ID-shaped input fails URN
// qualification. Code() returns the stable cross-language contract handle.
type NotQualifiedError struct {
	OffendingValue string
	ExpectedType   string
}

// Code is the stable contract handle (mirrors urn-lib-js UrnNotQualifiedError.code).
func (e *NotQualifiedError) Code() string { return "URN_NOT_QUALIFIED" }

func (e *NotQualifiedError) Error() string {
	fixHint := `Use the canonical form "<org>::<memory>[::path]" — org and memory slugs are mandatory at the API boundary.`
	if e.ExpectedType != "" {
		fixHint = fmt.Sprintf("Expected a %s URN with at least %s.", e.ExpectedType, minSegmentsHint[e.ExpectedType])
	}
	return fmt.Sprintf("URN %q is not fully qualified. %s", e.OffendingValue, fixHint)
}

var qualPrefixRe = regexp.MustCompile(`^(?:hrn|urn):([a-z][a-z0-9-]*):(.+)$`)
var qualPrefixStripRe = regexp.MustCompile(`^(?:hrn|urn):[a-z][a-z0-9-]*:(.+)$`)
var tripleColonRe = regexp.MustCompile(`:{3,}`)

// AssertFullyQualifiedUrn rejects inputs that lack the fully-qualified shape for
// expectedType. Checks SHAPE, not full canonical grammar.
func AssertFullyQualifiedUrn(input, expectedType string) error {
	path := input
	prefixType := ""
	if m := qualPrefixRe.FindStringSubmatch(input); m != nil {
		prefixType = m[1]
		path = m[2]
	} else if HasSchemePrefix(input) {
		return &NotQualifiedError{input, expectedType}
	}

	if prefixType == "loc" {
		return &NotQualifiedError{input, expectedType}
	}

	if prefixType != "" && prefixType != expectedType {
		isNodeRoleAlias := expectedType == "node" && nodeRoleAliases[prefixType]
		// The grammar-v2 `mem` type word aliases `memory` at the boundary (#697
		// emission flip), so a v2-emitted hrn:mem:root:slug qualifies as a memory.
		isMemoryAlias := expectedType == "memory" && prefixType == "mem"
		if !isNodeRoleAlias && !isMemoryAlias {
			return &NotQualifiedError{input, expectedType}
		}
	}

	if tripleColonRe.MatchString(path) {
		return &NotQualifiedError{input, expectedType}
	}

	var segments []string
	if strings.Contains(path, "::") {
		segments = strings.Split(path, "::")
	} else {
		segments = strings.Split(path, ":")
	}
	for _, s := range segments {
		if s == "" {
			return &NotQualifiedError{input, expectedType}
		}
	}
	if len(segments) < minHierarchySegments[expectedType] {
		return &NotQualifiedError{input, expectedType}
	}
	return nil
}

// NodeLikeURNParts is the result of SplitNodeUrn / SplitEdgeUrn (node and edge
// share one shape). JSON tags match the corpus.
//
// Loc is GRAMMAR-NORMALIZED: both hrn:node:acme.com::specs::cor:urn and
// hrn:node:acme.com:specs:cor:urn yield "cor:urn", so a caller never has to
// know which grammar it was handed. That is the reason to prefer these
// decomposers over ParsedURN.PathSegments, which is a RAW split whose shape
// follows the input grammar (urn-lib-js#12).
type NodeLikeURNParts struct {
	// MemoryURN is the bare <org>:<memorySlug>, or the full path for a
	// multi-segment memory (<org>:<app>:<agent>:<role>).
	MemoryURN string `json:"memoryUrn"`
	Loc       string `json:"loc"`
	// Fragment is set only when the input carried a #<fragment> suffix (v2
	// #data). Omitted otherwise so existing corpus cases keep matching.
	Fragment string `json:"fragment,omitempty"`
}

// splitNodeLikeUrn is shared by SplitNodeUrn and SplitEdgeUrn — node and edge
// have the same <root>::<mem>::<loc...> shape, and an edge loc is an opaque
// terminal (never re-split into source:target).
func splitNodeLikeUrn(input, expectedType string) (NodeLikeURNParts, error) {
	// Strip an optional trailing #<fragment> FIRST (urn-lib-js#13). v2 spells
	// node-data as a #data fragment of its parent, and the decomposition below
	// is purely positional — left in place the fragment would ride along into
	// the terminal atom and produce Loc "cor:urn#data", which is not a loc: no
	// memory contains it, # is outside the atom charset, and a caller using it
	// as a lookup key gets a silent miss. The v1 spelling of the same resource
	// (hrn:data:<root>::<mem>::<loc>) yields a clean loc, so folding the
	// fragment in would make one resource decompose two different ways.
	fragment := ""
	urn := input
	if i := strings.Index(input, "#"); i != -1 {
		fragment = input[i+1:]
		urn = input[:i]
		// Validate the fragment against the SAME rules ParseUrnV2 enforces, before
		// stripping it. Otherwise these decomposers — self-validating by contract —
		// would be more permissive than ParseUrn for the same input, silently
		// accepting #bogus (not a registered fragment word) or a fragment on an
		// edge (only node/apprun may parent one) and handing back a clean loc.
		if !v2Fragments[fragment] || !v2FragmentParentTypes[expectedType] {
			return NodeLikeURNParts{}, &NotQualifiedError{OffendingValue: input, ExpectedType: expectedType}
		}
	}

	if err := AssertFullyQualifiedUrn(urn, expectedType); err != nil {
		return NodeLikeURNParts{}, err
	}
	path := urn
	if m := qualPrefixStripRe.FindStringSubmatch(urn); m != nil {
		path = m[1]
	}

	// v1 hierarchy (`::`) vs flat v2 (single `:`) — and the two grammars put the
	// memory/loc boundary in DIFFERENT places, so they can't share one rule:
	//
	//   v1: the TERMINAL `::` segment is the opaque loc and everything before it
	//       is the memory path, which may be multi-segment
	//       (<org>::<app>::<agent>::<role>::<loc> — in the parser only the final
	//       segment is unbounded). Slicing a fixed two segments here silently
	//       moved most of a multi-segment memory into the loc.
	//   v2: hrn:<type>:<root>:<mem>:<loc...> — the memory is exactly ONE atom
	//       after the root, and everything past it is the loc.
	//
	// Both still normalize to the same Loc for the common 3-part case, which is
	// the whole point of preferring these over PathSegments.
	if strings.Contains(path, "::") {
		parts := strings.Split(path, "::")
		return NodeLikeURNParts{
			MemoryURN: strings.Join(parts[:len(parts)-1], ":"),
			Loc:       parts[len(parts)-1],
			Fragment:  fragment,
		}, nil
	}
	atoms := strings.Split(path, ":")
	return NodeLikeURNParts{
		MemoryURN: atoms[0] + ":" + atoms[1],
		Loc:       strings.Join(atoms[2:], ":"),
		Fragment:  fragment,
	}, nil
}

// SplitNodeUrn splits a fully-qualified node URN into its memory URN and loc.
// Self-validating (AssertFullyQualifiedUrn(input, "node")). A #data fragment is
// reported separately rather than folded into Loc (urn-lib-js#13).
func SplitNodeUrn(input string) (NodeLikeURNParts, error) {
	return splitNodeLikeUrn(input, "node")
}

// SplitEdgeUrn splits a fully-qualified edge URN into its memory URN and loc
// (urn-lib-js#12). Same shape as SplitNodeUrn — the edge loc is an OPAQUE
// terminal and is never re-split into source:target.
func SplitEdgeUrn(input string) (NodeLikeURNParts, error) {
	return splitNodeLikeUrn(input, "edge")
}
