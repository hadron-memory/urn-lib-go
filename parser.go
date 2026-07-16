package urn

import (
	"regexp"
	"strings"
)

// Canonical-form URN parser (spec 021). Ported verbatim from hadron-server
// src/lib/urn.ts. Returns a *ParseError on any acceptance-rule violation.

// ParsedURN is the result of ParseUrn. JSON tags match the conformance corpus.
type ParsedURN struct {
	Type                          string   `json:"type"`
	PathSegments                  []string `json:"pathSegments"`
	ParserCanonical               string   `json:"parserCanonical"`
	InputForm                     string   `json:"inputForm"`
	ParserRewrites                []string `json:"parserRewrites"`
	NeedsResolverCanonicalization bool     `json:"needsResolverCanonicalization"`
}

var locPrefixRe = regexp.MustCompile(`(?i)^((urn|hrn):)?loc:`)
var prefixRe = regexp.MustCompile(`^(hrn|urn):([a-z][a-z0-9-]*):(.+)$`)
var embeddedLocRe = regexp.MustCompile(`(^|::)loc:`)
var secretUserRootRe = regexp.MustCompile(`^user:([A-Za-z0-9._-]+)$`)

// rewriteSecretUserRoot (#679): normalize a `user:<handle>` secret owner-root to
// the canonical `@<handle>` form. ROOT (position 0) only.
func rewriteSecretUserRoot(segments []string) []string {
	if len(segments) == 0 {
		return segments
	}
	if m := secretUserRootRe.FindStringSubmatch(segments[0]); m != nil {
		return append([]string{"@" + m[1]}, segments[1:]...)
	}
	return segments
}

// validateSecretSegments (#679): a secret URN is `<root>::[<app|memory>:<slug>::]<name>`
// — at most 3 segments; when the middle owner segment is present it MUST be
// exactly `app:<slug>` or `memory:<slug>`.
func validateSecretSegments(input string, segments []string) error {
	if len(segments) > 3 {
		return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: segments[3]}
	}
	if len(segments) == 3 {
		atoms := strings.Split(segments[1], ":")
		if len(atoms) != 2 || (atoms[0] != "app" && atoms[0] != "memory") {
			return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: segments[1]}
		}
	}
	return nil
}

// validatePathSegment: per-segment charset/length validation + the spec-047
// @<handle> owner-namespace gate.
func validatePathSegment(input, segment, typ string, index, totalSegments int) error {
	atoms := strings.Split(segment, ":")
	ownerNamespaced := ownerNamespacedSet[typ]

	authorContextHere := false
	if index >= 1 {
		switch {
		case typ == "agent":
			authorContextHere = index <= totalSegments-1
		case typ == "memory":
			authorContextHere = index <= totalSegments-2
		case nodeURNTypeSet[typ] || typ == "edge":
			authorContextHere = index <= totalSegments-3
		}
	}

	markerPrefixed := index >= 1 && len(atoms) >= 2 && TypeMarkers[atoms[0]]
	handleIdx := 0
	if markerPrefixed {
		handleIdx = 1
	}

	for j := 0; j < len(atoms); j++ {
		atom := atoms[j]
		isOwnerHandleAtom := ownerNamespaced &&
			j == handleIdx &&
			strings.HasPrefix(atom, "@") &&
			(index == 0 || (authorContextHere && len(atoms) >= handleIdx+2))
		if isOwnerHandleAtom {
			atom = atom[1:]
			if len(atom) == 0 {
				return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: segment}
			}
		}
		if err := ValidateAtomShape(input, atom); err != nil {
			return err
		}
	}
	return nil
}

func rejectInvalidSegmentShapes(input, typ string, segments []string) error {
	finalIdx := len(segments) - 1
	isNodeURN := nodeURNTypeSet[typ]
	for i := 0; i < len(segments); i++ {
		segment := segments[i]
		atomCount := len(strings.Split(segment, ":"))
		if i == 0 {
			if atomCount != 1 {
				return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: segment}
			}
			continue
		}
		if isNodeURN && i == finalIdx {
			continue // leaf node loc: unbounded
		}
		if atomCount > 3 {
			return &ParseError{Input: input, Reason: ReasonInvalidSegmentShape, OffendingSegment: segment}
		}
	}
	return nil
}

func rejectReservedWordsAtIllegalPositions(input, typ string, segments []string) error {
	// secret (#679): enforce its own shape here (it skips cat-4 stripping).
	if typ == "secret" {
		return validateSecretSegments(input, segments)
	}
	finalIdx := len(segments) - 1
	roleMarkerIdx := -1 // -1 = no structural role-marker position
	minSegments := 0
	switch {
	case typ == "memory":
		roleMarkerIdx = finalIdx
		minSegments = 3
	case nodeURNTypeSet[typ]:
		roleMarkerIdx = finalIdx - 1
		minSegments = 4
	}

	leafIsNodeLoc := nodeURNTypeSet[typ]
	for i := 0; i < len(segments); i++ {
		if leafIsNodeLoc && i == finalIdx {
			continue
		}
		atoms := strings.Split(segments[i], ":")
		for j := 0; j < len(atoms); j++ {
			atom := atoms[j]
			lower := strings.ToLower(atom)
			if !reservedSlugSet[lower] {
				continue
			}
			isRoleMarkerPosition := roleMarkerIdx != -1 &&
				i == roleMarkerIdx &&
				j == 0 &&
				len(segments) >= minSegments &&
				roleMarkerSet[lower]
			if isRoleMarkerPosition {
				continue
			}
			return &ParseError{Input: input, Reason: ReasonReservedWordSlug, OffendingSegment: atom}
		}
	}
	return nil
}

