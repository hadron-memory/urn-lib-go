# urn-lib-go

Hadron URN compose / parse / normalize for Go.

Paired with [`urn-lib-js`](https://github.com/hadron-memory/urn-lib-js). Both
implementations run the **same conformance corpus** (`fixtures/corpus.json`) —
the corpus is the contract, so the two languages cannot drift. This exists to
replace the copy-pasted URN parsers that had already drifted across
hadron-server / portal / docs / cli (hadron-server#239, #693).

## Status

**Increment 1 — v1-parity core.** Ports the pure, self-contained slice of
`hadron-server/src/lib/urn.ts` verbatim, behind the shared corpus:

- **scheme** — `HasSchemePrefix`, `NormalizeScheme`, `CanonicalScheme`, `LegacyScheme`
- **registry** — the locked type registry (`CoreTypes`, `RoleMarkers`, reserved slugs, …)
- **normalize** — `NormalizeUrnForLookup`, `LegacyMemoryUrnToCanonical`, `AgentSlugFromUrn`
- **slug** — `ValidateAtomShape`, `ValidateUserSlug`, `ValidateOrgSlug`, `DeriveSlugFromName`
- **errors** — `ParseError` + the machine-stable `Reason` codes

Not yet ported (later increments, gated by the same corpus): `ParseUrn` and the
qualification/format/compose surfaces, then the **grammar-v2 flat forms**
(hadron-server#694).

## Usage

```go
import urn "github.com/hadron-memory/urn-lib-go"

urn.NormalizeUrnForLookup("acme.com::specs::cor:urn") // "acme.com:specs:cor:urn"

if err := urn.ValidateOrgSlug("Acme.com"); err != nil {
    var pe *urn.ParseError
    if errors.As(err, &pe) {
        fmt.Println(pe.Reason) // "slug-not-lowercase"
    }
}
```

## The conformance corpus

`fixtures/corpus.json` is a synced copy of the canonical corpus in `urn-lib-js`.
Each case is `{ fn, in, out?, throws? }`; `corpus_test.go` runs every case.
**Add behavior by adding a corpus case**, not by editing a test in one language.

```bash
go test ./...
```

## License

MIT © Baragaun, Inc.
