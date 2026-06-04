// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// renderWithMarginBox renders a single page with one bottom-center margin box
// and returns the page content stream as a string.
func renderWithMarginBox(t *testing.T, box MarginBox, tagged bool) string {
	t.Helper()
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetTagged(tagged)
	r.Add(NewParagraph("Body", font.Helvetica, 12))
	r.SetMarginBoxes(map[string]MarginBox{"bottom-center": box})
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}
	return string(pages[0].Stream.Bytes())
}

// TestMarginBoxExplicitBlackColorHonored verifies issue #328's adjacent color
// bug: an explicit color (HasColor=true) must be emitted verbatim, including
// pure black {0,0,0}, rather than being mistaken for unset and forced to gray.
func TestMarginBoxExplicitBlackColorHonored(t *testing.T) {
	content := renderWithMarginBox(t, MarginBox{
		Content:  "Page 1",
		FontSize: 9,
		Color:    [3]float64{0, 0, 0},
		HasColor: true,
	}, false)

	// Pure black fill: "0 0 0 rg". The default-gray "0.4 0.4 0.4 rg" must not
	// appear for this box.
	if !strings.Contains(content, "0 0 0 rg") {
		t.Errorf("expected explicit black fill color (0 0 0 rg) in output; got:\n%s", content)
	}
	if strings.Contains(content, "0.4 0.4 0.4 rg") {
		t.Error("explicit black color was overridden by the default gray")
	}
}

// TestMarginBoxDefaultGrayWhenNoColor verifies the default gray applies when
// the CSS did not set a color (HasColor=false).
func TestMarginBoxDefaultGrayWhenNoColor(t *testing.T) {
	content := renderWithMarginBox(t, MarginBox{
		Content:  "Page 1",
		FontSize: 9,
		HasColor: false,
	}, false)

	if !strings.Contains(content, "0.4 0.4 0.4 rg") {
		t.Errorf("expected default gray fill (0.4 0.4 0.4 rg); got:\n%s", content)
	}
}

// TestMarginBoxTaggedWrappedAsArtifact verifies that in tagged mode the
// margin-box drawing is wrapped in an /Artifact marked-content sequence, so a
// running header/footer stays out of the structure tree (PDF/UA).
func TestMarginBoxTaggedWrappedAsArtifact(t *testing.T) {
	content := renderWithMarginBox(t, MarginBox{
		Content:  "Page 1",
		FontSize: 9,
	}, true)

	if !strings.Contains(content, "/Artifact BDC") {
		t.Errorf("expected /Artifact BDC wrapper in tagged output; got:\n%s", content)
	}
	// The artifact sequence must enclose the text and be closed with EMC.
	artIdx := strings.Index(content, "/Artifact BDC")
	emcIdx := strings.Index(content[artIdx:], "EMC")
	tjIdx := strings.Index(content[artIdx:], "Tj")
	if emcIdx < 0 || tjIdx < 0 || tjIdx > emcIdx {
		t.Errorf("expected Tj before EMC inside the /Artifact sequence; got:\n%s", content[artIdx:])
	}
}

// TestMarginBoxUntaggedNoArtifact verifies that when not tagged, no artifact
// wrapper is emitted (avoids polluting untagged streams).
func TestMarginBoxUntaggedNoArtifact(t *testing.T) {
	content := renderWithMarginBox(t, MarginBox{
		Content:  "Page 1",
		FontSize: 9,
	}, false)

	if strings.Contains(content, "/Artifact BDC") {
		t.Error("did not expect /Artifact wrapper in untagged output")
	}
}
