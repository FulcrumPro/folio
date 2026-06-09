// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Table-rowspan creates a one-page PDF demonstrating cells that span
// multiple rows (rowspan) alongside multi-column spans (colspan).
//
// Usage:
//
//	go run ./examples/table-rowspan
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	if err := buildDocument().Save("table-rowspan.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created table-rowspan.pdf")
}

// buildDocument assembles a page with two tables: the minimal rowspan
// case from issue #357, and a richer schedule grid where a cell spans
// three rows. Extracted from main() so the example test can build the
// same document against an in-memory buffer.
func buildDocument() *document.Document {
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Table Rowspan"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Rowspan", layout.H1))
	doc.Add(layout.NewParagraph(
		"The first column spans both rows; the second column has one cell per row.",
		font.Helvetica, 11,
	))

	basic := layout.NewTable()
	r1 := basic.AddRow()
	r1.AddCell("Span", font.HelveticaBold, 10).SetRowspan(2).SetVAlign(layout.VAlignMiddle)
	r1.AddCell("B1", font.Helvetica, 10)
	r2 := basic.AddRow()
	r2.AddCell("B2", font.Helvetica, 10)
	doc.Add(basic)

	doc.Add(layout.NewHeading("Schedule grid", layout.H2))
	doc.Add(layout.NewParagraph(
		"\"Morning\" spans three time slots; \"All day\" spans the whole column.",
		font.Helvetica, 11,
	))

	sched := layout.NewTable()
	sched.SetColumnWidths([]float64{90, 160, 160})

	h := sched.AddHeaderRow()
	h.AddCell("Block", font.HelveticaBold, 10)
	h.AddCell("Track A", font.HelveticaBold, 10)
	h.AddCell("Track B", font.HelveticaBold, 10)

	row1 := sched.AddRow()
	row1.AddCell("Morning", font.HelveticaBold, 10).SetRowspan(3).SetVAlign(layout.VAlignMiddle)
	row1.AddCell("Registration", font.Helvetica, 10)
	row1.AddCell("All day: help desk", font.Helvetica, 10).SetRowspan(3).SetVAlign(layout.VAlignMiddle)

	row2 := sched.AddRow()
	row2.AddCell("Keynote", font.Helvetica, 10)

	row3 := sched.AddRow()
	row3.AddCell("Workshop intro", font.Helvetica, 10)

	doc.Add(sched)

	return doc
}
