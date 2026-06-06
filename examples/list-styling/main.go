// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// List-styling demonstrates rich <li> content and styled list-item
// boxes in HTML→PDF conversion:
//
//   - Block-level children of an <li> (<div>, <p>, <br>) lay out as
//     blocks, with the list marker on the first line (issue #342).
//   - An <li> carries its own box: background, border, border-radius,
//     and padding (issue #339).
//   - An inline-block or explicit-width <li> shrinks to fit its
//     content — e.g. a "number in a coloured circle" badge — while a
//     block <li> fills the content column.
//
// Usage:
//
//	go run ./examples/list-styling
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	fhtml "github.com/carlos7ags/folio/html"
)

const listHTML = `<!DOCTYPE html>
<html>
<head>
  <title>List styling</title>
  <style>
    body { font-family: sans-serif; font-size: 11pt; color: #1f2937; }
    h2 { font-size: 13pt; margin: 14pt 0 6pt 0; }
    .step-title { font-weight: bold; }

    /* A list item with block-level children (#342). */
    ol.steps > li { margin-bottom: 8pt; }

    /* A list item rendered as a rounded card (#339). */
    ul.cards > li {
      background: #EEF2FF;
      border: 1pt solid #C7D2FE;
      border-radius: 8pt;
      padding: 8pt 12pt;
      margin-bottom: 6pt;
    }

    /* Badges: a number in a coloured circle (inline-block + width). */
    ul.badges > li {
      display: inline-block;
      width: 26pt;
      height: 26pt;
      line-height: 26pt;
      text-align: center;
      border-radius: 50%;
      color: #fff;
      font-weight: bold;
    }
    ul.badges > li.a { background: #4F46E5; }
    ul.badges > li.b { background: #16A34A; }
    ul.badges > li.c { background: #DC2626; }
  </style>
</head>
<body>
  <h2>1. Block children inside &lt;li&gt; (#342)</h2>
  <ol class="steps">
    <li>
      <span class="step-title">Prepare the document.</span>
      <div>Gather the source files and assets.</div>
      <div>Confirm the page size and margins.</div>
    </li>
    <li>
      <span class="step-title">Render and verify.</span>
      <div>Convert to PDF.</div>
      <div>Check the output against the spec.</div>
    </li>
  </ol>

  <h2>2. List items as rounded cards (#339)</h2>
  <ul class="cards">
    <li>A list item with its own background, border, and rounded corners.</li>
    <li>Each card hugs the content column and keeps its bullet alongside.</li>
  </ul>

  <h2>3. Badges — a number in a coloured circle</h2>
  <ul class="badges">
    <li class="a">1</li>
    <li class="b">2</li>
    <li class="c">3</li>
  </ul>
</body>
</html>`

func main() {
	doc, err := buildDocument()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := doc.Save("list-styling.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created list-styling.pdf")
}

// buildDocument converts the demo HTML into a Document. Extracted from
// main() so the example test can exercise the same construction against
// an in-memory buffer.
func buildDocument() (*document.Document, error) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "List styling"
	doc.Info.Author = "Folio"
	if err := doc.AddHTML(listHTML, &fhtml.Options{}); err != nil {
		return nil, fmt.Errorf("AddHTML: %w", err)
	}
	return doc, nil
}
