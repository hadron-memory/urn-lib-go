package urn

import (
	"errors"
	"testing"
)

// The root word `server` is reserved for the platform's own root
// (cor:urn:010:01, Holger 2026-11-10 R3): registration validators refuse it
// case-insensitively, as a root only (urn-lib-go#21).
func TestReservedServerRoot(t *testing.T) {
	validators := map[string]func(string) error{
		"ValidateOrgSlug":    ValidateOrgSlug,
		"ValidateUserHandle": ValidateUserHandle,
	}
	for name, v := range validators {
		for _, in := range []string{"server", "SERVER", "Server", "sErVeR"} {
			err := v(in)
			var pe *ParseError
			if !errors.As(err, &pe) || pe.Reason != ReasonReservedWordSlug {
				t.Errorf("%s(%q): want reserved-word-slug, got %v", name, in, err)
			}
		}
	}
}

// Root-only: `server` stays legal as a resource slug, and near-misses stay legal roots.
func TestServerNotReservedOutsideRoot(t *testing.T) {
	if err := ValidateUserSlug("server"); err != nil {
		t.Errorf("ValidateUserSlug(server) must stay accepted (root-only reservation): %v", err)
	}
	for _, in := range []string{"servers", "myserver", "server-1"} {
		if err := ValidateOrgSlug(in); err != nil {
			t.Errorf("ValidateOrgSlug(%q) refused: %v", in, err)
		}
	}
}