func stripTypeMarkers(segments []string, urnType string) ([]string, bool) {
	// secret (#679): the app:/memory: marker is STRUCTURAL — never strip it.
	if urnType == "secret" {
		return segments, false
	}
	fired := false
	isNodeURN := nodeURNTypeSet[urnType]
	lastIdx := len(segments) - 1
	out := make([]string, len(segments))
	for i, segment := range segments {
		if i == 0 {
			out[i] = segment
			continue
		}
		if isNodeURN && i == lastIdx {
			out[i] = segment
			continue
		}
		atoms := strings.Split(segment, ":")
		if len(atoms) >= 2 && TypeMarkers[atoms[0]] {
			fired = true
			out[i] = strings.Join(atoms[1:], ":")
			continue
		}
		out[i] = segment
	}
	return out, fired
}

func collapseSelfInstall(typ string, segments []string) ([]string, bool) {
	if (typ != "memory" && typ != "agent") || len(segments) < 2 {
		return segments, false
	}
	orgSeg := segments[0]
	lastScanIdx := len(segments) - 1
	if typ == "memory" {
		lastScanIdx = len(segments) - 2
	}
	for i := 1; i <= lastScanIdx; i++ {
		atoms := strings.Split(segments[i], ":")
		if len(atoms) != 2 {
			continue
		}
		if atoms[0] != orgSeg {
			continue
		}
		collapsed := make([]string, 0, len(segments))
		collapsed = append(collapsed, segments[:i]...)
		collapsed = append(collapsed, atoms[1])
		collapsed = append(collapsed, segments[i+1:]...)
		return collapsed, true
	}
	return segments, false
}

func needsCat2(typ string, _ []string) bool { return typ == "node" }

func needsCat3(typ string, segments []string) bool {
	if typ != "agent" || len(segments) < 3 {
		return false
	}
	installSlot := segments[len(segments)-1]
	return len(strings.Split(installSlot, ":")) == 1
}

// ParseUrn parses a URN string, applying parser-layer canonicalization (D11
// cats 1, 4). Returns a *ParseError on any acceptance violation.
func ParseUrn(input string) (ParsedURN, error) {
	if locPrefixRe.MatchString(input) {
		return ParsedURN{}, &ParseError{Input: input, Reason: ReasonLocSegmentRejected}
	}
	m := prefixRe.FindStringSubmatch(input)
	if m == nil {
		return ParsedURN{}, &ParseError{Input: input, Reason: ReasonMalformedGrammar}
	}
	legacySchemeUsed := m[1] == LegacyScheme
	typ := m[2]
	if !urnTypeSet[typ] {
		return ParsedURN{}, &ParseError{Input: input, Reason: ReasonUnknownType}
	}
	path := m[3]
	if embeddedLocRe.MatchString(path) {
		return ParsedURN{}, &ParseError{Input: input, Reason: ReasonLocSegmentRejected}
	}
	if strings.HasSuffix(path, "::") {
		return ParsedURN{}, &ParseError{Input: input, Reason: ReasonTrailingDoubleColon}
	}
	rawSegments := strings.Split(path, "::")
	for _, s := range rawSegments {
		if s == "" {
			return ParsedURN{}, &ParseError{Input: input, Reason: ReasonEmptySegment}
		}
	}

	// secret (#679): normalize a `user:<handle>` owner-root to `@<handle>` at
	// ROOT position 0 before the shape checks.
	workSegments := rawSegments
	if typ == "secret" {
		workSegments = rewriteSecretUserRoot(rawSegments)
	}

	for i := 0; i < len(workSegments); i++ {
		if err := validatePathSegment(input, workSegments[i], typ, i, len(workSegments)); err != nil {
			return ParsedURN{}, err
		}
	}

	parserRewrites := []string{}
	if legacySchemeUsed {
		parserRewrites = append(parserRewrites, "legacy-urn-scheme")
	}

	cat4Segments, cat4Fired := stripTypeMarkers(workSegments, typ)
	if cat4Fired {
		parserRewrites = append(parserRewrites, "type-marker-optionality")
	}

	if err := rejectInvalidSegmentShapes(input, typ, cat4Segments); err != nil {
		return ParsedURN{}, err
	}
	if err := rejectReservedWordsAtIllegalPositions(input, typ, cat4Segments); err != nil {
		return ParsedURN{}, err
	}

	cat1Segments, cat1Fired := collapseSelfInstall(typ, cat4Segments)
	if cat1Fired {
		parserRewrites = append(parserRewrites, "source-install-memory")
	}

	needsResolver := needsCat2(typ, cat1Segments) || needsCat3(typ, cat1Segments)
	parserCanonical := CanonicalScheme + ":" + typ + ":" + strings.Join(cat1Segments, "::")

	return ParsedURN{
		Type:                          typ,
		PathSegments:                  cat1Segments,
		ParserCanonical:               parserCanonical,
		InputForm:                     input,
		ParserRewrites:                parserRewrites,
		NeedsResolverCanonicalization: needsResolver,
	}, nil
}

// IsParserCanonical reports whether ParseUrn(input).ParserCanonical == input
// with no rewrites.
func IsParserCanonical(input string) bool {
	parsed, err := ParseUrn(input)
	if err != nil {
		return false
	}
	return parsed.ParserCanonical == input && len(parsed.ParserRewrites) == 0
}

// ToParserCanonical parses and returns the parser-layer canonical form.
func ToParserCanonical(input string) (string, error) {
	parsed, err := ParseUrn(input)
	if err != nil {
		return "", err
	}
	return parsed.ParserCanonical, nil
}
