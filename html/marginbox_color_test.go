// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import "testing"

// TestMarginBoxHasColorExplicit verifies that an explicit `color` declaration
// on a margin box sets HasColor=true and carries the parsed color through to
// the converted layout.MarginBox (issue #328 adjacent color bug).
func TestMarginBoxHasColorExplicit(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-center { content: "Page " counter(page); color: black; } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-center"]
	if !ok {
		t.Fatalf("no bottom-center margin box; got %v", result.MarginBoxes)
	}
	if !box.HasColor {
		t.Error("HasColor = false, want true for explicit color: black")
	}
	if box.Color != [3]float64{0, 0, 0} {
		t.Errorf("Color = %v, want black {0,0,0}", box.Color)
	}
}

// TestMarginBoxHasColorUnset verifies HasColor stays false when no color
// declaration is present, so the renderer applies its default gray.
func TestMarginBoxHasColorUnset(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-center { content: "Page " counter(page); } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-center"]
	if !ok {
		t.Fatalf("no bottom-center margin box; got %v", result.MarginBoxes)
	}
	if box.HasColor {
		t.Error("HasColor = true, want false when no color declared")
	}
}

// TestMarginBoxNonBlackColor verifies a distinct (non-black) color is parsed
// and carried with HasColor=true.
func TestMarginBoxNonBlackColor(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @top-right { content: counter(page); color: #ff0000; } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["top-right"]
	if !ok {
		t.Fatalf("no top-right margin box; got %v", result.MarginBoxes)
	}
	if !box.HasColor {
		t.Error("HasColor = false, want true for explicit color")
	}
	if box.Color[0] != 1 || box.Color[1] != 0 || box.Color[2] != 0 {
		t.Errorf("Color = %v, want red {1,0,0}", box.Color)
	}
}
