// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"
)

// End-to-end: li::marker { content: counters(item, ".") ". " } on the FAST
// (inline) path drives native multi-level numbering through the counter stack.
// This also exercises Phase 1's fast-path nested counter wiring.
//
// Tokenization note: a marker like "1.1. " is emitted as two text words —
// "1.1" then "." (the trailing ". " starts a new word), so the stream contains
// (1.1) Tj followed by (.) Tj rather than a single (1.1.) Tj. The top-level
// "1." has no internal space and renders as one (1.) Tj. Assertions match the
// real tokens while still proving the marker came from content.
func TestMarkerContentNestedCounters(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: item; list-style: none }
		li { counter-increment: item }
		li::marker { content: counters(item, ".") ". " }
	</style>
	<ol><li>Alpha<ol><li>Beta</li><li>Gamma</li></ol></li><li>Delta</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	for _, want := range []string{"(1.) Tj", "(1.1) Tj", "(1.2) Tj", "(2.) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing marker %q\nstream:\n%s", want, stream)
		}
	}
	for _, want := range []string{"(Alpha) Tj", "(Beta) Tj", "(Gamma) Tj", "(Delta) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing body %q", want)
		}
	}
	// list-style: none would normally suppress all markers; the only reason
	// numbers appear is the ::marker content, and no default bullet leaks in.
	if strings.Contains(stream, "•") {
		t.Errorf("unexpected default bullet glyph in content-driven markers")
	}
}

// counter() with an explicit style argument inside ::marker content.
func TestMarkerContentCounterStyle(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: c; list-style: none }
		li { counter-increment: c }
		li::marker { content: counter(c, upper-roman) ". " }
	</style>
	<ol><li>One</li><li>Two</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	// "Two" is emitted with kerning as a TJ array, so only "One" is asserted
	// as a literal Tj for body presence; the markers are the load-bearing part.
	for _, want := range []string{"(I.) Tj", "(II.) Tj", "(One) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
}

// content: none on ::marker suppresses the marker entirely while item text
// still renders; the default decimal "1." must be absent.
func TestMarkerContentNoneSuppresses(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: item }
		li { counter-increment: item }
		li::marker { content: none }
	</style>
	<ol><li>X</li><li>Y</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	for _, want := range []string{"(X) Tj", "(Y) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing body %q\nstream:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "(1.) Tj") {
		t.Errorf("content:none did not suppress the default marker (found (1.) Tj)\nstream:\n%s", stream)
	}
}

// Cascade: when multiple ::marker { content } rules match, the highest
// specificity wins. "ol li::marker" outranks "li::marker", so the counter
// marker must win over the literal "LO " (a first-wins bug would pick "LO").
func TestMarkerContentCascadeSpecificity(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: item; list-style: none }
		li { counter-increment: item }
		li::marker { content: "LO " }
		ol li::marker { content: counter(item) ". " }
	</style>
	<ol><li>A</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	if !strings.Contains(stream, "(1.) Tj") {
		t.Errorf("higher-specificity marker did not win\nstream:\n%s", stream)
	}
	if strings.Contains(stream, "(LO) Tj") {
		t.Errorf("lower-specificity marker leaked (first-wins bug)\nstream:\n%s", stream)
	}
}

// Element-path li (block child) with a custom ::marker content. The marker is
// drawn before the element. counter(item) ") " renders ") " with the close
// paren escaped per PDF string rules, so the token is (1\)) Tj.
func TestMarkerContentElementPath(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: item }
		li { counter-increment: item }
		li::marker { content: counter(item) ") " }
	</style>
	<ol><li><p>Body</p></li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	if !strings.Contains(stream, `(1\)) Tj`) {
		t.Errorf("element-path marker missing (1\\)) Tj\nstream:\n%s", stream)
	}
	if !strings.Contains(stream, "(Body) Tj") {
		t.Errorf("element-path body missing (Body) Tj\nstream:\n%s", stream)
	}
}
