package urn

import "testing"

// Round trip (urn-lib-go#25, FIT condition 4): what the builders emit parses
// back to the same parts, through the v2 parser, the typed decomposition and
// ParseUrn, and is canonical.
func TestAppSecretRoundTrip(t *testing.T) {
	cases := []struct{ root, slug, name string }{
		{"acme.com", "internal-ops", "stripe-key"},
		{"holger", "gmail-app", "k"},
		{"acme.com", "app", "app"},
		{"acme.com", "x", "a1234567890123456789012345678901234567890123456789012345678901"},
	}
	for _, c := range cases {
		u, err := ComposeAppSecretUrnV2(c.root, c.slug, c.name)
		if err != nil {
			t.Fatalf("compose %v: %v", c, err)
		}
		if !IsFlatV2(u) {
			t.Errorf("%s not canonical", u)
		}
		p, err := ParseSecretUrnV2(u)
		if err != nil || p.OwnerKind != SecretOwnerApp || p.Root != c.root || p.AppSlug != c.slug || p.Name != c.name {
			t.Errorf("typed round trip %s: %+v %v", u, p, err)
		}
		v, err := ParseUrnV2(u)
		if err != nil || v.Root != c.root || len(v.Segments) != 3 || v.Segments[0] != "app" || v.Segments[1] != c.slug || v.Segments[2] != c.name {
			t.Errorf("v2 round trip %s: %+v %v", u, v, err)
		}
		g, err := ParseUrn(u)
		if err != nil || g.Type != "secret" || g.ParserCanonical != u {
			t.Errorf("ParseUrn round trip %s: %+v %v", u, g, err)
		}
	}
	// The flat builder round-trips as the root kind, never the App kind.
	u, _ := ComposeSecretUrnV2("acme.com", "app")
	p, err := ParseSecretUrnV2(u)
	if err != nil || p.OwnerKind != SecretOwnerRoot || p.Name != "app" || p.AppSlug != "" {
		t.Errorf("flat round trip %s: %+v %v", u, p, err)
	}
}
