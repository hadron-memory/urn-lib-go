package urn

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// Conformance test: runs the shared fixture corpus (fixtures/corpus.json — the
// identical file urn-lib-js runs) against this implementation. Add behavior by
// adding a corpus case, not by editing a test in one language.

type corpusCase struct {
	Fn     string          `json:"fn"`
	In     []string        `json:"in"`
	Out    json.RawMessage `json:"out"`
	Throws string          `json:"throws"`
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
	case "deriveSlugFromName":
		return DeriveSlugFromName(args[0])
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

			if c.Throws != "" {
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("expected ParseError(%s), got err=%v", c.Throws, err)
				}
				if string(pe.Reason) != c.Throws {
					t.Fatalf("expected reason %q, got %q", c.Throws, pe.Reason)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(c.Out) > 0 {
				var want any
				if err := json.Unmarshal(c.Out, &want); err != nil {
					t.Fatalf("bad `out` in corpus: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("got %#v (%T), want %#v (%T)", got, got, want, want)
				}
			}
			// else: void success — reaching here (no error) is the pass.
		})
	}
}
