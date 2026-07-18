package urn

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

// Conformance test: runs the shared fixture corpus (fixtures/corpus.json — the
// identical file urn-lib-js runs) against this implementation. Add behavior by
// adding a corpus case, not by editing a test in one language.

type corpusCase struct {
	Fn               string          `json:"fn"`
	In               []string        `json:"in"`
	Out              json.RawMessage `json:"out"`
	Throws           string          `json:"throws"`
	OffendingSegment string          `json:"offendingSegment"`
	ThrowsQualified  bool            `json:"throwsQualified"`
	ThrowsAny        bool            `json:"throwsAny"`
}

// call dispatches a corpus fn. All corpus args are strings. Value-returning fns
// return (value, nil); void validators return (nil, err).
func call(fn string, args []string) (any, error) {
	switch fn {
	case "hasSchemePrefix":
		return HasSchemePrefix(args[0]), nil
	case "normalizeScheme":
		return NormalizeScheme(args[0]), nil
	case "normalizeUrnForLookup":
		return NormalizeUrnForLookup(args[0]), nil
	case "legacyMemoryUrnToCanonical":
		return LegacyMemoryUrnToCanonical(args[0]), nil
	case "agentSlugFromUrn":
		return AgentSlugFromUrn(args[0]), nil
	case "validateUserSlug":
		return nil, ValidateUserSlug(args[0])
	case "validateOrgSlug":
		return nil, ValidateOrgSlug(args[0])
	case "validateUserHandle":
		return nil, ValidateUserHandle(args[0])
	case "deriveSlugFromName":
		return DeriveSlugFromName(args[0])
	case "isValidSlug":
		return IsValidSlug(args[0]), nil
	case "formatUrn":
		return FormatUrn(args[0], args[1]), nil
	case "parseUrnInput":
		return ParseUrnInput(args[0]), nil
	case "validateUrnTypeFromInput":
		return ValidateUrnType(ParseUrnInput(args[0]), args[1]), nil
	case "parseUrn":
		return ParseUrn(args[0])
	case "isParserCanonical":
		return IsParserCanonical(args[0]), nil
	case "toParserCanonical":
		return ToParserCanonical(args[0])
	case "formatCanonicalUrn":
		return FormatCanonicalUrn(args[0], args[1])
	case "composeNodeUrn":
		return ComposeNodeUrn(args[0], args[1])
	case "composeEdgeUrn":
		return ComposeEdgeUrn(args[0], args[1])
	case "assertFullyQualifiedUrn":
		return nil, AssertFullyQualifiedUrn(args[0], args[1])
	case "splitNodeUrn":
		return SplitNodeUrn(args[0])
	case "composeInstalledAgentUrn":
		return ComposeInstalledAgentUrn(args[0], args[1])
	case "parseForRow":
		var norm *time.Time
		if args[1] == "1" {
			tt := time.Unix(0, 0)
			norm = &tt
		}
		return ParseFor(UrnRow{URN: args[0], URNNormalizedAt: norm})
	case "parseDisplayUrn":
		hint := ""
		if len(args) > 1 {
			hint = args[1]
		}
		return ParseDisplayUrn(args[0], hint), nil
	case "parseUrnV2":
		return ParseUrnV2(args[0])
	case "composeUrnV2":
		return ComposeUrnV2(args[0], args[1], args[2:]...)
	case "isFlatV2":
		return IsFlatV2(args[0]), nil
	case "composeSecretUrnV2":
		return ComposeSecretUrnV2(args[0], args[1])
	case "composeAppRunUrnV2":
		return ComposeAppRunUrnV2(args[0], args[1], args[2])
	case "composeNodeRevUrnV2":
		return ComposeNodeRevUrnV2(args[0], args[1], args[2], args[3])
	case "composeDataFragmentV2":
		return ComposeDataFragmentV2(args[0])
	case "parseNodeRevUrnV2":
		return ParseNodeRevUrnV2(args[0])
	default:
		return nil, fmt.Errorf("unknown fn %q in corpus", fn)
	}
}

func TestCorpus(t *testing.T) {
	data, err := os.ReadFile("fixtures/corpus.json")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var corpus struct {
		Cases []corpusCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("empty corpus")
	}

	for i, c := range corpus.Cases {
		t.Run(fmt.Sprintf("#%d_%s", i, c.Fn), func(t *testing.T) {
			got, err := call(c.Fn, c.In)

			if c.ThrowsQualified {
				var qe *NotQualifiedError
				if !errors.As(err, &qe) {
					t.Fatalf("expected NotQualifiedError, got err=%v", err)
				}
				return
			}

			if c.ThrowsAny {
				if err == nil {
					t.Fatalf("expected an error, got none")
				}
				return
			}

			if c.Throws != "" {
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("expected ParseError(%s), got err=%v", c.Throws, err)
				}
				if string(pe.Reason) != c.Throws {
					t.Fatalf("expected reason %q, got %q", c.Throws, pe.Reason)
				}
				if c.OffendingSegment != "" && pe.OffendingSegment != c.OffendingSegment {
					t.Fatalf("expected offendingSegment %q, got %q", c.OffendingSegment, pe.OffendingSegment)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(c.Out) > 0 {
				// JSON-normalize both sides so a Go struct/nil compares
				// structurally against the corpus JSON (matches urn-lib-js).
				var want any
				if err := json.Unmarshal(c.Out, &want); err != nil {
					t.Fatalf("bad `out` in corpus: %v", err)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatalf("marshal got: %v", err)
				}
				var gotNorm any
				if err := json.Unmarshal(gotJSON, &gotNorm); err != nil {
					t.Fatalf("normalize got: %v", err)
				}
				if !reflect.DeepEqual(gotNorm, want) {
					t.Fatalf("got %s, want %s", gotJSON, c.Out)
				}
			}
			// else: void success — reaching here (no error) is the pass.
		})
	}
}
