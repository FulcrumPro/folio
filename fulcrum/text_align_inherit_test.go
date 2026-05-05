package fulcrum

import (
	"bytes"
	"testing"
)

// TestTextAlignInherits pins the v0.7.1-fulcrum.3 patch: text-align is
// an inherited CSS property (CSS Text Module Level 3 §7.1), so a child
// block element with no explicit text-align must render text using the
// nearest ancestor's text-align. Before the patch, computedStyle.inherit
// copied the value but not the TextAlignSet flag, so the
// buildParagraphFromRuns conditional in converter_paragraph.go skipped
// the SetAlign call and child paragraphs silently fell back to
// AlignLeft.
//
// We observe the fix by rendering identical content twice — once with
// text-align declared on the parent, once with it declared directly on
// the child — and asserting byte-equal PDFs. folio is deterministic
// for the same input, so any difference here means inheritance took a
// different code path than direct declaration.
func TestTextAlignInherits(t *testing.T) {
	cases := []struct {
		name string
		// Both sources must produce the same PDF: parent vs direct.
		parent string
		direct string
	}{
		{
			name: "right via parent class",
			parent: `<html><head><style>
				.outer { text-align: right; width: 200pt; }
			</style></head><body><div class="outer"><div>aligned</div></div></body></html>`,
			direct: `<html><head><style>
				.outer { width: 200pt; }
				.inner { text-align: right; }
			</style></head><body><div class="outer"><div class="inner">aligned</div></div></body></html>`,
		},
		{
			name: "center via grandparent (two-level inheritance)",
			parent: `<html><head><style>
				.outer { text-align: center; width: 200pt; }
			</style></head><body><div class="outer"><div><div>aligned</div></div></div></body></html>`,
			direct: `<html><head><style>
				.outer { width: 200pt; }
				.target { text-align: center; }
			</style></head><body><div class="outer"><div><div class="target">aligned</div></div></div></body></html>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parentPDF, err := renderHTMLToPDF(tc.parent)
			if err != nil {
				t.Fatalf("render parent: %v", err)
			}
			directPDF, err := renderHTMLToPDF(tc.direct)
			if err != nil {
				t.Fatalf("render direct: %v", err)
			}
			if !bytes.Equal(parentPDF, directPDF) {
				t.Errorf("inherited text-align produced different PDF than directly-applied (parent=%d bytes, direct=%d bytes) — inheritance is broken",
					len(parentPDF), len(directPDF))
			}
		})
	}
}
