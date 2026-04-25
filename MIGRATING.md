# Migrating

For the full list of new features and fixes, see [CHANGELOG.md](CHANGELOG.md).

---

## Upgrading from v0.7.x to v0.8.0

### `html.Options.BasePath` is removed

`BaseFS` is now the single way to resolve every local asset referenced
by an HTML document — `<img src>`, `<link href>`, `@font-face url()`,
`background-image: url()`, and `Options.FallbackFontPath`.

Migration:

```go
// before
opts := &html.Options{BasePath: "./assets"}

// after
opts := &html.Options{BaseFS: os.DirFS("./assets")}
```

Any `fs.FS` works:

| Source                | Example                                  |
|-----------------------|------------------------------------------|
| Embedded assets       | `BaseFS: assetsFS // //go:embed assets`  |
| On-disk directory     | `BaseFS: os.DirFS("./assets")`           |
| Sandboxed directory   | `r, _ := os.OpenRoot("./assets"); BaseFS: r.FS()` |
| In-memory (tests)     | `BaseFS: fstest.MapFS{...}`              |

### Path semantics changed

Paths in the document are normalised to `fs.FS` conventions before the
read: forward slashes only, no leading `/`, no `..` traversal — invalid
paths are rejected before the open.

A leading `/` in `src`/`href` is now treated as web-style root of the
`BaseFS` (matching how `<base href="/">` works in browsers) instead of
an absolute filesystem path. If you previously relied on absolute
filesystem paths, mount the directory containing them as `BaseFS` and
use root-relative `/`-prefixed paths in the document.

When `BaseFS` is nil, every local-asset reference fails — the document
must inline its assets via `data:` URIs.

### `@font-face` resolves relative to its containing stylesheet

A linked stylesheet at `css/site.css` containing
`@font-face { src: url(../fonts/Inter.ttf); }` now resolves to
`fonts/Inter.ttf` from the `BaseFS` root — matching browser behavior.
Previously the URL resolved from the document root regardless of where
the stylesheet lived.

If your `@font-face` rules currently rely on the old root-anchored
behavior, either:

- use a root-relative path (`url(/fonts/Inter.ttf)`), or
- move the rule into an inline `<style>` block in the document.

HTTP-origin stylesheets resolve relative URLs as HTTP, FS-origin
stylesheets resolve them through `BaseFS`. Inline `<style>` blocks
continue to resolve relative URLs from the `BaseFS` root.

### Surfacing asset-load errors

A new optional `Options.Logger` (`*slog.Logger`) receives warn-level
events when an asset fails to load: missing `@font-face` files,
unreadable linked stylesheets, image fetches that fall back to alt
text. Previously these were silently swallowed. To surface them during
development:

```go
opts := &html.Options{
    BaseFS: os.DirFS("./assets"),
    Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
}
```

The default remains silent, so existing callers see no behavior change
unless they opt in.

### `tmpl.RenderFile` and `tmpl.RenderFileTo`

These helpers now auto-populate `BaseFS` to `os.DirFS(filepath.Dir(templatePath))`
when the caller leaves it nil, so a template referencing `<img src="logo.png">`
next to itself resolves without extra wiring. Callers that already pass a
`BaseFS` keep their value.

### C ABI

The signature of `folio_document_add_html_with_options` is unchanged —
it still takes a `basePath` C string and wraps it as
`os.DirFS(basePath)` internally at the boundary. C consumers see no
breaking change.

---

## Upgrading from v0.6.x to v0.7.0

No code changes are required. Every new API is additive and the
zero-value `document.WriteOptions` reproduces byte-identical output to
v0.6.2.

Several bug fixes change the visible output of affected documents.
Review these before regression-diffing PDFs against a v0.6.x baseline:

