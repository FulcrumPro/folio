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
| `v0.9.1-fulcrum.11` | A run of separate `display:grid` blocks in normal flow silently dropped every block past the first page. `grid.buildOverflowResult`'s "not even the first row fits" branch returned `LayoutFull` — force-drawing the grid past the page edge and reporting to the parent that it fit — so the parent kept placing subsequent grids off the page bottom, where they were clipped. | `layout/grid.go` (`buildOverflowResult` returns `LayoutNothing` when `lastFitRow < 0` instead of force-drawing) | The .NET DocGen v3 invoice/PO/quote/sales-order templates render every line-item row as its own `display:grid` element (`.v2-items-row { display: grid }` in styles-v3). Pre-patch, a multi-page invoice kept only the rows that fit page 1 and dropped the rest (16 line items rendered as 5). `Div` and the renderer already handle `LayoutNothing` correctly (push the whole element to the next page; force-render only when alone at page top); Flex already returned `LayoutNothing` in the same case, so this aligns Grid with Flex. Regression test: `fulcrum/grid_pagination_test.go`. |
| `v0.9.1-fulcrum.12` | A borderless single-row/single-cell layout-wrapper `<table>` trapped its entire body in one cell. folio's table paginator splits only between rows, so the single tall cell overflowed page 1 and the rest was clipped — multi-page Job/WorkOrder travelers rendered ~26% of their content on one page. | `html/converter_table.go` (`convertTable` detects a chrome-less single-row/single-cell table via `layoutWrapperBodyCell` and emits the cell's content as block flow instead of a `Table`) | The .NET DocGen Job/WorkOrder travelers wrap the whole body in `<table class="wo-pdf-preview-table"><thead>`(spacer)`</thead><tbody><tr><td>`…`</td>` to reserve per-page header space. folio can't fragment a single tall cell across pages, so content past page 1 was dropped. Routing the cell content through block flow (which folio paginates correctly) recovers it; nested real (multi-row) tables still render as tables and split between their rows. Gated so a table OR cell with a visible box (border/background/border-radius) keeps its `Table` treatment. WorkOrder: 26%→96% content, 1→4 pages matching Chrome; CAPA/NCR/commerce unchanged. Regression test: `fulcrum/table_wrapper_pagination_test.go`. |
| `v0.9.1-fulcrum.13` | An empty (but visible) element child of a flex container was dropped — `convertFlex` skipped any child whose conversion produced zero layout elements (`len(childElems)==0 → continue`). That removed it from the flex-item count, changing `justify-content` distribution. Per CSS Flexbox §4 every non-`display:none` in-flow child generates a flex item (zero-size for empty content) that participates in distribution. | `html/converter_flex.go` (`convertFlex` synthesizes a zero-size placeholder `Div` for an empty visible element child; new `isNonVisualFlexChild` helper excludes script/style/link/title/meta/head/base; `display:none` and non-element nodes still generate no item) | The .NET DocGen v3 commerce footer is `<div class="footer" style="display:flex;justify-content:space-between"><div class="last-modified"></div><div class="powered-by">Powered By …</div></div>`. The `.last-modified` block is empty when the doc has no modified-by stamp. Chrome keeps it as a zero-width first item, so space-between holds "Powered By // fulcrum" at the right edge; folio dropped it, leaving one item, and space-between with one item packs at flex-start — the footer rendered hard left instead of right. Patch keeps the empty item. PO v3 footer: left→right, matching Chrome. Regression test: `fulcrum/flex_empty_item_test.go`. |
| `v0.9.1-fulcrum.14` | A flex container dropped the overflow of a fragmentable item. `Flex.planRow` read only each item plan's `Consumed` height and discarded its `Status`/`Overflow`, then reported `LayoutFull` even when an item's block child returned `LayoutPartial` (only part fit the remaining page). The remainder vanished and no later page was produced. | `layout/flex.go` (`planRow` captures each item plan's overflow into `rowOverflowItems`; returns `LayoutPartial` with an overflow flex when content paginated, or `LayoutNothing` when nothing fit so the renderer relocates/force-renders; new `cloneItemWithElement` + `overflowWithItems` helpers) | The .NET DocGen v3 commerce templates wrap the totals in `<div class="document-summary-wrapper no-break"><div class="document-summary-main" style="display:flex"><div class="note-section" style="flex:2"></div><div class="document-summary" style="flex:1">`…totals rows…`</div></div></div>`. When the block overflowed the bottom of the last line-item page, folio kept only "Subtotal" and dropped Volume Discount / Shipping / Tax / the grand **Total** — the PO's total amount silently disappeared. With the patch the flex reports it didn't fully fit, so the `page-break-inside:avoid` (KeepTogether) wrapper relocates the whole block to the next page, matching Chrome. PO v3: 2→3 pages with full totals (Chrome=3); SalesOrder/Invoice v3 page-parity + totals restored; non-fragmentable items (tall images/text) still force-render as before. Regression test: `fulcrum/flex_item_overflow_test.go`. |
| `v0.9.1-fulcrum.15` | An explicit `flex-basis: 0` was treated as `flex-basis: auto`. Row basis resolution gated on `effectiveBasis() > 0`, so a basis that resolved to 0 fell through to the content's max-content size. A `flex-basis: 0; flex-grow: N` item with long content then produced a huge bogus basis, overflowing the row, so items shrank by content weight instead of growing in their N:M proportions. | `layout/flex.go` (new `hasExplicitBasis` + `resolveItemBasis`; `resolveRowBasis` and `resolveGrowShrink` use them so an explicit 0 basis is honored, only `auto` falls back to max-content) | The .NET DocGen CAPA/NCR quality reports lay out a two-column data block: `.data-container { display: flex }`, `.data-container .left { flex-grow: 1; flex-basis: 0 }`, `.data-container .right { flex-grow: 2; flex-basis: 0 }` — a 1:2 (1/3 : 2/3) split, with long paragraphs (Problem Statement, Root Cause Analysis) in the right column. Pre-patch the left column collapsed to ~1/8 of the row and its text wrapped word-by-word; with the patch the columns land at the 1/3 : 2/3 boundary (folio right-column start x≈212 vs Chrome ≈213). Regression test: `fulcrum/flex_basis_zero_test.go`. |
| `v0.9.1-fulcrum.16` | `width: fit-content` (and `min-content` / `max-content`) was parsed as nil and treated as `width: auto`, so a box with a background and `width: fit-content` stretched to fill the containing block instead of hugging its content. | `html/css_props.go` (width Apply recognizes the content-sizing keywords → new `computedStyle.WidthFitContent`); `html/style.go` (field); `html/converter_block.go` (`applyDivStyles` calls `SetShrinkToFit(true)` when set); `docs/CSS_SUPPORT.md` (regenerated) | The .NET DocGen v3 header uses `.title-background { width: fit-content }` so the colored title plate hugs "Purchase Order" / "Invoice" / "Quote" etc. Pre-patch the plate stretched across the whole header column. Patch shrinks it to content, matching Chrome (folio title text width 169pt vs Chrome 165pt). folio approximates all three content keywords as shrink-to-fit. Regression tests: `fulcrum/width_fit_content_test.go`. |
| `v0.9.1-fulcrum.17` | Corrects fulcrum.14, which made a flex react to ANY non-full item status (`LayoutPartial` OR `LayoutNothing`) and return `LayoutPartial`. That fragmented page-by-page, and for an atomic item taller than a fixed-size page (the 7.5pt SVG icon on a 2.625in×1in item label) it never terminated — the icon never fit, the parent `Div` masked the no-progress as `LayoutPartial`-with-empty-box, and folio emitted an unbounded run of empty pages (each an expensive non-memoized `MaxWidth` re-layout). folio hung for minutes on `ItemLabel*`, then (after a first fix attempt) rendered them blank. | `layout/flex.go` (`planRow` sets the relocate flag ONLY for a `LayoutPartial` item — a genuinely fragmentable block with a remainder; a `LayoutNothing` item, i.e. an atomic icon/box that does not fit at all, is force-rendered in place exactly as before; when the flag is set the flex returns `LayoutNothing` so the renderer relocates it whole or, alone at page top, force-renders it — which terminates); `layout/div.go` (a `Div` that places nothing returns `LayoutNothing` rather than an empty `LayoutPartial`, so the relocate makes progress instead of emitting empty pages) | The fulcrum.14 win is preserved: the v3 no-break totals wrapper does not fit at the bottom of the last item page, its summary Div returns `LayoutPartial`, so the flex relocates the whole block to the next page — PO/SO/Invoice v3 keep all totals incl. the grand Total (PO v3 3 pages = Chrome). Item labels render their content again (icon clipped in place, matching pre-fragmentation output): `ItemLabelSmall` 14 words, 1 page. Found by sweeping every DocGen template through pdfdiff. Regression tests: `fulcrum/flex_item_overflow_test.go`, `fulcrum/flex_small_page_test.go`.
| `v0.9.1-fulcrum.18` | A run of consecutive sibling floats (a float-based column layout) all rendered at the container's left edge, stacked on top of each other (illegible). folio's float positioning offsets non-float content past a float but never offsets one float past another, so `.three-columns { float:left; width:33.3% }` columns drew at the same x. | `html/converter.go` (`walkChildren` groups a run of 2+ consecutive sibling floats and lays them out via `floatRunToRow` as a flex row — each float's CSS width becomes the flex item's basis; new `isFloatRunChild` helper. A single float is left as a real `layout.Float` so text still wraps around it.) | The .NET DocGen Certification (CoC) header lays Customer / Comments / Date out as three `.three-columns` floats inside `.wrap-container`; folio stacked all three at x=0, rendering the customer/contact info as an unreadable blob. Patch lays the run out as a flex row (reusing the hardened flex path incl. the flex-basis:0 fix), matching Chrome's side-by-side columns. Found via visual PDF review (cmd/pdfdiff diff.png) + an overlap detector. Regression test: `fulcrum/float_columns_test.go`. |
| `v0.9.1-fulcrum.19` | Row flex ignored item horizontal margins: `resolveGrowShrink` didn't reserve them and `computeJustifyOffsets` positioned items by content width alone. When the row fit there was incidental spacing, but when it overflowed, flex-shrink swallowed the inter-item gap. | `layout/flex.go` (`resolveGrowShrink` subtracts item left+right margins from free space — CSS Flexbox §9.7, margins never flex; `planRow` distributes on each item's OUTER width = content+margins and shifts each item by its own margin-left) | The v3 commerce PDF header (`.mirrored { display:flex }` with a `company-info` column carrying `margin-left:5px` + an `<hr class="mirror-line">` separator) collapsed to a ~0.7pt inter-column gap with the embedded Arial-metric font, so the two contact columns abutted and read as merged text ("Created ByAcme Industries"). Patch restores the gap (folio 8.3pt = Chrome 8.3pt) on Invoice/SalesOrder/Quote v3. Regression test: `fulcrum/flex_row_margin_test.go`. |
| `v0.9.1-fulcrum.20` | `flex-direction: row-reverse` / `column-reverse` were treated as plain row/column (no reversal), and `justify-content: right` / `left` fell through to flex-start. | `html/converter_flex.go` (reverse the child order after the `order` sort and flip justify-content start/end — the reversed-axis equivalence; map `right`→flex-end and `left`→flex-start) | The .NET DocGen v3 commerce header right-aligns its logo with `.logo.container { display:flex; flex-direction:row-reverse }`; folio left the ACME logo stuck against the Invoice title at the left of the header instead of top-right. Patch right-aligns it, matching Chrome. (Also fixes `.mirrored { justify-content:right }` to right-pack.) Regression test: `fulcrum/flex_direction_reverse_test.go`. |
| `v0.9.1-fulcrum.21` | (test-only) Fix the fulcrum.20 regression test: its `justify-content:right` assertion used `findTextX`, which only reads simple `Tj` shows and missed the `TJ`-array (kerned) word, reporting a false failure. Switched to the row-based `Td` scan. No code change — fulcrum.20's row-reverse / justify fixes are unchanged and verified. | `fulcrum/flex_direction_reverse_test.go` | — |

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
