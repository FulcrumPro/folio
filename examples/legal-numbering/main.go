// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Legal-numbering demonstrates multi-level clause numbering in a legal
// agreement using native CSS counters — no markup workarounds:
//
//   - ol { counter-reset: item; list-style: none }
//     li { counter-increment: item }
//     li::marker { content: counters(item, ".") ". " }
//
// counters(item, ".") joins the whole counter stack, so nesting yields
// "1.", "1.1.", "1.1.1.", "2." automatically. The marker sits in the left
// gutter (list-style-position: outside, the default) so long clause bodies
// wrap with a hanging indent under the clause text rather than under the
// number.
//
// Usage:
//
//	go run ./examples/legal-numbering
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	fhtml "github.com/carlos7ags/folio/html"
)

const legalHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Master Services Agreement</title>
  <style>
    body { font-family: serif; font-size: 11pt; color: #111827; }
    h1 { font-size: 16pt; margin: 0 0 12pt 0; text-align: center; }

    /* Native multi-level legal numbering via counters() + ::marker. */
    ol { counter-reset: item; list-style: none; }
    li { counter-increment: item; margin-bottom: 6pt; }
    li::marker { content: counters(item, ".") ". "; }

    .clause-title { font-weight: bold; }
  </style>
</head>
<body>
  <h1>Master Services Agreement</h1>

  <ol>
    <li>
      <span class="clause-title">Definitions.</span>
      <p>In this Agreement, the following terms have the meanings set out below,
         and these definitions apply both to the singular and plural forms of
         such terms and to every clause in which they appear throughout the body
         of this Agreement.</p>
      <ol>
        <li>"Agreement" means this Master Services Agreement together with all
            schedules, exhibits, and statements of work executed under it.</li>
        <li>"Services" means the professional services described in each
            statement of work agreed between the parties.
          <ol>
            <li>Each statement of work is incorporated into and governed by the
                terms of this Agreement upon signature by both parties.</li>
            <li>In the event of a conflict between a statement of work and this
                Agreement, this Agreement controls except where the statement of
                work expressly states otherwise.</li>
          </ol>
        </li>
      </ol>
    </li>
    <li>
      <span class="clause-title">Term and Termination.</span>
      <p>This Agreement commences on the Effective Date and continues until
         terminated in accordance with this clause.</p>
      <ol>
        <li>Either party may terminate this Agreement for convenience on thirty
            (30) days' prior written notice to the other party.</li>
        <li>Either party may terminate this Agreement immediately if the other
            party commits a material breach that remains uncured for fifteen
            (15) days after written notice describing the breach in reasonable
            detail.</li>
      </ol>
    </li>
    <li>
      <span class="clause-title">Confidentiality.</span>
      <p>Each party shall protect the other party's Confidential Information
         using at least the same degree of care it uses to protect its own
         confidential information of a similar nature.</p>
    </li>
  </ol>
</body>
</html>`

func main() {
	doc, err := buildDocument()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := doc.Save("legal-numbering.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created legal-numbering.pdf")
}

// buildDocument converts the demo HTML into a Document. Extracted from
// main() so the example test can exercise the same construction against an
// in-memory buffer.
func buildDocument() (*document.Document, error) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Master Services Agreement"
	doc.Info.Author = "Folio"
	if err := doc.AddHTML(legalHTML, &fhtml.Options{}); err != nil {
		return nil, fmt.Errorf("AddHTML: %w", err)
	}
	return doc, nil
}
