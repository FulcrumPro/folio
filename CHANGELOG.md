# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Changed (breaking)

- **`html.Options.BasePath` removed; `BaseFS` is the single way to resolve local assets** — the v0.7.x compromise that kept both fields is gone. Callers pass any `fs.FS` (`embed.FS`, `os.DirFS(dir)`, `(*os.Root).FS()`, `fstest.MapFS`) and every local reference — `<img src>`, `<link href>`, `@font-face url()`, `background-image: url()`, and `Options.FallbackFontPath` — flows through it. Paths are normalised to `fs.FS` conventions before the read: forward slashes, no leading `/`, no `..` traversal. A leading `/` in document `src`/`href` is treated as web-style root-of-`BaseFS` (matching how `<base href="/">` works in browsers) instead of an absolute filesystem path. With `BaseFS` nil, every local-asset reference fails — the document is expected to inline its assets via `data:` URIs. The C ABI signature is unchanged: `folio_document_add_html_with_options` still takes a `basePath` C string and wraps it as `os.DirFS(basePath)` internally (#85)
- **`@font-face` URLs resolve relative to their containing stylesheet** — a linked `<link rel="stylesheet" href="css/site.css">` containing `@font-face { src: url(../fonts/Inter.ttf); }` now resolves to `fonts/Inter.ttf` from the `BaseFS` root, not from the document root. HTTP-origin stylesheets resolve relative URLs as HTTP, FS-origin stylesheets resolve them through `BaseFS`. Inline `<style>` blocks continue to resolve relative URLs from `BaseFS` root. Documents that previously relied on the root-anchored behavior need to use root-relative `/fonts/Inter.ttf` or move the `@font-face` rule into an inline `<style>`

### Added

- **`html.Options.Logger` (`*slog.Logger`)** — receives warn-level events when a local or remote asset fails to load: missing fonts in `@font-face`, unreadable linked stylesheets, image fetch errors that fall back to alt text. Defaults to nil (silent). Pair with `slog.NewTextHandler(os.Stderr, nil)` during development to surface what would otherwise be swallowed (#85)
- **`html.Options.Client` (`*http.Client`)** — HTTP client used for remote fetches (`<img>`, linked stylesheets, `@font-face url(http://...)`). Lets callers configure timeouts, transport, and proxies; mock the network in tests via `httptest.NewServer`; or share connection pools with surrounding code. Defaults to `http.DefaultClient` (#85)
- **`tmpl.RenderFile` / `tmpl.RenderFileTo` auto-populate `BaseFS`** — when the caller's `Options.BaseFS` is nil, the helpers default it to `os.DirFS(filepath.Dir(templatePath))` so that a template referencing `<img src="logo.png">` next to itself resolves without extra wiring
- **`html.Options.StrictAssets` (`bool`)** — promotes asset-load failures from warn-and-continue to returned errors. When true, `Convert` and `ConvertFull` collect every failed `@font-face url()`, `<img>`, `background-image: url()`, linked stylesheet, SVG load, and `FallbackFontPath`, then return them joined via `errors.Join` at the end of the conversion. The partial result (the elements that did render) is returned alongside the error so callers can inspect both. Errors are returned in document order — linked stylesheets, `@font-face` rules, then asset references in tree-walk order — and are byte-stable across runs given byte-identical input. Defaults to false — production keeps the warn-and-continue behavior. Use it in development and CI to surface broken asset paths in the local feedback loop instead of letting them silently degrade the output (#232)
- **`html.ErrURLPolicyDenied`** — sentinel returned when a `URLPolicy` callback rejects a fetch. The denial wraps it with `fmt.Errorf("%w: %w", ErrURLPolicyDenied, policyErr)` so callers can branch on `errors.Is(err, html.ErrURLPolicyDenied)` to distinguish "I told it to block" from "the asset broke." Under `StrictAssets`, `ErrURLPolicyDenied` is logged through `Options.Logger` but excluded from the joined return error — the caller already received the signal they wired the policy to produce (#232)

### Fixed

- **SVG `preserveAspectRatio` slice viewport clip** — when an SVG is drawn with `slice` meet-or-slice, the renderer now emits a PDF clip path on the target rectangle before the viewport transform. Previously the uniform scale was applied correctly but content outside the target rectangle leaked onto the page. Callers that already clipped externally will continue to work (#196)
- **TrueType Collection (`.ttc`) fonts now load** — `font.ParseFont` and `font.LoadFont` advertised TTC support but routed the bytes to `sfnt.Parse`, which rejects collections with `invalid single font (data is a font collection)`. The dispatch now extracts face 0 from the collection into a standalone single-font TTF (table directory rewritten to point at the new offsets) and parses that. Selection of face 0 matches browser behavior for `url()` references without a `#` fragment. Very large CJK collections may still hit `golang.org/x/image/font/sfnt`'s hardcoded `maxCmapSegments` limit — that is an upstream parser limit, orthogonal to TTC dispatch (#227)

## [0.7.1] - 2026-04-22

C ABI follow-up to v0.7.0. No Go-side behavior changes; every addition is in `export/`. C ABI exports grow from 372 to 388 (+16). The header `export/folio.h` is updated in lockstep — `scripts/audit-cabi.sh` reports Go and header in sync.

### Added

#### Writer optimizer (C ABI)

Handle-based builder so future toggles compose without renaming existing calls. The save and buffer entry points accept a zero handle as "use defaults", so callers that only want the optimizer for a single write do not need to allocate and free an options object.

- **`folio_write_options_new`** / **`folio_write_options_free`**
- **`folio_write_options_set_use_xref_stream`** — ISO 32000-1 §7.5.8
- **`folio_write_options_set_use_object_streams`** — §7.5.7
- **`folio_write_options_set_object_stream_capacity`**
- **`folio_write_options_set_orphan_sweep`**
- **`folio_write_options_set_clean_content_streams`** — §7.8
- **`folio_write_options_set_deduplicate_objects`**
- **`folio_write_options_set_recompress_streams`** — §7.4.4
- **`folio_document_save_with_options`**
- **`folio_document_write_to_buffer_with_options`**

#### Document and per-element setters (C ABI)

- **`folio_document_set_actual_text`** — toggles the `/Span /ActualText` emission for shaped Arabic words (ISO 32000-2 §14.9.4)
- **`folio_paragraph_set_direction`** — `FOLIO_DIR_AUTO` (0) / `FOLIO_DIR_LTR` (1) / `FOLIO_DIR_RTL` (2). Out-of-range values normalize to auto
- **`folio_list_set_direction`**
- **`folio_table_set_direction`**
- **`folio_columns_set_balanced`**

#### Header

- **`FOLIO_DIR_AUTO`, `FOLIO_DIR_LTR`, `FOLIO_DIR_RTL`** preprocessor constants in `export/folio.h` so C consumers do not encode magic numbers for the direction setters

### Test plan

`export/testdata/test_cabi.c` gains 21 new assertions covering: `WriteOptions` lifecycle, all seven setters (happy path and bad-handle rejection), zero-options default path, an end-to-end document write with every optimizer toggle on that confirms the output starts with `%PDF-`, `folio_document_set_actual_text` happy path plus bad-handle, direction setters on Paragraph / List / Table (happy path, normalization of out-of-range codes, bad handle), `folio_columns_set_balanced` happy path and bad-handle. Total `test_cabi` assertions: 390 (384 pre-existing + 6 new blocks).

### Not exposed

Internal-only v0.7.0 additions remain unexported from the C ABI: `core.PdfIndirectReference.SetNum` and `core.PdfStream.WillCompress` (writer-internal); `core.DeflateStreamData` / `core.InflateStreamData` (callers can use any host-language zlib); `core.PdfArray` / `PdfDictionary` / `PdfNumber` / `PdfBoolean` / `PdfString` Go-style accessors; `font.ParseGPOS` / `ParseGSUB` / `ParseKern` and `face.GSUB` / `GPOS` / `GIDToUnicode` (font parser internals); `font.CanEncodeWinAnsiRune`, `EmbeddedFont.EncodeGIDs` / `MeasureGIDs` (shaper-internal); `layout.ShapeArabic` / `ShapeArabicWithFont` / `ShapeDevanagari*`, `ScriptOf` / `SegmentByScript`, `GraphemeBreaks` / `NextGraphemeBreak` / `GraphemeCount`, `FindKashidaCandidates` / `InsertKashidas` (run inside the layout pipeline); `layout.GSUBProvider` / `GPOSProvider` (Go interfaces); `tmpl` package (Go-specific templating). RTL and shaping for HTML-driven workflows continue to flow through `folio_document_add_html` unchanged.
## [0.7.0] - 2026-04-21

No breaking API changes. Every new field, method, and package is additive; zero-value `WriteOptions` and existing constructors produce byte-identical output to v0.6.2. Several bug fixes change the visible output of affected documents — see Visual changes before regression-diffing PDFs.

### Visual changes

- **CSS multi-column fill order** — children are distributed sequentially with height-balanced packing instead of round-robin by index. Matches CSS Multi-column Layout `column-fill: balance` (the spec default). Documents using `column-count` will reflow (#145)
- **Arabic text shaping** — the layout engine uses the font's OpenType GSUB contextual substitutions (init/medi/fina/isol) when present and falls back to the legacy Arabic Presentation Forms-B substitutions only when the font lacks GSUB. v0.6.x rendered Arabic as disconnected isolated forms; v0.7.0 renders connected (#160)
- **TrueType kerning** — fonts whose `kern` table uses Apple-format v0 coverage (Arial and many others) previously returned zero kern for every pair due to a flipped coverage-byte decode. Pairs are now applied; spacing in affected documents tightens (#172)
- **CJK line-breaking** — Japanese, Chinese, and Korean paragraphs break per JIS X 4051 kinsoku shori rules instead of arbitrary character boundaries (#157)
- **Per-glyph font fallback** — runes outside the primary font's coverage previously rendered as `.notdef` boxes; the layout engine selects a fallback face per glyph cluster when one is configured
- **CSS `!important` cascade** at the inline/stylesheet boundary is now honored (#137)
- **`/ActualText` markers around shaped Arabic words** are emitted by default. Adds a small per-word byte cost and improves copy/paste fidelity in PDF readers. Opt out with `Document.SetActualText(false)`

### Deprecated

These remain fully functional in v0.7.0. Plan to migrate before v1.0, when the stable API will be declared and deprecated symbols removed.

- **`core.PdfDictionary.Entries`** direct field access — use `All`, `Get`, `Set`, `Remove`. Direct slice mutation bypasses the lazy key index
- **`core.PdfArray.Elements`** direct field access — use `All`, `At`, `Len`, `Add`, `Set`, `RemoveAt`, `Replace`
- **`core.PdfIndirectReference.ObjectNumber`** — use `Num` for reads and `SetNum` for writes
- **`core.PdfIndirectReference.GenerationNumber`** — use `Gen`

### Discouraged (security)

- **`core.RevisionRC4128`** — RC4 is cryptographically broken; use `RevisionAES256` for new documents. The constant remains supported for reading and writing legacy RC4-encrypted PDFs and is not scheduled for removal

### Added

#### Internationalization

- **Right-to-left text** — bidi paragraph layout per UAX #9 via `golang.org/x/text/unicode/bidi`, character-level bidi splitting, RTL list support (markers on right, text indented from right), HTML `dir` attribute and CSS `direction` property wiring (#37)
- **Arabic shaping** — presentation-forms shaper with full GSUB pipeline for init/medi/fina/isol features, GSUB ligature substitutions (`rlig`, `liga`), kashida (tatweel) justification, `/ActualText` markers (default on) for round-trip-safe copy/paste of shaped words (#37, #160, #179, #180)
- **Devanagari shaping** — Indic shaper for Hindi, Sanskrit, Marathi, and Nepali. Five-phase OpenType pipeline with reordering, half-form substitution, and conjunct formation (#186)
- **CJK line-breaking** — kinsoku shori rules per JIS X 4051 with leading and trailing prohibition sets (#157)
- **Unicode infrastructure** — UAX #29 grapheme clusters (`unicode/grapheme` package), UAX #24 script segmentation, cluster-aware `font.MeasureString` (#170, #176, #183)

#### OpenType infrastructure

- **GSUB LookupType 4** ligature substitutions (#171)
- **GSUB LookupType 6** chaining contextual substitution with depth-bounded action recursion (#174, #184)
- **GPOS LookupType 2** pair adjustment (Format 1 explicit pairs and Format 2 class-based pairs); `face.Kern` consults GPOS first and falls back to the legacy `kern` table (#175)
- **GPOS LookupType 4** mark-to-base anchoring with draw-pipeline wiring; correct diacritic placement for Arabic and Devanagari (#175, #185)
- **TrueType `kern` table** parser hardening: correct v0 / v1 coverage decode, per-face cache, kerning-aware `MeasureString` for both `Standard` and `EmbeddedFont` (#172)

#### Writer optimizer

The `WriteOptions` struct is the single extension point for opt-in writer behavior. Zero value preserves byte-identical output to v0.6.2. Pass dispatch order: orphan sweep → content-stream cleanup → object dedup → stream recompression → encryption → serialization.

- **`document.WriteOptions`** with `Writer.WriteToWithOptions`, `Document.WriteToWithOptions`, `Document.SaveWithOptions`, `Document.ToBytesWithOptions`
- **`UseXRefStream`** — cross-reference stream object (ISO 32000-1 §7.5.8) replacing the traditional xref table and trailer. The xref stream is always written as the last indirect object so its own offset is known before serialization
- **`UseObjectStreams`** — packs eligible indirect objects into compressed object streams (§7.5.7). Implies `UseXRefStream`. Refused on encrypted documents
- **`ObjectStreamCapacity`** — cap on objects packed per `/ObjStm` (default 100)
- **`OrphanSweep`** — drops indirect objects unreachable from `/Root`, `/Info`, `/Encrypt`; renumbers survivors contiguously
- **`RecompressStreams`** — re-Flates eligible payloads at `zlib.BestCompression`, gated by a size-regression guard. Skips DCT/JPX/CCITT/JBIG2 leaf filters, multi-filter chains, and FlateDecode streams carrying `/DecodeParms` (predictor handling per §7.4.4.4)
- **`DeduplicateObjects`** — merges byte-identical indirect objects via SHA-256 of canonical serialization; rewrites references to the canonical survivor; renumbers contiguously. Excludes catalog, `/Info`, `/Encrypt`
- **`CleanContentStreams`** — removes empty `q`...`Q` save/restore pairs and identity `1 0 0 1 0 0 cm` operators (§7.8) from page content streams. Includes a byte-level lexer respecting strings, hex strings, comments, names, arrays, dictionaries, and inline images (BI/ID/EI per §8.9.7)
- **`core.PdfIndirectReference.SetNum`** — object-number setter used by writer-side renumbering passes
- **`core.PdfStream.WillCompress`** — getter matching the existing `SetCompress` setter; lets writer-side passes skip streams the writer will already deflate
- **`core.DeflateStreamData` / `core.InflateStreamData`** — exported zlib codec helpers (formerly the unexported `deflate`)
- **`examples/optimize`** — runnable demo with four fixtures (text-heavy, many empty pages, table-heavy, imported text-heavy) reporting byte-size deltas across the optimizer toggles. Imported fixture: 40,569 → 6,071 bytes (85.0% saved) with the full stack

#### Templates and HTML

- **`tmpl` package** — `html/template` integration: parse a template, execute against caller data, feed the result to the HTML converter (#155)
- **`@font-face` with `src: url(data:font/...)`** base64 data URI fonts (#159)

#### Examples

- **`examples/indic`** — Hindi, Sanskrit, Marathi, Nepali samples (#191)
- **`examples/rtl`** — expanded with full Arabic shaping showcase (#191)
- **`examples/optimize`** — multi-fixture optimizer comparison (#177, #181)
- **Stress tests** for column, grid, flexbox, and SVG layouts (#126, #153)

### Fixed

- **`<br>` inside `<strong>`, `<em>`, `<a>`, and other inline elements** — previously panicked. All 14 inline tags now route line breaks correctly through the `TextRun.IsLineBreak` field (#147, #150)
- **Multi-line headings** no longer overprint wrapped lines (#132)
- **`column-span: all` inside multi-column containers** — correct break handling at leading, trailing, consecutive, and column-boundary positions (#127)
- **Heading overflow tag and `Consumed` accounting on page break** (#139)
- **Gradient `stop-opacity`** plumbed through `layout.GradientStop` to the rasterizer (#146)
- **JPEG parser out-of-bounds** found by fuzz testing (#169)
- **CSS Custom property `var()` references** in `align-items` (#128)

### Audits

- **`core` package** — correctness fixes, hardening, gradual API cleanup; sentinel errors for parser failures (#165)
- **`content` package** — operator validation against ISO 32000 operator set; coverage to 100% on operator emission paths (#166)
- **`image` package** — limits on dimension and component count; fuzz suite for JPEG, PNG, TIFF; CMYK JPEG via Adobe APP14 (#167, #169, #173)
- **`font` package** — sentinel errors, documented concurrency contract, coverage on subset and cmap paths (#168, #173)
- **SVG `preserveAspectRatio`** parsed and honored; barcode fuzz suite (#178)

### Changed

- **Go module dependencies**:
  - `golang.org/x/image` 0.38.0 → 0.39.0
  - `golang.org/x/net` 0.52.0 → 0.53.0
  - Added `golang.org/x/text/unicode/bidi` for UAX #9 bidirectional text
- **`softprops/action-gh-release`** v2 → v3 (release CI)
- **Internal**: `ARCHITECTURE.md` updated with the bidi dependency entry; `font/kern.go` extracted from `truetype.go`; GSUB extracted from the `Face` interface into an optional `GSUBProvider`; `unicode/grapheme` extracted as a leaf package
- **READMEs**: end-to-end HTML-to-PDF benchmarks and performance section; Language SDKs section pointing at Java and WASM ports

### Contributors

- **David Richardson** ([@enquora](https://github.com/enquora)) — stress-test contributions for column, grid, flexbox, SVG layouts (#126)

## [0.6.2] - 2026-04-08

No breaking API changes — all additions below are additive. `layout.Grid.SetAlignContent` keeps its signature; it now also records that the value was explicitly set so implicit "normal" stretching is preserved. The C ABI grows from 348 to 372 exports, all additive.

### Added

- **SVG `<image>` elements with data-URI raster sources** — `<image href="data:image/png;base64,...">` inside an `<svg>` now decodes and draws the embedded PNG/JPEG/WebP/GIF. Missing `width`/`height` fall back to the raster's intrinsic pixel dimensions (#130)
- **Real SVG gradients** — `linearGradient` and `radialGradient` referenced via `fill="url(#id)"` are now rasterized and drawn clipped to the shape instead of collapsing to the first stop color (#130)
- **`svg.RenderOptions.RegisterImage`** — new callback for wiring an external image decoder and XObject registrar from SVG `<image>` elements, keeping the svg package free of image-format dependencies (#130)
- **`svg.RenderOptions.RegisterGradient`** — new callback for rasterizing SVG gradients into PDF image XObjects; nil callback preserves the legacy first-stop fallback (#130)
- **`svg.Node.LinearGradient()` / `svg.Node.RadialGradient()`** — accessors returning parsed gradient definitions (`LinearGradientInfo` / `RadialGradientInfo`) with resolved `Stop` values, so callers can drive an external rasterizer without reparsing SVG syntax (#130)
- **`svg.BBox`** — new exported type representing an axis-aligned bounding box in SVG local coordinates, used as the shape bounding box passed to `RegisterGradient` (#130)
- **24 new C ABI exports** — `folio_document_to_bytes`, `folio_document_validate_pdfa`, Div extras (`set_aspect_ratio`, `set_keep_together`, `set_border_radius_per_corner`, `set_width_percent`, `set_hcenter`, `set_hright`, `set_clear`, `set_outline`, `add_box_shadow`), Cell extras (`set_border_radius`, `set_border_radius_per_corner`), Grid extras (`set_border`, `set_borders`, `set_template_areas`), Flex extras (`set_align_content`, `set_borders`), `paragraph_set_text_align_last`, Image extras (`set_object_fit`, `set_object_position`), `run_list_last_set_background_color`, `signer_new_pkcs12`, `parse_css_length`. Total C ABI: 372 exports, up from 348 (#143)

### Fixed

- **Inline `<svg>` / `<img>` dropped from paragraph flow** — replaced elements inside a `<p>` inherited `display: block` from the parent and were silently skipped by the inline collector. They now default to `display: inline` in `applyTagDefaults`, matching browser replaced-element behavior. Affects any `<p>` containing a bare `<svg>` or `<img>` without an explicit inline `display` override (#130)
- **Inline `<strong>` / `<em>` / `<span>` / `<a>` split paragraphs into three** — `<div>text <strong>bold</strong> more text</div>` produced three stacked paragraphs with the trailing punctuation orphaned on a new line. Two interacting bugs: text-emphasis tags (`strong`, `em`, `span`, `b`, `i`, `u`, `s`, `del`, `mark`, `small`, `sub`, `sup`, `code`, `a`) inherited `display: block`, and `walkChildren` didn't group consecutive inline siblings into anonymous block boxes per CSS 2.1 §9.2.1.1. `walkChildren` now buffers consecutive inline flow children and flushes them as a single paragraph (#142)
- **`<td style="width:50%">` overflow in narrow flex columns** — cell width percentages were resolved against the converter's outer container width instead of the table's actual layout width, producing absolute hints much larger than the table could hold and cells that ran off the page. Percentage widths are now deferred to layout time where the real table width is known (#142)
- **CSS Grid `align-items`, `justify-items`, and container height** — explicit `height` on a grid container is now honored so `align-items` / `align-content` have room to distribute; `justify-items` values (`start`/`end`/`center`/`stretch`) are applied; explicit `align-content: flex-start` no longer triggers row stretching (#129)
- **Flexbox `order` property** — children are stable-sorted by their CSS `order` value before layout; ties preserve DOM order per Flexbox spec. `align-items` now resolves `var(--custom-property)` references through the cascade (#128)
- **Table sizing preserved across page-break continuations** — `min-width`, auto-column widths with cell hints, and `border-collapse`/`border-spacing` now survive when a table splits across pages (#134)
- **Multi-line headings no longer overprint wrapped lines** — `Heading.PlanLayout` allocates per-line boxes so long headings that wrap at narrow widths render cleanly without glyph overlap (#132)
- **`column-span: all` inside multi-column containers** — elements with `column-span: all` now break the column flow correctly at leading, trailing, consecutive, and column-boundary positions, including when `column-rule` is set (#127)
- **Heading overflow tag + `Consumed` accounting on page break** — headings that split across pages carry the correct structure tag on continuation and no longer over-advance the available height (#139)

## [0.6.1] - 2026-04-05

### Added

- **CSS `aspect-ratio`** property on Div elements (CSS Sizing Level 4 §5.1) — derives height from width when no explicit height is set; supports `16 / 9`, `auto 16/9`, single number forms (#112)
- **CSS Color Level 4** space-separated `rgb()`/`hsl()` syntax — `rgb(255 0 0 / 0.5)`, percentage alpha, applies to `rgba()`/`hsla()` too (#108)
- **`html.ParseCSSLength`** public utility — converts CSS length strings (`"1in"`, `"16px"`, `"2em"`, `"50%"`, `calc()`) to PDF points (#109)
- **`Document.ToBytes`** convenience method — returns serialized PDF as `[]byte` for HTTP responses, base64 encoding, in-memory processing (#66)
- **Per-side border-radius on table cells** — `drawCellBordersRounded` now draws each border side independently with corner arcs, instead of requiring all four borders to be identical (#115)
- **WASM header/footer** — `folioRender` accepts `headerHtml`/`footerHtml` in settings JSON, rendered via `SetHeaderElement`/`SetFooterElement` (#102)
- **Invoice example** (`examples/invoice/`) — professional invoice PDF demonstrating rounded table headers, CSS Grid, Flexbox, and optional Tailwind CSS v2

### Fixed

- **Nil font panic** in `runMeasurer` — falls back to Helvetica when `TextRun` has no font set (#98)
- **Table default `border-collapse`** changed from `collapse` to `separate` per CSS 2.1 §17.6 — previously prevented cell border-radius from rendering (#114)
- **Table default margins** removed — browsers set zero margins on `<table>`; added `border-spacing: 2px` default per browser UA stylesheets (#117)
- **Div `drawRoundedBorders`** now uses per-corner radii (`RoundedRectPerCorner`) instead of uniform radius; previously only `r[0]` was used (#104)
- **`:not([hidden])` selector** — attribute selectors inside pseudo-class parens were incorrectly extracted, leaving empty `:not()` that always returned false; enables CSS framework `space-y-*` utilities (#101)
- **`rem` unit parsing** — `parsePlainLength` checked `"em"` suffix before `"rem"`, so `"1rem"` failed to parse (#111)
- **Table cell border-radius in HTML** — converter now skips radius wiring in `border-collapse: collapse` mode per CSS Backgrounds Level 3 §5.3 (#100)
- **README Go version** corrected from 1.21+ to 1.25+ to match go.mod (#124)

### Visual change

- **Table borders now render in `separate` mode by default** (previously `collapse`). Tables without explicit `border-collapse: collapse` in CSS will show individual cell borders instead of shared borders. This matches browser behavior per CSS 2.1 §17.6. To restore the old behavior, add `table { border-collapse: collapse; }` to your CSS.

### Changed

- **Layout test coverage** 70% → 77.9% — 40 new integration tests for draw functions, table rendering, Div features, Grid layout, Flex column, paragraph indent/ellipsis/orphans
- **Playground URL** updated to `playground.foliopdf.dev`

## [0.6.0] - 2026-04-03

**Breaking changes** — see [MIGRATING.md](MIGRATING.md) for upgrade steps.

### Breaking

- Renamed constructors to `New*`/`Load*`/`Parse*` across `reader`, `barcode`, `layout`, `sign`, `forms`
- `sign.LoadPKCS12` renamed to `sign.ParsePKCS12` (same signature, name-only change)
- `Document.Page(index)` returns `(*Page, error)` instead of panicking
- Unexported internal symbols in `reader` and `svg`
- Baseline positioning uses CSS half-leading with actual font metrics (visual change — text shifts up ~4pt for 12pt Helvetica)
- `vertical-align` accepts length/percentage values (previously ignored)

### Added

- Element-based headers/footers: `SetHeaderElement`, `SetFooterElement`, `SetHeaderText`, `SetFooterText`
- `AddHTMLTemplate` / `AddHTMLTemplateFuncs` for Go template → PDF
- `ValidatePdfA` for early PDF/A validation
- Per-run text highlight (`WithBackgroundColor`, `<mark>` in HTML)
- Inline elements in paragraphs (`<img>`, `<svg>`, `display:inline-block`)
- `<sub>` and `<sup>` rendering with baseline shift and correct spacing
- `baseline-shift` CSS property (keywords and lengths/percentages)
- `vertical-align` extended with length/percentage values per CSS 2.1
- Empty lines from consecutive `\n\n` in paragraphs
- `RunInline` for inline layout elements within paragraphs
- CSS: `text-align-last`, `::marker`, `cmyk()`, `object-fit`, `@supports`, `min()`/`max()`/`clamp()`, `:is()`/`:where()`, repeating gradients, `column-width`/`column-rule`, `string-set`/`string()`, `page-break-inside: avoid`, escape sequences in selectors, multiple `box-shadow`
- WebP and GIF image formats

### Fixed

- `<sub>`/`<sup>` baseline shift — previously only reduced font size (#86)
- Adjacent styled runs no longer insert spurious spaces; inline whitespace collapsing per CSS Text Level 3 §4.1.1 (#86)
- Punctuation after `</sup>`/`</sub>` keeps correct styling (#86)
- Punctuation at font boundaries keeps its own font (#30)
- Consecutive `\n\n` produce visible empty lines (#91)
- Blank lines preserved across page splits (#95)
- Paragraph baseline uses CSS half-leading `(lineH + ascent - descent) / 2` (#90)
- `cloneWithWords` preserves all Word styling + line breaks on page-split paragraphs
- Style changes at line break boundaries preserved during page splits
- Overflow handling includes following siblings in Div layout (#13)
- Table layout handles zero/negative height without panicking
- Inline-block SVG/IMG dispatch to correct converters (#71)
- Inline element alignment: line-relative child positions (#71)
- `buildParagraphFromRuns` always uses `NewStyledParagraph` (was dropping `BaselineShift`/`BackgroundColor`)
- RGB color components clamped to 0–1
- Case-insensitive attribute selector matching
- Alpha premultiplication fix for PNG
- Font descriptor flags from actual metadata
- Kern format 0 nPairs validated
- Encrypted PDFs detected with clear error
- Signatures preserved on multi-sign PDFs
- Highlight/underline/strikethrough use actual font metrics (#73)
- Predictor column count bounded to prevent allocation DoS

### Contributors

- **Ben Davidson** ([@bendavidsonku](https://github.com/bendavidsonku)) — inline elements in paragraphs, per-run text highlight background (#71, #72)
- **Jason Kulatunga** ([@AnalogJ](https://github.com/AnalogJ)) — table zero-height fix, overflow sibling handling (#13)

### Changed

- `golang.org/x/image` v0.37.0 → v0.38.0
- Internal: `html/converter.go` split into focused modules
- Internal: `ARCHITECTURE.md` with design principles, layering rules, naming conventions

## [0.5.2] - 2026-03-26

### Added
- **PDF redaction** — `RedactText`, `RedactPattern`, `RedactRegions` permanently remove text from content streams with character-level TJ splitting precision; configurable fill color, overlay text, and metadata stripping (#59)
- **Page import** — `Page.ImportPage` and `Page.ImportPageWithOpts` load existing PDF pages as Form XObjects (ISO 32000 §8.10) for template workflows; `reader.ExtractPageImport` convenience API with full indirect-ref resolution (#47)
- **Drawing primitives** — `Page.AddLine`, `Page.AddRect`, `Page.AddRectFilled` for low-level graphics on pages
- **PDF/UA accessibility** — alt text for images, custom structure tags, structure tree reading from existing PDFs (#60)
- **Paragraph `\n` line breaks** — `\n`, `\r\n`, and `\r` now produce forced line breaks in paragraphs and table cells (#61, #63)
- **C ABI expanded to 346 functions** (up from 330) — adds redaction, page import, drawing, digital signatures, encryption permissions, page manipulation, content extraction, form flattening, merge, TextRun builder, styled list/heading exports
- **Examples** — `merge/` (parse, merge, extract text), `sign/` (PAdES B-B digital signature), `report/` (multi-page layout API), `import-page/` (external PDF template filling), `redact/` (sensitive text removal)

### Fixed
- **Import page blank output** — `resolveDeep` recursively resolves all indirect references in imported resources; `hoistStreams` converts nested PdfStream objects to indirect refs, fixing blank/partial output with real-world PDFs
- **Paragraph `\n` collapsed to space** — `splitWords` now splits on newlines first and inserts forced line break markers

## [0.5.1] - 2026-03-25

### Fixed
- **Release workflow** — replaced deprecated `macos-13` runner with `macos-latest` for x86_64 builds
- **Fuzz test regex** — anchored `-fuzz='^FuzzParse$'` to avoid matching `FuzzParsePDF`

## [0.5.0] - 2026-03-25

### Contributors

- **Marc Ole Bulling** — `<br>` nil pointer fix (#10)
- **Moritz** ([@FrauElster](https://github.com/FrauElster)) — PDF/A-3b file attachments (#17)
- **Piotr Pawlak** ([@piotrxp](https://github.com/piotrxp)) — SSRF prevention for remote resources (#39)

### Added
- **C ABI expanded to 281 functions** (up from 115) — covers nearly all Go engine features
- **Barcode C ABI** — `folio_barcode_qr`, `qr_ecc`, `code128`, `ean13` + layout elements
- **SVG C ABI** — `folio_svg_parse`, `parse_bytes` + layout elements with size/align
- **Link C ABI** — hyperlink, embedded font, and internal link layout elements
- **Flex C ABI** — full flexbox container with items, direction, justify, align, wrap, gap, borders
- **Grid C ABI** — CSS Grid with template columns/rows, auto-rows, placement, justify/align items/content
- **Columns C ABI** — multi-column layout with gap and custom widths
- **Float C ABI** — left/right floating elements with margin
- **TabbedLine C ABI** — tab-stop text with dot leaders for TOC-style layouts
- **Form filling C ABI** — `folio_form_filler_new`, `set_value`, `set_checkbox`, `field_names`, `get_value`
- **Form field builder C ABI** — `folio_form_create_text_field`, `create_checkbox` + `set_value`, `set_read_only`, `set_required`, `set_background_color`, `set_border_color`, then `add_field`
- **Additional form fields** — multiline text, password, listbox, radio group
- **Document watermark** — `folio_document_set_watermark` and `set_watermark_config`
- **Outlines/bookmarks C ABI** — `folio_document_add_outline`, `add_outline_xyz`, `outline_add_child`
- **Named destinations** — `folio_document_add_named_dest`
- **Viewer preferences** — `folio_document_set_viewer_preferences`
- **Page labels** — `folio_document_add_page_label`
- **File attachments** — `folio_document_attach_file` for PDF/A-3b compliance
- **Inline HTML** — `folio_document_add_html` and `add_html_with_options`
- **Page-specific margins** — `folio_document_set_first_margins`, `set_left_margins`, `set_right_margins`
- **Absolute positioning** — `folio_document_add_absolute`
- **Page extensions** — art box, page size override, page-to-page links, text annotations, text markup annotations (highlight, underline, squiggly, strikeout), separate fill/stroke opacity
- **All 14 standard font accessors** — added `helvetica_oblique`, `helvetica_bold_oblique`, `times_italic`, `times_bold_italic`, `courier_bold`, `courier_oblique`, `courier_bold_oblique`, `symbol`, `zapf_dingbats`
- **Paragraph extensions** — orphans, widows, ellipsis, word-break, hyphens
- **Table extensions** — footer rows, cell spacing, auto column widths, min width
- **Cell extensions** — per-side padding, vertical alignment, borders, width hints
- **Div extensions** — border radius, opacity, overflow, max/min width, box shadow, max height
- **List extensions** — leading, nested sub-lists
- **Image extensions** — TIFF loading, element alignment
- **C ABI audit script** (`scripts/audit-cabi.sh`) — detects drift between Go exports, folio.h, and built symbols
- **C integration tests** — 258 tests covering all exported functions

### Changed
- **Library version injected at build time** via `-ldflags "-X main.version=..."` — `folio_version()` returns the git tag in releases, `git describe` in dev builds
- **CI** — added C ABI audit, shared library build, and C integration test steps
- **Release workflow** — builds native shared libraries for 5 platforms (linux-x86_64, linux-aarch64, macos-x86_64, macos-aarch64, windows-x86_64) with SHA256 checksums
- **Makefile** — added `audit`, `audit-build`, cross-compilation targets (`cross-linux-amd64`, etc.), OS-aware shared library extension detection

### Fixed
- **`<br>` nil pointer** — fixed nil pointer exception with `<br>` tags (#10)
- **PDF/A-3b file attachments** — proper embedded file streams with `/AF`, `/Names`, MIME types (#17)
- **ZUGFeRD lint** — deterministic timestamps, fixed example (#41)
- **Content stream compression** — FlateDecode for content streams, merge stream dict bug (#44)
- **SSRF prevention** — URL policy interceptor for blocking/modifying remote resource requests (#39)
- **Radio button / checkbox appearance** — fixed appearance stream generation for form fields
- **`:root` selector** — now correctly matches the `<html>` element
- **Gradient rendering** — fixed CSS gradient parsing and rendering
- **Page number CSS counter** — `counter(page)` and `counter(pages)` now work in margin boxes
- **MarginBox API** — exposed `MarginBoxes`/`FirstMarginBoxes` on `ConvertResult` for simpler programmatic access (#54)

## [0.4.2] - 2026-03-22

### Added
- **Clickable links in PDFs** — `<a href="...">` inside paragraphs, headings, and list items now produce PDF link annotations (#23, #26, #27)
- **Multiple links per line** — paragraphs with several inline links each get their own precise annotation rectangle
- **Internal document links** — `layout.NewInternalLink` resolves to direct page references for macOS Preview compatibility
- **Layout API link support** — `TextRun.WithLinkURI()` and `WithDecoration()` for building linked text programmatically; `List.AddItemRuns()` for linked list items
- **Links example** (`examples/links/`) showcasing external, inline, multi-line, styled, heading, and list item links plus bookmarks and internal navigation
- **Fonts example** (`examples/fonts/`) demonstrating custom `@font-face` with Unicode (CJK, Cyrillic, Japanese)

### Fixed
- **Custom `@font-face` family names ignored** — `parseFontFamily` was mapping all names to standard fonts; now preserves custom names for embedded font matching (#16)
- **`page-break-after` ignored when body has `width: 100%`** — `AreaBreak` elements trapped inside Div wrappers are now hoisted out so the renderer can act on them (#21)
- **CSS class selectors case-insensitive** — `.myClass` now matches `class="myClass"` regardless of case (#28)
- **Punctuation spacing at run boundaries** — period/comma after a styled span (e.g. `<b>word</b>.`) no longer gets an extra inter-word space (#25)
- **Underline continuous across multi-word links** — decoration extends through trailing spaces between consecutive linked/decorated words
- **`@font-face` family name case mismatch** — font-face names are now lowercased consistently so CSS lookup matches

### Changed
- **Split `html/converter.go`** into 11 focused files by responsibility (paragraph, table, block, flex, forms, image, list, heading, link, style, helpers) — no behavior changes (#34)

## [0.4.1] - 2026-03-22

### Contributors

- **Emrecan BATI** — Apache 2.0 license text cleanup

### Added
- **Comprehensive GoDoc comments** across all 14 packages — every exported and unexported symbol now has an accurate doc comment following Go conventions
- **Package-level doc comments** added to `layout`, `html`, `svg`, and consolidated in `core` (had two conflicting comments)
- **`ARCHITECTURE.md`** documenting design principles, package responsibilities, layering rules, dependency policy, and non-goals
- **Examples directory** with hello-world sample

### Fixed
- Stale/inaccurate doc comments: watermark "prepends" → "appends", `dss.Build` return description, `PdfObjects` → `BuildObjects`, merged interface docs in layout, and others
- `cmd/folio printUsage` referenced nonexistent "region" extraction strategy
- `svg/doc.go` listed nonexistent `clipPath` support, missing `radialGradient`
- `layout/doc.go` referenced wrong type names (`Tab` → `TabbedLine`, nonexistent `Transform` element)
- `font/standard.go` package doc was stale ("and later font parsing" — parsing is fully implemented)

### Changed
- Removed committed `folio.wasm` binary (7.2MB) from the repository; the release workflow already builds it fresh per tagged version
- Added `*.wasm` to `.gitignore`
- golangci-lint issues fixed and linter added to CI
- Apache 2.0 license replaced with verbatim text; missing license headers added

## [0.4.0] - 2026-03-19

### Added
- **WOFF1 font decoding** — `@font-face` now supports `.woff` files via automatic format detection (`font.LoadFont`)
- **CSS custom properties (variables)** — `--name: value` declarations with `var(--name, fallback)` resolution, inheritance, and nesting
- **CSS counters** — `counter-reset`, `counter-increment`, `counter()` and `counters()` in `::before`/`::after` content
- **CSS `clear` property** — `clear: left/right/both` advances past active floats before placing elements
- **CSS `border-spacing`** — horizontal and vertical cell spacing for tables in separate border model
- **HTTP background images** — `background-image: url('https://...')` fetches remote images in non-WASM builds
- **Inline-block in text flow** — `display: inline-block` elements flow within paragraphs as "big words" with correct line-breaking and height expansion
- **Containing-block absolute positioning** — `position: absolute` resolves against nearest positioned ancestor via overlay children, not just the page
- **Liang-Knuth hyphenation** — 4938 TeX US English patterns for linguistically correct syllable breaks, replacing geometric character splitting
- **C ABI export layer** — `export/` package for FFI from Python, Ruby, Swift, etc.
- **Full sRGB ICC profile** and PDF/A-1b compliance support
- **QR code v1-40** with numeric/alphanumeric encoding modes
- **Symbol and ZapfDingbats** font width tables

### Changed
- `font.LoadTTF` calls in HTML converter replaced with `font.LoadFont` (auto-detects TTF/OTF/WOFF)
- `hyphenateWord()` uses pattern-based breaks first, falls back to character splitting

## [0.3.0] - 2026-03-18

### Added
- **CSS Grid layout** — `display: grid` with `grid-template-columns`, `grid-template-rows`, `grid-template-areas`, named areas, `auto-rows`, alignment (`justify-items`, `align-items`), and page break support
- **Absolute positioning with z-index** — `position: absolute` elements ordered by `z-index`
- **`margin-left: auto` right-alignment** for inline-block SVGs in flex containers

### Fixed
- Flex width double-resolution when both `widthUnit` and percentage were set
- Inline-block SVGs disappearing due to missing width propagation

## [0.2.0] - 2026-03-17

### Added
- **Auto-height pages** via CSS `@page { size: 80mm 0; }` — page height sizes to content (receipts, flyers)
- **Negative margin support** for flex column children — enables CSS patterns like `margin: -10px -14px` to break out of parent padding
- **`margin-left: auto`** on flex row items — pushes items to the right edge (e.g., seat box alignment)
- **`margin-top: auto`** on flex column items — pushes items to the bottom (e.g., footer positioning)
- **Cross-axis stretch** (W3C Flexbox §9.4) — flex row items stretch to match tallest sibling, with or without definite container height
- **Flex column `flex-grow`** — items with `flex: 1` now grow to fill remaining space in column direction
- **Flex column `justify-content`** — space-between, center, flex-end, space-around, space-evenly in column direction
- **`hasDefiniteCrossSize` flag** on Flex — enables stretch when Flex is wrapped in a height-constrained Div
- **Watermark support** in WASM render API via `watermark` parameter
- **Automatic Unicode font embedding** for non-WinAnsi characters (CIDFont with embedded cmap)
- **CIDFont fallback decoding** from embedded font cmap tables
- **Font caching** for repeated font resolution
- **Form XObject resolution** in PDF reader
- **Tagged PDF extraction** improvements
- **Full text matrix tracking** and font-aware space detection in reader
- **Xref cycle detection**, hybrid xref support, stream length correction
- **SVG enhancements**: text-anchor, tspan, defs/use, gradient support

### Fixed
- **Percentage heights** now resolve against parent container's explicit height, not the page — fixes vertical bar charts overflowing their containers
- **`box-sizing: border-box`** no longer double-subtracts padding from width/height — only border is subtracted since the Div handles padding internally
- **Double-padding on wrapped flex containers** — when a Flex has CSS width/height, visual properties (padding, borders, margins) are cleared from the Flex and applied only to the wrapper Div
- **`letter-spacing` in width measurement** — `Paragraph.MinWidth()` and `MaxWidth()` now include letter-spacing, preventing flex items from being measured too narrow
- **Floating-point overflow in `margin-top: auto`** — added 0.01pt epsilon tolerance to prevent items from silently overflowing due to float rounding
- **`margin-top: auto` phase consistency** — `neededBelow` calculation now includes `marginBottom` of subsequent items in both phase 1 and phase 3
- **SpaceBefore/SpaceAfter doubling** on flex items — element margins are cleared when FlexItem margins take over (Div, Flex, and Paragraph)
- **Background preserved on wrapped Flex** — `min-height` backgrounds now fill the full height (kept on both Div wrapper and inner Flex)
- **`parseFloat` negative numbers** — CSS parser now correctly handles negative values like `-10px`
- **Flex children splitting** into separate items instead of grouping per HTML child
- **SVG shapes invisible** and text mirrored
- **Sequential elements overlapping** by tracking cumulative Y offset in renderer
- **`<br>` tags in paragraphs** and CSS width as flex-basis
- **WASM binary size** halved by excluding `net/http` from js builds

### Changed
- Cross-axis stretch now fires for all flex row items (not just when container has definite height)
- `planColumn` refactored into 3-phase layout: measure, grow, position

## [0.1.1] - 2026-03-16

### Added
- CLI `extract` command with pluggable strategies (simple, location)
- CLI `sign` command for PAdES digital signatures
- Table `SetBorderCollapse(true)` for CSS-style collapsed borders
- CSS `calc()` support in HTML-to-PDF (e.g., `width: calc(100% - 40px)`)
- CSS `@page` rule parsing (size, margins) in HTML-to-PDF
- CSS `orphans`/`widows` properties in HTML-to-PDF
- CSS `break-before`/`break-after`/`break-inside` modern syntax
- Remote image loading (`<img src="https://...">`) in HTML-to-PDF
- Data URI image support (`<img src="data:image/png;base64,...">`)
- PDF metadata extraction from HTML `<title>` and `<meta>` tags
- Content stream processor with full graphics state (CTM, color, font)
- Pluggable text extraction strategies (Simple, Location, Region)
- Path and image extraction from content streams
- Per-glyph span extraction (opt-in)
- Text rendering mode awareness (invisible text filtering)
- Marked content tag tracking (BMC/BDC/EMC)
- Form XObject recursion in content processing
- Actual glyph widths from font metrics (replaces estimation)
- Auto-bookmarks from layout headings
- Viewer preferences (page layout, mode, UI options)
- Page labels (decimal, Roman, alpha)
- Page geometry boxes (CropBox, BleedBox, TrimBox, ArtBox)
- SVG package in README

### Changed
- CLI version bumped to 0.1.1
- README updated with extract and sign commands, border-collapse, SVG package

### Fixed
- Table border-collapse: adjacent cells no longer draw double borders
- Tables section in README had undefined variable

## [0.1.0] - 2026-03-15

### Added
- Initial release
- PDF generation with layout engine (Paragraph, Heading, Table, List, Div, Image, Float, Flex, Columns)
- PDF reader with tokenizer, parser, xref streams, object streams
- PDF merge and modify
- HTML-to-PDF conversion with CSS support
- Digital signatures (PAdES B-B, B-T, B-LT)
- Interactive forms (AcroForms)
- Barcodes (Code128, QR, EAN-13)
- Tagged PDF and PDF/A compliance
- SVG rendering
- CLI tool (merge, info, pages, text, create, blank)
- Font embedding and subsetting (TrueType)
- JPEG, PNG, TIFF image support
- Encryption (AES-256, AES-128, RC4)
