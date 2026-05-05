# FulcrumPro/folio fork

This is FulcrumPro's working fork of [carlos7ags/folio](https://github.com/carlos7ags/folio).

## Why we fork

Fulcrum's HTML→PDF pipeline (`FulcrumProduct/fulcrum/internal/pdfrender`) ships
hand-ported `.hbs` templates from `FulcrumProduct/DocGen/`. When folio v0.7.1
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

- `v0.7.1-fulcrum.0` — fork mirror of upstream v0.7.1, zero patches. Baseline.
- `v0.7.1-fulcrum.1`, `.2`, … — each patch ships its own tag, in commit order.

Semver-prerelease ordering puts `v0.7.1-fulcrum.N` *before* `v0.7.1` in semver
terms; that's fine because Fulcrum's `go.mod` uses a `replace` directive,
which short-circuits version comparison.

## How Fulcrum's repo wires it

In `FulcrumProduct/fulcrum/go.mod`:

```
replace github.com/carlos7ags/folio => github.com/FulcrumPro/folio v0.7.1-fulcrum.N
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
| `v0.7.1-fulcrum.1` | Keep inline whitespace between adjacent inline siblings | `html/converter.go` (`isInlineFlowChild`, plus new `isInlineFlowElement` and `participatesInInlineFlow` helpers) | `fulcrum/internal/pdfrender/inline_ws.go` (HTML pre-rewrite) |
| `v0.7.1-fulcrum.2` | Accept `start` / `end` shorthand on `justify-content` (CSS Box Alignment Level 3) | `html/converter_flex.go` (justify-content switch, lines 30-44) | Lets `justify-content: end` from .NET source render correctly without rewriting templates to use `flex-end` |
| `v0.7.1-fulcrum.3` | Inherited `text-align` was not applied to child block paragraphs because `computedStyle.inherit()` copied the value but not the `TextAlignSet` flag the apply path conditioned on | `html/style.go` (`inherit()`) | Lets the .NET source's `text-align: right`/`center` parent rules work on child blocks without sprinkling `text-align: …` directly on every block — drops the per-block `text-center` / `text-right` class duplication in the hand-ported templates |
| `v0.7.1-fulcrum.4` | Horizontal margin on inline elements (`<span style="margin-right: 5px">`) was ignored — `collectRunsFromNode` had no margin handling | `html/converter_paragraph.go` (collectRunsFromNode element branch, ~lines 320-360) | Lets `<span class="data-label">…</span><span class="data-value">…</span>` from .NET DocGen render with the requested 5px gap; once we regenerate `dataitem.html` from `.hbs` source, the line-break workaround between the two spans goes away. Approximation note: exact font-metric calibration isn't possible without the run measurer (layout time), so the rendered gap is via `LetterSpacing` on a single space and is approximate but always >= 0. |
| `v0.7.1-fulcrum.5` | `float: left/right` on direct flex children was honored, contradicting CSS Flexbox §3 — folio wrapped the child in a `layout.Float` and the flex width calc mis-shrunk the resulting non-flex-shaped item | `html/converter_flex.go` (post-convertNode unwrap loop) plus a new `Float.Content()` accessor in `layout/float.go` | Lets the .NET DocGen `.three-columns { float: left }` inside `.data-container { display: flex }` render with full 33% column widths instead of mis-wrapped narrow columns — drops the styles.html deviation that removed `float: left` from `.three-columns` |

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
