# FulcrumPro/folio fork

This is FulcrumPro's working fork of [carlos7ags/folio](https://github.com/carlos7ags/folio).

## Why we fork

Fulcrum's HTML→PDF pipeline (`FulcrumProduct/fulcrum/internal/pdfrender`) ships
hand-ported `.hbs` templates from `FulcrumProduct/DocGen/`. When folio v0.9.1
diverges from CSS / browser behavior, we have two choices:

1. Modify the templates on the Fulcrum side to dodge the gap. Each deviation
   adds drift between `.hbs` source and our Go-side copy, and accumulates
   tribal knowledge about "things to avoid because of folio."
2. Fix folio. Templates stay byte-for-byte close to the `.hbs` source, and the
   fix benefits every future endpoint we port.

This fork is where (2) lives. We don't currently file upstream PRs — patches
stay here.

## Branch layout

- `main` — tracks `carlos7ags/folio` upstream `main`. Don't push patches here.
  Used to pull in upstream changes via merge or rebase onto `fulcrum`.
- `fulcrum` — working branch. All Fulcrum patches land here, on top of the
  upstream tag we depend on. Released as `vX.Y.Z-fulcrum.N` tags.

## Versioning

Tags follow `v<upstream>-fulcrum.<n>`:

- `v0.9.1-fulcrum.0` — fork mirror of upstream v0.9.1, zero patches. Baseline.
- `v0.9.1-fulcrum.1`, `.2`, … — each patch ships its own tag, in commit order.

Semver-prerelease ordering puts `v0.9.1-fulcrum.N` *before* `v0.9.1` in semver
terms; that's fine because Fulcrum's `go.mod` uses a `replace` directive,
which short-circuits version comparison.

## How Fulcrum's repo wires it

In `FulcrumProduct/fulcrum/go.mod`:

```
replace github.com/carlos7ags/folio => github.com/FulcrumPro/folio v0.9.1-fulcrum.N
```

The replace directive lets us keep all imports as `github.com/carlos7ags/folio/*`
(no rename across hundreds of import statements) while pointing the resolver at
this fork. Bumping `N` is the only thing that has to change when we ship a new
patch.

## Pulling upstream

When `carlos7ags/folio` cuts a new release we want:

```
git fetch upstream --tags
git checkout fulcrum
git rebase v0.X.Y    # the new upstream tag
# resolve conflicts patch-by-patch
git tag v0.X.Y-fulcrum.0
git push --force-with-lease origin fulcrum
git push origin v0.X.Y-fulcrum.0
```

Then bump the Fulcrum-side `go.mod` replace to the new tag.

## Patches

Each patch lands as a single, focused commit on the `fulcrum` branch. The
table below is the source of truth — when you add or remove a patch, update
this table in the same commit.