- **CSS multi-column fill order** — sequential balanced fill replaces
  round-robin distribution (#145).
- **Arabic text shaping** — OpenType GSUB contextual shaping replaces
  the isolated-forms fallback when the font has GSUB positional
  features (#160).
- **TrueType kerning** — Apple-format v0 coverage (Arial and others)
  now returns non-zero kern pairs; spacing tightens (#172).
- **CJK line-breaking** — JIS X 4051 kinsoku shori rules (#157).
- **Per-glyph font fallback** — runes outside the primary font's
  coverage route to a configured fallback face.
- **CSS `!important` cascade** at the inline/stylesheet boundary
  is now honored (#137).
- **`/ActualText` markers around shaped Arabic words** emitted by
  default; opt out with `Document.SetActualText(false)`.

Deprecated direct-field access on `core.PdfDictionary.Entries`,
`core.PdfArray.Elements`, and `core.PdfIndirectReference.ObjectNumber` /
`.GenerationNumber` remains supported in v0.7.0 and is scheduled for
removal at v1.0. Migration targets are listed in the Deprecated section
of the CHANGELOG.

---

## Upgrading from v0.5.x to v0.6.0

## 1. Rename constructors

All constructors now follow `New*` / `Load*` / `Parse*` conventions.

Run this to fix all renames automatically:

```bash
find . -name '*.go' -exec sed -i '' \
  -e 's/reader\.Open(/reader.Load(/g' \
  -e 's/barcode\.QRWithECC(/barcode.NewQRWithECC(/g' \
  -e 's/barcode\.QR(/barcode.NewQR(/g' \
  -e 's/barcode\.Code128(/barcode.NewCode128(/g' \
  -e 's/barcode\.EAN13(/barcode.NewEAN13(/g' \
  -e 's/layout\.RunEmbedded(/layout.NewRunEmbedded(/g' \
  -e 's/layout\.Run(/layout.NewRun(/g' \
  -e 's/sign\.LoadPKCS12(/sign.ParsePKCS12(/g' \
  -e 's/forms\.MultilineTextField(/forms.NewMultilineTextField(/g' \
  -e 's/forms\.PasswordField(/forms.NewPasswordField(/g' \
  -e 's/forms\.SignatureField(/forms.NewSignatureField(/g' \
  -e 's/forms\.TextField(/forms.NewTextField(/g' \
  -e 's/forms\.Checkbox(/forms.NewCheckbox(/g' \
  -e 's/forms\.Dropdown(/forms.NewDropdown(/g' \
  -e 's/forms\.ListBox(/forms.NewListBox(/g' \
  -e 's/forms\.RadioGroup(/forms.NewRadioGroup(/g' \
  {} +
```

Full rename table:

| Old | New |
|-----|-----|
| `reader.Open(path)` | `reader.Load(path)` |
| `barcode.QR(data)` | `barcode.NewQR(data)` |
| `barcode.Code128(data)` | `barcode.NewCode128(data)` |
| `barcode.EAN13(data)` | `barcode.NewEAN13(data)` |
| `barcode.QRWithECC(data, level)` | `barcode.NewQRWithECC(data, level)` |
| `layout.Run(text, font, size)` | `layout.NewRun(text, font, size)` |
| `layout.RunEmbedded(text, ef, size)` | `layout.NewRunEmbedded(text, ef, size)` |
| `sign.LoadPKCS12(data, password)` | `sign.ParsePKCS12(data, password)` |
| `forms.TextField(...)` | `forms.NewTextField(...)` |
| `forms.Checkbox(...)` | `forms.NewCheckbox(...)` |
| `forms.Dropdown(...)` | `forms.NewDropdown(...)` |
| `forms.ListBox(...)` | `forms.NewListBox(...)` |
| `forms.RadioGroup(...)` | `forms.NewRadioGroup(...)` |
| `forms.PasswordField(...)` | `forms.NewPasswordField(...)` |
| `forms.MultilineTextField(...)` | `forms.NewMultilineTextField(...)` |
| `forms.SignatureField(...)` | `forms.NewSignatureField(...)` |

## 2. Rename `sign.LoadPKCS12` → `sign.ParsePKCS12`

The function was renamed to match the `Parse*` convention (it takes
`[]byte`, not a file path). The signature is unchanged — only the name.

```go
// Before
signer, err := sign.LoadPKCS12(data, "password")

// After
signer, err := sign.ParsePKCS12(data, "password")
```

## 3. Handle `Document.Page` error return

```go
// Before
page := doc.Page(0)

// After
page, err := doc.Page(0)
if err != nil {
    // handle out-of-range index
}
```

## 4. Remove references to unexported symbols

These were internal and are now unexported. If your code referenced
them directly, switch to the public API equivalents.

**reader:** `buildFontCache`, `parseStructureTree`, `glyphToRune`,
`serializeContentOps`, `winAnsiEncoding`, `macRomanEncoding`,
`standardEncoding`

**svg:** `parseColor`, `parseTransform`, `arcToCubics`, `parsePathData`,
`defaultStyle`, `resolveStyle`, `identity`

## 5. Visual review — baseline positioning changed

Text baselines now use CSS half-leading with actual font metrics.
For Helvetica 12pt at 1.2 leading, text moves **up ~4pt** within
line boxes. Background colors now cover the full line height.

**All generated PDFs will look different.** The change is correct
per CSS 2.1 §10.8.1, but documents that relied on the old positioning
should be visually reviewed.

## 6. Check for `vertical-align` with length values

`vertical-align: 5pt` was previously silently ignored. It now
produces a baseline shift. If your HTML has accidental length values
on `vertical-align`, they will now take effect.
