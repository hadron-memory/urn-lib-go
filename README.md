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
- **legacy** — `ParseUrnInput`, `FormatUrn`, `ValidateUrnType` (the pre-021 surface)
- **parser** — `ParseUrn` (the canonical-form parser, D11 cats 1 + 4), `IsParserCanonical`, `ToParserCanonical`
- **compose** — `FormatCanonicalUrn`, `ComposeNodeUrn`, `ComposeEdgeUrn`
- **errors** — `ParseError` + the machine-stable `Reason` codes

Not yet ported (later increments, gated by the same corpus): the **qualification**
family (`AssertFullyQualifiedUrn`, `SplitNodeUrn`) and the install/migration
helpers, then the **grammar-v2 flat forms** (hadron-server#694).

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

## Actions (grammar v2)

`hrn:action:<root>:<agent-slug>:<code>` has exactly two segments after the root,
keyed on the Team Agent (cor:urn:010:01). Use `ComposeActionUrnV2`; it parses
through `ParseUrnV2`, renders its own `action` display badge and normalizes a
legacy `urn:` prefix to `hrn:`. The code follows the shared atom rule (a dotted
code such as `task.finish.open-children` is valid, at most 64 characters). It has
no v1 equivalent; `ParseUrn` rejects it with `unknown-type`.

## Tickers and agent tasks (grammar v2)

`hrn:ticker:<root>:<app-slug>:<ticker-slug>` and
`hrn:agenttask:<root>:<app-slug>:<task-id>` each have exactly two segments after
the root. Use `ComposeTickerUrnV2` with the ticker's permanent derived slug and
`ComposeAgentTaskUrnV2` with the task's id. Both parse through `ParseUrnV2`,
render their own display badge, and normalize a legacy `urn:` prefix to `hrn:`.
They have no v1 equivalent; `ParseUrn` rejects them with `unknown-type`.

Task ids follow the shared URN atom rule (letters/digits, dots, underscores and
hyphens, at most 64 characters). A 32-hex or cuid id is valid in the task-id
leaf; root-name registration restrictions do not apply to that leaf.

## Root registration policy

`ValidateOrgSlug` admits dotted and undotted organization names. Both it and
`ValidateUserHandle` reject 32-hex/cuid-shaped roots (`root-id-shaped`) and the
reserved root words `hrn`/`urn` (`reserved-word-slug`). New handles remain
dot-free. Bare-name, lowercase, charset and length checks remain in place.

This is create/rename policy. Generic slug validation, URN parsing and
composition remain lenient so existing stored URNs and ID-shaped task leaves
still round-trip. The obsolete `org-root-not-dotted` reason is retired.

## License

MIT © Baragaun, Inc.
