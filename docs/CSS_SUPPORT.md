# Folio CSS support

> Auto-generated from `html/css_props.go`. Do not edit by hand.
> Run `go generate ./html/...` to regenerate after changing the registry.

Folio's HTML-to-PDF converter recognizes the CSS properties listed below.
Properties not in this document are silently ignored at render time.

## How to read this document

Each per-category table lists the property name, any alternative names
(aliases) accepted by the parser, the value forms that are recognized,
and any notes about parsing or interactions with other properties.

Value forms are written in CSS spec shorthand: `<length>` means a
length value (e.g. `12px`, `1em`, `0.5in`); `<color>` means any
supported color form (named, hex, rgb/rgba, hsl/hsla, cmyk); and so on.
See [Value-form glossary](#value-form-glossary) below for the full list.

If you don't see a property here, Folio's parser silently ignores it
at render time — there is no warning. Use
[`html.Options.StrictAssets`](../html) to escalate certain asset failures,
but unknown CSS properties are always silent.

## At a glance

| Category | Properties |
|---|---:|
| Typography | 23 |
| Color | 1 |
| Backgrounds | 6 |
| BoxModel | 17 |
| Borders | 17 |
| Layout | 12 |
| Flexbox | 11 |
| Grid | 16 |
| MultiColumn | 9 |
| Tables | 2 |
| Pagination | 5 |
| Lists | 1 |
| Effects | 12 |
| PDF | 6 |
| **Total** | **138** |

## Value-form glossary

Angle-bracket placeholders used in the per-property tables below.

| Placeholder | Meaning |
|---|---|
| `<length>` | A CSS length: `<number><unit>` where unit is `px`, `pt`, `em`, `rem`, `cm`, `mm`, or `in`. Examples: `12px`, `1.5em`, `0.5in`. Distinct from `<percentage>`, which is listed separately as an alternative in per-property tables. |
| `<percentage>` | A `<number>%`. Resolves against the containing context (line-height, parent dimension, etc.). |
| `<number>` | A unitless real number, e.g. `1.5`, `0.7`, `-2`. |
| `<integer>` | A whole number, e.g. `0`, `5`, `-1`. Range constraints (e.g. `<integer 1..6>`) are listed in the per-property table. |
| `<string>` | A quoted text literal, e.g. `"My Title"` or `'caption'`. |
| `<color>` | Any of: `<named>` (`red`, `transparent`), `<hex>` (`#abc`, `#aabbcc`), `rgb()`, `rgba()`, `hsl()`, `hsla()`, `cmyk()`. Folio renders sRGB only — `oklch()` and `color-mix()` are not supported. |
| `<named>`, `<hex>` | Component forms of `<color>`: `<named>` is a CSS named color (`red`, `aliceblue`, etc.); `<hex>` is `#RGB`, `#RGBA`, `#RRGGBB`, or `#RRGGBBAA`. |
| `<line-width>` | A `<length>` or one of the keywords `thin`, `medium`, `thick`. Used in border/outline shorthands. |
| `<line-style>` | One of `solid`, `dashed`, `dotted`, `double`, `none`. |
| `<position>` | A 1- or 2-component position keyword/length. Examples: `center`, `top right`, `50% 25%`, `10px 20px`. Applies to `background-position`, `object-position`, `transform-origin`. |
| `<grid-line>` | A grid line reference: an integer (e.g. `2`), a `span` keyword (`span 3`), or a named line (rare; line names not yet supported). |
| `<track-list>` | A space-separated list of track sizes for `grid-template-columns`/`-rows`. Examples: `1fr 1fr`, `100px auto`, `repeat(3, 1fr)`. |
| `<track-size>` | A single grid track size: `<length>`, `<percentage>`, `<number>fr`, `auto`, `min-content`, `max-content`. |
| `<ratio>` | An aspect ratio expressed as `<number>/<number>` or a single `<number>`. Example: `16/9`. |
| `<gradient>` | `linear-gradient(...)`, `repeating-linear-gradient(...)`, `radial-gradient(...)`, or `repeating-radial-gradient(...)`. |
| `<transform-function>` | A CSS transform: `translate()`, `translateX()`/`Y()`, `rotate()`, `scale()`/`X()`/`Y()`, `skew()`/`X()`/`Y()`. |
| `<offset-x>`, `<offset-y>`, `<blur>`, `<spread>` | Component lengths in shadow shorthands (`box-shadow`, `text-shadow`). All are `<length>`; spread accepts negatives to inset the shadow. |
| `<identifier>` | A custom name, e.g. for `counter-reset` or `string-set`. |

**`calc()`, `min()`, `max()`, `clamp()`** are accepted everywhere a `<length>` or `<percentage>` is. The parser preserves them as single tokens through shorthand splitting.

## Box-alignment properties

`justify-content`, `align-items`, `align-self`, and `align-content` are
listed under Flexbox or Grid in the per-category tables for grouping,
but per CSS Box Alignment Level 3 they apply to BOTH flex and grid
containers. Folio honors them in either context.

Similarly, `gap` (and its alias `grid-gap`) is grouped under Grid but
also takes effect on flex containers as the gap between items.

## Typography

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `baseline-shift` | — | `super`, `sub`, `baseline`, `<length>`, `<percentage>` | CSS Inline Layout Module Level 3 §4.3 — percentages resolve against line-height. |
| `direction` | — | `ltr`, `rtl` | Interacts with unicode-bidi; together they control bidi paragraph direction. |
| `font` | — | `<font-style>? <font-weight>? <font-size>[/<line-height>]? <font-family>` | — |
| `font-family` | — | `<family-name>`, `<generic-family>` | — |
| `font-size` | — | `<absolute-size>`, `<relative-size>`, `<length>`, `<percentage>` | — |
| `font-style` | — | `normal`, `italic`, `oblique` | — |
| `font-weight` | — | `normal`, `bold`, `bolder`, `lighter`, `<integer 100..900>` | — |
| `hyphens` | `-webkit-hyphens` | `none`, `manual`, `auto` | — |
| `letter-spacing` | — | `<length>`, `normal` | — |
| `line-height` | — | `<number>`, `<length>`, `<percentage>`, `normal` | — |
| `text-align` | — | `left`, `right`, `center`, `justify`, `start`, `end` | — |
| `text-align-last` | — | `left`, `right`, `center`, `justify`, `start`, `end` | — |
| `text-decoration` | — | `none`, `underline`, `overline`, `line-through`, `blink` | — |
| `text-decoration-color` | — | `<color>` | — |
| `text-decoration-style` | — | `solid`, `dashed`, `dotted`, `double`, `wavy` | — |
| `text-indent` | — | `<length>`, `<percentage>` | — |
| `text-shadow` | — | `<offset-x> <offset-y> [<blur>] [<color>]`, `none` | — |
| `text-transform` | — | `uppercase`, `lowercase`, `capitalize`, `none` | — |
| `unicode-bidi` | — | `normal`, `embed`, `bidi-override`, `isolate`, `isolate-override`, `plaintext` | Interacts with direction; together they control bidi paragraph layout. |
| `vertical-align` | — | `top`, `middle`, `bottom`, `super`, `sub`, `baseline`, `text-top`, `text-bottom`, `<length>`, `<percentage>` | — |
| `white-space` | — | `normal`, `nowrap`, `pre`, `pre-wrap`, `pre-line` | — |
| `word-break` | — | `normal`, `break-all`, `keep-all`, `break-word` | — |
| `word-spacing` | — | `<length>`, `normal` | — |

## Color

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `color` | — | `<named>`, `<hex>`, `rgb()`, `rgba()`, `hsl()`, `hsla()`, `cmyk()` | sRGB only. oklch() and color-mix() are not supported. |

## Backgrounds

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `background` | — | `<color>`, `<gradient>`, `url(...)` | Background shorthand: dispatches to BackgroundImage for gradient/url, BackgroundColor otherwise. |
| `background-color` | — | `<color>` | — |
| `background-image` | — | `<gradient>`, `url(...)`, `none` | — |
| `background-position` | — | `<position>` | — |
| `background-repeat` | — | `repeat`, `repeat-x`, `repeat-y`, `no-repeat`, `space`, `round` | — |
| `background-size` | — | `<length>`, `<percentage>`, `auto`, `cover`, `contain` | — |

## BoxModel

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `aspect-ratio` | — | `<ratio>`, `auto` | — |
| `height` | — | `<length>`, `<percentage>`, `auto` | — |
| `margin` | — | `<length>`, `<percentage>`, `auto`, `<1-4 of these>` | auto keyword sets MarginTopAuto/LeftAuto/RightAuto per CSS shorthand position rules. |
| `margin-bottom` | — | `<length>`, `<percentage>` | margin-top, margin-left, margin-right also accept `auto`; margin-bottom does not. |
| `margin-left` | — | `<length>`, `<percentage>`, `auto` | — |
| `margin-right` | — | `<length>`, `<percentage>`, `auto` | — |
| `margin-top` | — | `<length>`, `<percentage>`, `auto` | — |
| `max-height` | — | `<length>`, `<percentage>`, `none` | — |
| `max-width` | — | `<length>`, `<percentage>`, `none` | — |
| `min-height` | — | `<length>`, `<percentage>` | — |
| `min-width` | — | `<length>`, `<percentage>` | — |
| `padding` | — | `<length>`, `<percentage>`, `<1-4 of these>` | — |
| `padding-bottom` | — | `<length>`, `<percentage>` | — |
| `padding-left` | — | `<length>`, `<percentage>` | — |
| `padding-right` | — | `<length>`, `<percentage>` | — |
| `padding-top` | — | `<length>`, `<percentage>` | — |
| `width` | — | `<length>`, `<percentage>`, `auto` | — |

## Borders

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `border` | — | `<line-width> <line-style> <color>` | Sets all 12 fields (4 sides × {width, style, color}) at once. |
| `border-bottom` | — | `<line-width> <line-style> <color>` | — |
| `border-bottom-left-radius` | — | `<length>`, `<percentage>` | — |
| `border-bottom-right-radius` | — | `<length>`, `<percentage>` | — |
| `border-bottom-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-color` | — | `<color>` | — |
| `border-left` | — | `<line-width> <line-style> <color>` | — |
| `border-left-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-radius` | — | `<length>`, `<percentage>`, `<1-4 of these>` | splitTopLevelFields preserves calc()/min()/max()/clamp() as single tokens. |
| `border-right` | — | `<line-width> <line-style> <color>` | — |
| `border-right-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-style` | — | `solid`, `dashed`, `dotted`, `double`, `none` | — |
| `border-top` | — | `<line-width> <line-style> <color>` | — |
| `border-top-left-radius` | — | `<length>`, `<percentage>` | — |
| `border-top-right-radius` | — | `<length>`, `<percentage>` | — |
| `border-top-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `border-width` | — | `<line-width>` | — |

## Layout

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `bottom` | — | `<length>`, `<percentage>`, `auto` | — |
| `box-sizing` | — | `content-box`, `border-box` | — |
| `clear` | — | `left`, `right`, `both`, `none` | — |
| `display` | — | `block`, `inline`, `inline-block`, `flex`, `grid`, `table`, `table-row`, `table-cell`, `list-item`, `none` | — |
| `float` | — | `left`, `right`, `none` | — |
| `left` | — | `<length>`, `<percentage>`, `auto` | — |
| `overflow` | — | `hidden`, `visible`, `auto`, `scroll` | — |
| `position` | — | `static`, `relative`, `absolute`, `fixed` | — |
| `right` | — | `<length>`, `<percentage>`, `auto` | — |
| `top` | — | `<length>`, `<percentage>`, `auto` | — |
| `visibility` | — | `visible`, `hidden`, `collapse` | — |
| `z-index` | — | `<integer>`, `auto` | — |

## Flexbox

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `align-items` | — | `stretch`, `flex-start`, `flex-end`, `center`, `baseline`, `start`, `end` | — |
| `align-self` | — | `auto`, `stretch`, `flex-start`, `flex-end`, `center`, `baseline` | — |
| `flex` | — | `<flex-grow> <flex-shrink>? <flex-basis>?`, `none`, `auto` | — |
| `flex-basis` | — | `<length>`, `<percentage>`, `auto`, `content` | — |
| `flex-direction` | — | `row`, `row-reverse`, `column`, `column-reverse` | — |
| `flex-flow` | — | `<flex-direction> <flex-wrap>` | — |
| `flex-grow` | — | `<number>` | — |
| `flex-shrink` | — | `<number>` | — |
| `flex-wrap` | — | `nowrap`, `wrap`, `wrap-reverse` | — |
| `justify-content` | — | `flex-start`, `flex-end`, `center`, `space-between`, `space-around`, `space-evenly`, `start`, `end` | — |
| `order` | — | `<integer>` | — |

## Grid

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `align-content` | — | `normal`, `stretch`, `flex-start`, `flex-end`, `center`, `space-between`, `space-around`, `space-evenly` | — |
| `gap` | `grid-gap` | `<row-gap>`, `<row-gap> <column-gap>` | Sets RowGap and GridColumnGap; in flex contexts, also Gap. |
| `grid-area` | — | `<grid-line> [/ <grid-line>]{0..3}` | — |
| `grid-auto-flow` | — | `row`, `column`, `dense`, `row dense`, `column dense` | — |
| `grid-auto-rows` | — | `<track-size>` | — |
| `grid-column` | — | `<grid-line> [/ <grid-line>]?` | — |
| `grid-column-end` | — | `<integer>` | — |
| `grid-column-start` | — | `<integer>` | — |
| `grid-row` | — | `<grid-line> [/ <grid-line>]?` | — |
| `grid-row-end` | — | `<integer>` | — |
| `grid-row-start` | — | `<integer>` | — |
| `grid-template-areas` | — | `<string>+`, `none` | — |
| `grid-template-columns` | — | `<track-list>`, `none` | — |
| `grid-template-rows` | — | `<track-list>`, `none` | — |
| `justify-items` | — | `start`, `end`, `center`, `stretch` | — |
| `row-gap` | — | `<length>`, `<percentage>`, `normal` | — |

## MultiColumn

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `column-count` | — | `<integer>`, `auto` | — |
| `column-gap` | — | `<length>`, `normal` | — |
| `column-rule` | — | `<line-width> <line-style> <color>` | — |
| `column-rule-color` | — | `<color>` | — |
| `column-rule-style` | — | `solid`, `dashed`, `dotted`, `double`, `none` | — |
| `column-rule-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `column-span` | — | `none`, `all` | — |
| `column-width` | — | `<length>`, `auto` | — |
| `columns` | — | `<column-count>`, `<column-width>`, `<column-count> <column-width>` | — |

## Tables

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `border-collapse` | — | `collapse`, `separate` | — |
| `border-spacing` | — | `<length>`, `<length> <length>` | — |

## Pagination

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `orphans` | — | `<integer>` | — |
| `page-break-after` | `break-after` | `always`, `page`, `avoid`, `avoid-page`, `auto` | — |
| `page-break-before` | `break-before` | `always`, `page`, `avoid`, `avoid-page`, `auto` | — |
| `page-break-inside` | `break-inside` | `avoid`, `avoid-page`, `auto` | — |
| `widows` | — | `<integer>` | — |

## Lists

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `list-style-type` | `list-style` | `disc`, `circle`, `square`, `decimal`, `lower-alpha`, `upper-alpha`, `lower-roman`, `upper-roman`, `none` | list-style is a shorthand; only the type is extracted. |

## Effects

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `box-shadow` | — | `<offset-x> <offset-y> [<blur>] [<spread>] [<color>] [inset]`, `none` | Supports comma-separated multiple shadows. |
| `object-fit` | — | `contain`, `cover`, `fill`, `none`, `scale-down` | — |
| `object-position` | — | `<position>` | — |
| `opacity` | — | `<number 0..1>` | Values outside 0..1 are clamped. |
| `outline` | — | `<line-width> <line-style> <color>` | — |
| `outline-color` | — | `<color>` | — |
| `outline-offset` | — | `<length>` | — |
| `outline-style` | — | `solid`, `dashed`, `dotted`, `double`, `none` | — |
| `outline-width` | — | `<length>`, `thin`, `medium`, `thick` | — |
| `text-overflow` | — | `clip`, `ellipsis` | — |
| `transform` | — | `<transform-function>+`, `none` | — |
| `transform-origin` | — | `<position>` | — |

## PDF

| Property | Aliases | Accepted values | Notes |
|---|---|---|---|
| `bookmark-label` | — | `<string>`, `content()`, `attr(<identifier>)` | content() and attr() are resolved at element-conversion time. |
| `bookmark-level` | — | `<integer 1..6>`, `none` | Per CSS GCPM. Levels are clamped to Folio's H1-H6 range. |
| `bookmark-state` | — | `open`, `closed` | — |
| `counter-increment` | — | `<identifier> [<integer>]+` | — |
| `counter-reset` | — | `<identifier> [<integer>]+` | — |
| `string-set` | — | `<identifier> <content-list>` | Used by @page margin boxes for running headers/footers. |

## Known unsupported features

These properties / values are commonly requested but NOT recognized by Folio.
Folio silently ignores unknown property names, so a stylesheet that uses
any of these will render — just without the styling those declarations
would have applied in a browser.

| Feature | Why | Workaround |
|---|---|---|
| `oklch()`, `oklab()`, `lch()`, `lab()` color | Folio renders sRGB only; no ICC profile support. | Precompute the sRGB equivalent and use `#hex` or `rgb()`. |
| `color-mix()` | Folio's parser doesn't expand the function. | Precompute the mixed color, or assign it to a CSS variable: `--btn-tint: #c44;`. |
| `-webkit-line-clamp` / `line-clamp` | PDFs are paginated, not scrollable; the property has no analogue. | Truncate before HTML emission, or use `layout.Paragraph.SplitAfterLine` for first-N-lines-plus-appendix flows. |
| `text-wrap: pretty` / `text-wrap: balance` | Browser-only line-break heuristic; cosmetic. | Render without it. |
| `filter`, `backdrop-filter`, `mix-blend-mode` | PDF lacks an analogue for screen-compositing. | Pre-bake effects into images. |
| `:hover`, `:focus`, `:active` | PDF has no interaction state. | Style the static state directly. |
| Custom HTML elements / Web Components | Folio's HTML parser handles a fixed element set. | Pre-render to a known element (`<div>` / `<span>`) before passing to Folio. |
| `@media` queries | PDF output has fixed page geometry. | Use `@page` rules for page-size-specific styling. |
| `position: sticky` | Has no analogue in paginated layout. | Use `@page` running headers/footers via margin boxes. |
| ICC profiles for color management | Folio is sRGB-only. | Use sRGB-correct hex values; convert assets to sRGB before embedding. |

## Adding a new CSS property

1. Append a `cssProperty` entry to `cssProperties` in `html/css_props.go`.
   Required: `Name` and `Apply`. Recommended: `Category`, `Values`, `Notes`.
2. Run `go generate ./html/...` to regenerate this document.
3. Add at least one row to `TestCSSPropertyParitySnapshot` in
   `html/css_props_test.go` asserting the new property's behavior.
4. CI guards: `TestCSSDocsInSync` ensures the doc matches the registry,
   and `TestNoSwitchRegistryOverlap` ensures no legacy switch case is
   reintroduced for a registered property.
