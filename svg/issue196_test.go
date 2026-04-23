// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
)

// TestSliceEmitsViewportClip verifies that slice mode emits a clip
// path on the target rectangle so content scaled to cover the
// viewport is not rendered outside it. Regression guard for #196.
func TestSliceEmitsViewportClip(t *testing.T) {
	svgXML := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 50 50" preserveAspectRatio="xMidYMid slice">
		<rect width="50" height="50" fill="red"/>
	</svg>`
	doc, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	stream := content.NewStream()
	doc.DrawWithOptions(stream, 0, 0, 200, 100, RenderOptions{})

	out := string(stream.Bytes())
	// The clip should be the target rectangle (0, 0, 200, 100) in the
	// translated coordinate space, followed by W (clip) and n (end path).
	if !strings.Contains(out, "0 0 200 100 re") {
		t.Errorf("expected slice viewport clip rectangle, got:\n%s", out)
	}
	// The re must be followed by W and n operators to establish the clip.
	idx := strings.Index(out, "0 0 200 100 re")
	if idx < 0 {
		t.Fatal("rectangle not found")
	}
	tail := out[idx:]
	if !strings.Contains(tail, "\nW\n") && !strings.Contains(tail, " W ") {
		t.Errorf("expected W operator after slice clip rectangle, got:\n%s", tail)
	}
	if !strings.Contains(tail, "\nn\n") && !strings.Contains(tail, " n") {
		t.Errorf("expected n operator after W for slice clip, got:\n%s", tail)
	}
}

// TestMeetDoesNotEmitViewportClip confirms the clip is limited to
// slice mode. Meet aligns content inside the viewport by construction,
// so emitting a viewport clip there would be dead code that bloats the
// content stream.
func TestMeetDoesNotEmitViewportClip(t *testing.T) {
	svgXML := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 50 50">
		<rect width="50" height="50" fill="red"/>
	</svg>`
	doc, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	stream := content.NewStream()
	doc.DrawWithOptions(stream, 0, 0, 200, 100, RenderOptions{})

	out := string(stream.Bytes())
	if strings.Contains(out, "0 0 200 100 re") {
		t.Errorf("meet mode must not emit a target-rect clip, got:\n%s", out)
	}
}

// TestNoneDoesNotEmitViewportClip confirms the same guard for the
// legacy non-uniform scaling path.
func TestNoneDoesNotEmitViewportClip(t *testing.T) {
	svgXML := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 50 50" preserveAspectRatio="none">
		<rect width="50" height="50" fill="red"/>
	</svg>`
	doc, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	stream := content.NewStream()
	doc.DrawWithOptions(stream, 0, 0, 200, 100, RenderOptions{})

	out := string(stream.Bytes())
	if strings.Contains(out, "0 0 200 100 re") {
		t.Errorf("none mode must not emit a target-rect clip, got:\n%s", out)
	}
}

// TestSliceWithoutViewBoxStillClips covers the fallback path where the
// SVG has no explicit viewBox; the renderer uses width/height instead.
// If those differ in aspect ratio from the target rectangle, slice
// still overflows and must still clip.
func TestSliceWithoutViewBoxStillClips(t *testing.T) {
	svgXML := `<svg xmlns="http://www.w3.org/2000/svg" width="50" height="50" preserveAspectRatio="xMidYMid slice">
		<rect width="50" height="50" fill="red"/>
	</svg>`
	doc, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	stream := content.NewStream()
	doc.DrawWithOptions(stream, 0, 0, 200, 100, RenderOptions{})

	out := string(stream.Bytes())
	if !strings.Contains(out, "0 0 200 100 re") {
		t.Errorf("expected slice clip even without a viewBox, got:\n%s", out)
	}
}

// TestZeroViewBoxDoesNotLeakClip guards against a future refactor
// moving the clip emission above the zero-viewBox early return. A
// zero-sized viewBox must produce an empty render (matched Save/
// Restore) with no orphan clip operators that would persist to
// subsequent page content.
func TestZeroViewBoxDoesNotLeakClip(t *testing.T) {
	svgXML := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 0 0" preserveAspectRatio="xMidYMid slice">
		<rect width="10" height="10" fill="red"/>
	</svg>`
	doc, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	stream := content.NewStream()
	doc.DrawWithOptions(stream, 0, 0, 200, 100, RenderOptions{})

	out := string(stream.Bytes())
	if strings.Contains(out, " re") {
		t.Errorf("zero viewBox must not emit any rectangle, got:\n%s", out)
	}
	if strings.Contains(out, "\nW\n") || strings.Contains(out, " W ") {
		t.Errorf("zero viewBox must not emit a clip, got:\n%s", out)
	}
}

// TestSliceClipHonorsFloatDimensions checks that the substring-based
// regression assertions remain correct when callers pass non-integer
// target dimensions. `content.formatNum` trims trailing zeros, so a
// half-point target prints as "150.5" rather than "150.500000".
func TestSliceClipHonorsFloatDimensions(t *testing.T) {
	svgXML := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" preserveAspectRatio="xMidYMid slice">
		<rect width="10" height="10" fill="red"/>
	</svg>`
	doc, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	stream := content.NewStream()
	doc.DrawWithOptions(stream, 0, 0, 150.5, 100.25, RenderOptions{})

	out := string(stream.Bytes())
	if !strings.Contains(out, "0 0 150.5 100.25 re") {
		t.Errorf("expected float dimensions in clip rectangle, got:\n%s", out)
	}
}