| Tag | Patch | Folio file(s) | Fulcrum-side workaround removed |
|---|---|---|---|
| `v0.9.1-fulcrum.1` | Keep inline whitespace between adjacent inline siblings | `html/converter.go` (`isInlineFlowChild`, plus new `isInlineFlowElement` and `participatesInInlineFlow` helpers) | `fulcrum/internal/pdfrender/inline_ws.go` (HTML pre-rewrite) |
| `v0.9.1-fulcrum.2` | Accept `start` / `end` shorthand on `justify-content` (CSS Box Alignment Level 3) | `html/converter_flex.go` (justify-content switch, lines 30-44) | Lets `justify-content: end` from .NET source render correctly without rewriting templates to use `flex-end` |
| `v0.9.1-fulcrum.3` | Inherited `text-align` was not applied to child block paragraphs because `computedStyle.inherit()` copied the value but not the `TextAlignSet` flag the apply path conditioned on | `html/style.go` (`inherit()`) | Lets the .NET source's `text-align: right`/`center` parent rules work on child blocks without sprinkling `text-align: …` directly on every block — drops the per-block `text-center` / `text-right` class duplication in the hand-ported templates |
| `v0.9.1-fulcrum.4` | Horizontal margin on inline elements (`<span style="margin-right: 5px">`) was ignored — `collectRunsFromNode` had no margin handling | `html/converter_paragraph.go` (collectRunsFromNode element branch, ~lines 320-360) | Lets `<span class="data-label">…</span><span class="data-value">…</span>` from .NET DocGen render with the requested 5px gap; once we regenerate `dataitem.html` from `.hbs` source, the line-break workaround between the two spans goes away. Approximation note: exact font-metric calibration isn't possible without the run measurer (layout time), so the rendered gap is via `LetterSpacing` on a single space and is approximate but always >= 0. |
| `v0.9.1-fulcrum.5` | `float: left/right` on direct flex children was honored, contradicting CSS Flexbox §3 — folio wrapped the child in a `layout.Float` and the flex width calc mis-shrunk the resulting non-flex-shaped item | `html/converter_flex.go` (post-convertNode unwrap loop) plus a new `Float.Content()` accessor in `layout/float.go` | Lets the .NET DocGen `.three-columns { float: left }` inside `.data-container { display: flex }` render with full 33% column widths instead of mis-wrapped narrow columns — drops the styles.html deviation that removed `float: left` from `.three-columns` |
| `v0.9.1-fulcrum.6` | `float: left/right` on inline elements (`<span style="float:right">`) was ignored — `isInlineFlowElement` returned true regardless of float, so the span stayed in the inline buffer and rendered as plain text at the left margin | `html/converter.go` (`isInlineFlowElement` adds float check) | CSS 2.1 §9.7: float promotes inline elements to block-level. With the patch, `<span style="float:right">` routes through `convertElement`'s existing block-level float-wrap path. Lets the .NET DocGen "Created By" right-floated span and grand-total float-right pattern render correctly without rewriting templates to use flex layouts. |
| `v0.9.1-fulcrum.7` | Column flex distributed `flex-grow` to children even when the container had no definite main-axis (vertical) size, contradicting CSS Flexbox §9.7 | `layout/flex.go` (`planColumn` Phase 2 gated on `heightUnit != nil \|\| hasDefiniteCrossSize`) | The .NET v3 sales/purchasing templates use `<div class="v3-info-contain1" style="display:flex;flex-direction:column">` containing a `<div class="v3-pdf-details-left" style="flex-grow:1;border:1px solid">` for BILLING. Folio was reading the row's available page height as the column's "main size" and using it to grow the bordered child to nearly page-bottom. Patch: only distribute remaining main-axis space to grow children when the column's main-axis size is definite (explicit `height:`, OR the column has been stretched by a row-direction parent that set hasDefiniteCrossSize). |
| `v0.9.1-fulcrum.8` | CSS `position: fixed; bottom: 0` placed the element's TOP at the requested Y, dropping the entire footer below the page edge | `layout/renderer.go` (new `BottomAnchored` field on `absoluteItem` + `AbsoluteOpts`); `layout/render_plans.go` (`renderAbsolutes` adds `plan.Consumed` to `item.y` when bottom-anchored); `html/converter.go` (`AbsoluteItem.BottomAnchored` set when CSS `bottom:` is used); `tmpl/tmpl.go` (forwards through `AddAbsoluteWithOpts`); `document/document.go` (struct + opts plumbing) | The .NET v3 templates' `<div id="pageFooter">` becomes `position: fixed; bottom: 0` inside `@media print` to anchor "Last Modified by …" / "Powered By // fulcrum" at the page bottom. Without the patch the footer rendered ~10pt below the page edge — invisible. With it, the footer's bottom edge sits at the requested Y. |
| `v0.9.1-fulcrum.9` | CSS Flexbox §9.7 step 4: items grown by the basis-and-grow pass were never clamped to their CSS `min-width` / `max-width`, so `.contain-left { flex: 1; min-width: 50%; max-width: 55% }` shrank to its 1/3 flex-grow share instead of being held at the 50% floor | `layout/flex.go` (new `minMainSize`/`maxMainSize` on `FlexItem` + `clampFlexItemsToMinMax` after `resolveGrowShrink`); `layout/div.go` (new `ClearMinWidthUnit`/`ClearMaxWidthUnit`); `html/converter_flex.go` (resolves and consumes min/max-width on flex items, then clears the inner Div's units to avoid double-resolution) | The .NET DocGen v3 BILLING/SHIPPING row uses `<div class="data-container" style="display:flex"><div class="contain-left" style="flex:1;min-width:50%;max-width:55%">…</div><div class="contain-right" style="flex:2">…</div></div>`. Pre-patch, .left was given ~33% of the row (its 1/3 grow share) and addresses like "Acme Industries" wrapped onto two lines where Chromium kept them on one. Patch: track resolved min/max on FlexItem, do a single-pass CSS-spec clamp (freeze clamped items, redistribute the delta to unclamped growers); clear the inner Div's `min-width`/`max-width` percentage units so the constraint isn't reapplied a second time against the FlexItem-allocated width (which would clamp `.left`'s 150pt allocation to `0.55 × 150 = 82.5pt`). |
| `v0.9.1-fulcrum.10` | Converter consumed CSS `width` as flex-basis regardless of flex direction, swallowing the cross-axis size in column-flex containers and stretching children to the full cross axis | `html/converter_flex.go` (gates the "CSS width → flex-basis" promotion on row-direction flex containers only) | Per CSS Flexbox spec, `width` / `height` always refer to physical dimensions while `flex-basis` tracks the main axis — so in `flex-direction: column`, `width: 250px` is the cross-axis constraint, not a vertical basis. The .NET DocGen v3 BILLING block uses `.v3-info-contain1 { flex: 1; display: flex; flex-direction: column }` containing `.v3-pdf-details-left { flex-grow: 1; width: 250px; border:… }`. Pre-patch, the bordered BILLING box stretched edge-to-edge on the page (~600pt) instead of sitting at 250pt as Chromium does. Patch: only treat `width` as basis when the flex container is row-direction; otherwise leave the Div's `widthUnit` intact so its 250pt cross-axis width is honored. |

