# Folio CSS support

> Auto-generated from `html/css_props.go`. Do not edit by hand.
> Run `go generate ./html/...` to regenerate after changing the registry.

Folio's HTML-to-PDF converter recognizes the CSS properties listed below.
Properties not in this document are silently ignored at render time.

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

These properties are commonly requested but NOT supported by Folio's HTML converter:

| Feature | Workaround |
|---|---|
| `oklch()` color | Use precomputed hex equivalents. Folio renders sRGB only. |
| `color-mix()` | Precompute the mixed color or define a CSS variable with the result. |
| `-webkit-line-clamp` / `line-clamp` | Truncate at the template / runtime layer before HTML emission; PDFs are paginated, not scrollable. |
| `text-wrap: pretty` | Cosmetic only; render without it. |
| ICC profiles | Folio renders into the sRGB color space. |
