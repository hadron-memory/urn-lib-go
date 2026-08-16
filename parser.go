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

// v2ToV1Type maps grammar-v2 flat type words that have a v1 canonical
// equivalent to it. A v2-emitted URN of one of these types is delegated to
// ParseUrnV2 and mapped into the ParsedURN shape (mem->memory, #697 emission
// flip). v2-ONLY types (apprun, noderev, appkey, ...) are absent on purpose —
// they never existed in the v1 parser surface, so a hrn:apprun:... input keeps
// its v1 unknown-type error rather than gaining a partial parse here.
var v2ToV1Type = map[string]string{
	"mem": "memory", "org": "org", "user": "user", "agent": "agent", "app": "app",
	"node": "node", "edge": "edge", "asset": "asset", "secret": "secret",
}

// v2FragmentToV1Type maps grammar-v2 fragment words to their v1 canonical TYPE
// word (urn-lib-js#8).
//
// v2 demoted the v1 node-PART type words — the registry's NodeParts, i.e. `data`
// and `condition` — from standalone types to `#<part>` fragments hanging off the
// parent node (#696). The demotion changed the SPELLING, not the meaning: v1's
// hrn:data:<root>::<mem>::<loc> and v2's hrn:node:<root>:<mem>:<loc>#data name
// the same resource, and v1's data URN carries exactly the parent node's path —
// only its type word differs. So the fragment maps back onto its v1 type word
// for Type dispatch, precisely as `mem` maps onto `memory` above.
//
// This is what makes the delegation SAFE for fragmented input: a consumer
// switching on Type lands in the "data" arm it has always had for the v1
// spelling, instead of the parent's "node" arm (the #994 harm). Derived from
// NodeParts so the two registries cannot drift. Mirrors urn-lib-js src/parser.ts.
var v2FragmentToV1Type = func() map[string]string {
	m := make(map[string]string, len(NodeParts))
	for _, part := range NodeParts {
		m[part] = part
	}
	return m
}()

// tryParseFlatV2 delegates a v1-rejected input to the grammar-v2 flat parser
// (#697). Returns (parsed, true) when input is a flat-v2 URN of a type with a v1
// equivalent, else (_, false) so ParseUrn rethrows the original v1 error. Type
// is the mapped v1 word (for consumer switch dispatch) while ParserCanonical
// keeps the actual v2 type word so the canonical string round-trips. v2 is
// already flat/pool-rooted, so no D11 resolver canonicalization applies.
//
// A FRAGMENTED input maps its fragment onto the fragment's v1 type word
// (urn-lib-js#8): hrn:node:<root>:<mem>:<loc>#data parses as Type "data" over
// the same path, which is exactly the v1 hrn:data:<root>::<mem>::<loc> reading.
// The fragment stays in ParserCanonical so the canonical string round-trips.
func tryParseFlatV2(input string) (ParsedURN, bool) {
	parsed, err := ParseUrnV2(input)
	if err != nil {
		return ParsedURN{}, false
	}
	mappedType, ok := v2ToV1Type[parsed.Type]
	if !ok {
		return ParsedURN{}, false
	}

	// Resolve the effective v1 type word. For a fragmented input that is the
	// FRAGMENT's v1 type word, not the parent's: v2 spells node-data as a #data
	// fragment of its node, v1 spelled it as the `data` type over the same path,
	// and both name the same resource (#696 demoted the type word to a fragment).
	//
	// Mapping it this way is what keeps the delegation from re-introducing #994:
	// reporting Type "node" here would send a consumer dispatching on the
	// structured fields to the PARENT node instead of its data resource. With the
	// fragment mapped, that consumer lands in the same "data" arm it has always
	// had for the v1 spelling. A fragment with no v1 type word (none today —
	// v2Fragments is {data}) is declined rather than silently flattened.
	//
	// A fragment on a v2-ONLY parent (hrn:apprun:...#data) never reaches here:
	// its parent type has no v2ToV1Type entry, so it keeps its v1 unknown-type
	// error like every other v2-only type.
	typ := mappedType
	fragmentSuffix := ""
	if parsed.Fragment != "" {
		fragmentType, ok := v2FragmentToV1Type[parsed.Fragment]
		if !ok {
			return ParsedURN{}, false
		}
		typ = fragmentType
		fragmentSuffix = "#" + parsed.Fragment
	}

	parserRewrites := []string{}
	if strings.HasPrefix(input, LegacyScheme+":") {
		parserRewrites = append(parserRewrites, "legacy-urn-scheme")
	}
	pathSegments := append([]string{parsed.Root}, parsed.Segments...)
	return ParsedURN{
		Type:                          typ,
		PathSegments:                  pathSegments,
		ParserCanonical:               CanonicalScheme + ":" + parsed.Type + ":" + strings.Join(pathSegments, ":") + fragmentSuffix,
		InputForm:                     input,
		ParserRewrites:                parserRewrites,
		NeedsResolverCanonicalization: false,
	}, true
}

// ParseUrn parses a URN string, applying parser-layer canonicalization (D11
// cats 1, 4). Returns a *ParseError on any acceptance violation. The v1 grammar
// is tried first; a v1-rejected input that is a valid flat grammar-v2 URN
// (single-colon, pool-rooted, `mem` type word — #697) is delegated to
// ParseUrnV2 and mapped into the ParsedURN shape. v1-accepted inputs keep their
// exact v1 result, so no existing behavior changes.
func ParseUrn(input string) (ParsedURN, error) {
	parsed, err := parseUrnV1(input)
	if err != nil {
		if v2, ok := tryParseFlatV2(input); ok {
			return v2, nil
		}
		if fragErr := fragmentOnlyFailure(input); fragErr != nil {
			return ParsedURN{}, fragErr
		}
		return ParsedURN{}, err
	}
	return parsed, nil
}

// fragmentOnlyFailure decides, for a rejected input carrying a #<fragment>,
// whether the fragment is the ONLY thing wrong with it — and if so returns a
// ReasonFragmentUnsupported error instead of leaking the v1 parser's (urn-lib-js#11).
//
// The v1 grammar has no concept of a fragment, so its complaint is always about
// the `#` CHARACTER (invalid-charset) or a v2 type word it doesn't know
// (unknown-type). Both are true internally and both mislead: they describe a
// symptom of the surface, not the defect in the URN.
//
// The discriminator is whether removing the fragment makes the input parse. If
// it does, the fragment is the whole problem. If it doesn't — hrn:worker:...#data,
// hrn:apprun:...#data, whose type words have no v1 equivalent at all — the
// original error stands, because fixing the fragment alone would NOT make the
// URN parse and reporting it would send the caller down the wrong path.
func fragmentOnlyFailure(input string) *ParseError {
	i := strings.Index(input, "#")
	if i == -1 {
		return nil
	}
	// The core cannot contain a "#", so this never recurses back into this branch.
	if _, err := ParseUrn(input[:i]); err != nil {
		return nil // broken with or without its fragment
	}
	return &ParseError{Input: input, Reason: ReasonFragmentUnsupported, OffendingSegment: input[i:]}
}

// parseUrnV1 is the v1-grammar parser (spec 021). See ParseUrn for the v2
// delegation wrapper.
func parseUrnV1(input string) (ParsedURN, error) {
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