### Conventions for patch commits

- Subject line: `fulcrum: <short description>` (e.g. `fulcrum: keep inline whitespace between adjacent inline siblings`).
- Body explains: what the bug is, what changes, what (if any) Fulcrum-side workaround
  the fix unlocks deleting, link to the corresponding Fulcrum PR.
- Include or extend a test under `fulcrum/` (see below) so the fix has a regression net.

## Fulcrum-specific tests

Tests that document our patches live under `fulcrum/` at the repo root, not
mixed into upstream's `*_test.go` files. This keeps the diff against
`carlos7ags/folio` minimal and obvious — `git diff carlos7ags/main..fulcrum`
should show exactly: (a) the surgical converter changes, (b) the matching
`fulcrum/` test files, (c) this `FORK.md`.

The `fulcrum/diag/` subdirectory holds a small `pdfdump` program used for
triangulating folio gaps (decompress FlateDecode'd content streams, inspect
`Tj`/`TJ` text-show ops, and compare against expected positions).

## v0.9.1 rebase notes (was v0.7.1)

Rebased the patch stack from upstream v0.7.1 onto v0.9.1 (via v0.8.0, v0.9.0).
Outcome of the 11 original patches:

- **Dropped (1):** the old patch #9 (`parseFontWeight` 500/600 → "bold") is superseded.
  Upstream v0.8.0 replaced string-valued weight resolution with a numeric
  CSS Fonts L4 weight ladder (`parseFontWeight(value, inherited) int`) plus
  nearest-weight `@font-face` matching, so `font-weight: 600` resolves to the
  Bold face on its own. More spec-correct; our coercion is no longer needed.
- **Adapted (1):** the inline-margin patch (now `v0.9.1-fulcrum.4`) was updated
  for upstream's lazy `*cssLength` margin migration — `childStyle.MarginLeft`
  is now `childStyle.MarginLeftAt(c.containerWidth)`. (This was a *semantic*
  conflict: it merged without a git conflict but failed `go build`.)
- **Conflict-resolved, still needed:**
  - `#3` TextAlignSet inheritance — upstream now propagates `TextAlignKeyword` /
    `TextAlignLast*` in `inherit()` but still omits `TextAlignSet`; we union our
    field into upstream's richer copy.
  - `#8` `position:fixed/absolute` bottom-anchor — upstream reworked
    `position:fixed` to render on *every* page and threaded `PageIdx`/`TotalPages`
    through `DrawContext`. Our `bottomAnchored` field/translation composes
    orthogonally (bottom-anchor geometry vs every-page emission) — a footer can
    now be both. Resolved as a struct-field union + a `drawY` logic merge in
    `layout/render_plans.go`.
  - `#10` (now `.9`) `width`-as-basis row-only gate — upstream already computes
    `childStyle` at the top of the flex-children loop, so we keep only the
    `isRowDirection` gate (dropped the duplicate `childStyle` block).

The full fork test suite (`go test ./...`) is green on v0.9.1, including the
`fulcrum/` patch-regression tests.
