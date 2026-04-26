// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"unicode/utf8"

	"github.com/carlos7ags/folio/font"
)

// NewParagraphFallback creates a paragraph that mixes scripts in a single
// string by dispatching each script run to the first face in fb that
// covers its base codepoint. Internally the paragraph is built as N
// TextRuns, one per (face, substring) segment, so the rest of the
// pipeline (shaping, measurement, drawing, subsetting, ToUnicode) runs
// unchanged on each segment.
//
// Phase 1 scope: cross-script fallback only. A whole script run (per UAX
// #24, with Common/Inherited resolved against neighbours) is locked to a
// single face so that per-script shapers (Arabic positional joining,
// Devanagari cluster formation, GPOS mark-to-base) see a coherent input.
// Within-script coverage gaps -- e.g. a Latin face missing U+1E9E -- are
// not yet resolved and will render as .notdef tofu; this is a Phase 2
// extension that requires per-cluster face dispatch with shaper
// coordination.
//
// Pointer identity: the *EmbeddedFont pointers stored on Fallback are
// passed verbatim into the generated TextRuns. The page-level font
// resource map dedupes by pointer, so the document still emits one
// Type0 font dict per underlying face regardless of how many paragraphs
// share the Fallback.
//
// Panics if fb is nil or fontSize is not positive.
func NewParagraphFallback(text string, fb *font.Fallback, fontSize float64) *Paragraph {
	if fb == nil {
		panic("layout.NewParagraphFallback: nil fallback")
	}
	if fontSize <= 0 {
		panic("layout.NewParagraphFallback: fontSize must be positive")
	}
	text = normalizeText(text)
	segs := segmentByFallback(text, fb)
	runs := make([]TextRun, len(segs))
	for i, seg := range segs {
		runs[i] = TextRun{Text: seg.Text, Embedded: seg.Face, FontSize: fontSize}
	}
	return &Paragraph{
		runs:    runs,
		leading: 1.2,
		align:   AlignLeft,
	}
}

// fallbackSegment is a contiguous byte range of the source string that
// has been assigned to a single face. Adjacent segments that share a
// face are merged at construction time so callers see the minimum number
// of runs for a given input.
type fallbackSegment struct {
	Face *font.EmbeddedFont
	Text string
}

// segmentByFallback walks the script runs of text and assigns each one
// to a single face from fb. The face is chosen by probing fb against
// the first base rune of the script run (i.e. the rune whose script
// caused the run to exist); if no face covers that rune, fb falls back
// to the first face. Consecutive script runs that resolve to the same
// face are coalesced into one segment so that downstream wrapping and
// measurement see a flat, minimal run list.
//
// An empty text input yields a single empty segment bound to the first
// face. This matches NewParagraphEmbedded's behaviour of always emitting
// at least one run, so the rest of the layout pipeline (which assumes
// every paragraph has runs) never has to special-case empty input.
func segmentByFallback(text string, fb *font.Fallback) []fallbackSegment {
	faces := fb.Faces()
	if text == "" {
		return []fallbackSegment{{Face: faces[0]}}
	}

	scriptRuns := SegmentByScript(text)
	if len(scriptRuns) == 0 {
		return []fallbackSegment{{Face: faces[0], Text: text}}
	}

	out := make([]fallbackSegment, 0, len(scriptRuns))
	for _, sr := range scriptRuns {
		sub := text[sr.Start:sr.End]
		probe := probeRune(sub, sr.Script)
		face := fb.PickFace(probe)
		if n := len(out); n > 0 && out[n-1].Face == face {
			out[n-1].Text += sub
			continue
		}
		out = append(out, fallbackSegment{Face: face, Text: sub})
	}
	return out
}

// probeRune returns the rune that should drive face selection for a
// script run. SegmentByScript promotes leading Common runes (spaces,
// punctuation, digits) into the script of their right neighbour via a
// reverse sweep, so the literal first rune of a script run is often a
// Common codepoint that has nothing to do with the run's resolved
// script. Probing on it would route the whole run to whatever face
// covers space/punctuation first (typically the Latin face), even
// when the run is Hebrew or Arabic.
//
// To avoid that miss, walk forward to the first rune whose own script
// matches the run's resolved script. That rune is the script's natural
// "base" and represents the coverage requirement for the segment. For
// pure-Common runs (e.g. "..."), no rune satisfies the match; return
// the first rune so the caller still has something to probe.
func probeRune(s string, want Script) rune {
	if s == "" {
		return 0
	}
	first, _ := utf8.DecodeRuneInString(s)
	if first == utf8.RuneError {
		return 0
	}
	if want == ScriptCommon {
		return first
	}
	for _, r := range s {
		if ScriptOf(r) == want {
			return r
		}
	}
	return first
}
