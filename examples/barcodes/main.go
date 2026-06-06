// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Barcodes renders every barcode format Folio supports into a single PDF:
// QR Code (at all four error-correction levels), Code 128, and EAN-13.
// Each symbol is drawn as vector graphics, so it stays sharp at any zoom.
//
// The page is meant to be scanned with a phone or barcode reader to confirm
// the generated symbols decode to the payloads shown beneath them.
//
// Usage:
//
//	go run ./examples/barcodes
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/barcode"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

const (
	qrPayload    = "https://github.com/carlos7ags/folio"
	code128Data  = "FOLIO-2026"
	ean13Data    = "5901234123457"
	captionColor = 0.39 // gray
)

func main() {
	if err := buildDocument().Save("barcodes.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created barcodes.pdf")
}

// buildDocument assembles the one-page barcode showcase and returns the
// ready-to-write Document.
func buildDocument() *document.Document {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetMargins(layout.Margins{Top: 64, Right: 64, Bottom: 64, Left: 64})
	doc.Info.Title = "Barcodes"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Barcodes", layout.H1))
	doc.Add(layout.NewParagraph(
		"Folio generates 1D and 2D barcodes as vector graphics rendered directly "+
			"into the PDF content stream. Scan any symbol below to verify it decodes "+
			"to the value printed beneath it.",
		font.Helvetica, 11,
	).SetLeading(1.5).SetSpaceAfter(10))

	// --- QR Code, one column per error-correction level ---
	doc.Add(layout.NewHeading("QR Code (ISO/IEC 18004)", layout.H2))
	doc.Add(caption(
		"The same payload encoded at each error-correction level. Higher levels " +
			"recover more damage at the cost of a denser symbol.",
	))
	doc.Add(qrLevelTable())
	doc.Add(caption("Payload: " + qrPayload))

	// --- Code 128 ---
	doc.Add(layout.NewHeading("Code 128 (ISO/IEC 15417)", layout.H2))
	doc.Add(caption(
		"A high-density 1D barcode for alphanumeric data such as SKUs and " +
			"tracking identifiers.",
	))
	code128 := mustBarcode(barcode.NewCode128(code128Data))
	doc.Add(layout.NewBarcodeElement(code128, 300).
		SetHeight(64).
		SetAlign(layout.AlignLeft).
		SetAltText("Code 128 barcode encoding " + code128Data))
	doc.Add(caption("Encoded value: " + code128Data))

	// --- EAN-13 ---
	doc.Add(layout.NewHeading("EAN-13 (GS1 General Specifications)", layout.H2))
	doc.Add(caption(
		"The 13-digit retail product barcode. The thirteenth digit is a checksum " +
			"over the first twelve.",
	))
	ean13 := mustBarcode(barcode.NewEAN13(ean13Data))
	doc.Add(layout.NewBarcodeElement(ean13, 240).
		SetHeight(96).
		SetAlign(layout.AlignLeft).
		SetAltText("EAN-13 barcode encoding " + ean13Data))
	doc.Add(caption("Encoded value: " + ean13Data))

	return doc
}

// qrLevelTable lays out one QR symbol per error-correction level in a row,
// each captioned with its level letter and approximate recovery capacity.
func qrLevelTable() *layout.Table {
	levels := []struct {
		name     string
		recovery string
		level    barcode.ECCLevel
	}{
		{"Level L", "~7%", barcode.ECCLevelL},
		{"Level M", "~15%", barcode.ECCLevelM},
		{"Level Q", "~25%", barcode.ECCLevelQ},
		{"Level H", "~30%", barcode.ECCLevelH},
	}

	tbl := layout.NewTable().
		SetColumnUnitWidths([]layout.UnitValue{
			layout.Pct(25), layout.Pct(25), layout.Pct(25), layout.Pct(25),
		})

	noBorder := layout.CellBorders{}

	symbols := tbl.AddRow()
	for _, lv := range levels {
		bc := mustBarcode(barcode.NewQRWithECC(qrPayload, lv.level))
		el := layout.NewBarcodeElement(bc, 96).
			SetAlign(layout.AlignCenter).
			SetAltText(lv.name + " QR code encoding " + qrPayload)
		symbols.AddCellElement(el).
			SetBorders(noBorder).
			SetVAlign(layout.VAlignMiddle).
			SetPadding(8)
	}

	labels := tbl.AddRow()
	for _, lv := range levels {
		labels.AddCellElement(
			layout.NewStyledParagraph(
				layout.NewRun(lv.name, font.HelveticaBold, 9),
				layout.NewRun("  "+lv.recovery, font.Helvetica, 9).
					WithColor(layout.RGB(captionColor, captionColor, captionColor)),
			).SetAlign(layout.AlignCenter),
		).SetBorders(noBorder).SetPadding(4)
	}

	return tbl
}

func caption(text string) *layout.Paragraph {
	return layout.NewStyledParagraph(
		layout.NewRun(text, font.Helvetica, 9).
			WithColor(layout.RGB(captionColor, captionColor, captionColor)),
	).SetLeading(1.4).SetSpaceAfter(8)
}

func mustBarcode(bc *barcode.Barcode, err error) *barcode.Barcode {
	if err != nil {
		panic(err)
	}
	return bc
}
