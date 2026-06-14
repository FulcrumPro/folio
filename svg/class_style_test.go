// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
)

// TestInternalStyleClassFill pins the v0.9.1-fulcrum.25 patch: a path whose
// fill is declared via a class in an internal <style> element must render with
// that fill. .NET DocGen icons (the "//" fulcrum logo on job labels) use
// `<path class="st4">` + `<style>.st4{fill:#135EAB}</style>`; folio's SVG style
// resolver only read presentation attributes and the inline style attribute, so
// the class fill was never applied and the logo rendered with no fill (invisible).
func TestInternalStyleClassFill(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 108 108">
		<style type="text/css">.st4 { fill: #135EAB }</style>
		<g><path class="st4" d="M10,10 H90 V90 H10 Z"/></g>
	</svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 108, 108)
	out := string(stream.Bytes())

	if !strings.Contains(out, "rg") {
		t.Fatalf("expected a fill color (rg) operator; path rendered unfilled.\nstream:\n%s", out)
	}
	// #135EAB → B channel 171/255 ≈ 0.671. Assert the blue fill made it in,
	// not a default/black fill.
	if !strings.Contains(out, "0.67") {
		t.Errorf("expected blue fill #135EAB (B≈0.671) from the .st4 class rule; got:\n%s", out)
	}
}

// TestInlineStyleBeatsClassRule guards the cascade: an inline style attribute
// outranks an internal-stylesheet class rule (CSS 2.1 §6.4.4).
func TestInlineStyleBeatsClassRule(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 10 10">
		<style>.c { fill: #135EAB }</style>
		<path class="c" style="fill:#ff0000" d="M0,0 H10 V10 Z"/>
	</svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 10, 10)
	out := string(stream.Bytes())
	// Red = 1 0 0 rg. The inline fill must win over the class's blue.
	if !strings.Contains(out, "1 0 0 rg") {
		t.Errorf("inline style fill:#ff0000 should win over .c class blue; got:\n%s", out)
	}
}
