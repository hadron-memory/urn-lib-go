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
		if !isNodeRoleAlias {
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

// NodeURNParts is the result of SplitNodeUrn. JSON tags match the corpus.
type NodeURNParts struct {
	MemoryURN string `json:"memoryUrn"`
	Loc       string `json:"loc"`
}

// SplitNodeUrn splits a fully-qualified node URN into its memory URN and loc.
// Self-validating (AssertFullyQualifiedUrn(input, "node")).
func SplitNodeUrn(input string) (NodeURNParts, error) {
	if err := AssertFullyQualifiedUrn(input, "node"); err != nil {
		return NodeURNParts{}, err
	}
	path := input
	if m := qualPrefixStripRe.FindStringSubmatch(input); m != nil {
		path = m[1]
	}
	if strings.Contains(path, "::") {
		segments := strings.Split(path, "::")
		return NodeURNParts{MemoryURN: segments[0] + ":" + segments[1], Loc: strings.Join(segments[2:], ":")}, nil
	}
	atoms := strings.Split(path, ":")
	return NodeURNParts{MemoryURN: atoms[0] + ":" + atoms[1], Loc: strings.Join(atoms[2:], ":")}, nil
}
