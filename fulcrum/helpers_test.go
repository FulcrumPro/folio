package fulcrum

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/document"
)

// newDoc returns an empty US-Letter document. Centralized so every
// fulcrum/* test renders with the same default page setup; test
// fixtures vary their HTML, not their page geometry.
func newDoc() *docWrap {
	return &docWrap{Document: document.NewDocument(document.PageSizeLetter)}
}

// docWrap exists only to give tests a `ToBytes()` shape — the upstream
// Document only has `WriteTo(io.Writer)`. Putting the io.Writer dance
// in one place keeps the per-test code one-liner.
type docWrap struct {
	*document.Document
}

func (d *docWrap) ToBytes() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := d.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// findTextY scans every FlateDecode'd content stream in a PDF for a
// text-show operator (`Tj` or `TJ`) whose preceding `Td` set Y to
// some value, and returns that Y. Handles both:
//
//   X Y Td (text) Tj                — single string show
//   X Y Td [(a) -10 (b) ...] TJ     — kerning-array show; we
//                                     concatenate the parenthesized
//                                     strings ignoring spacing values
//
// Returns -1 when the text isn't found. Conservative — finds the
// FIRST occurrence; tests that need every-occurrence behavior should
// extend this rather than reuse it.
func findTextY(pdf []byte, text string) float64 {
	// Match `X Y Td` followed (after optional whitespace and font /
	// color setup operations) by either `(...) Tj` or `[...] TJ`.
	// Use two separate regexes so the .* non-greedy windows stay tight.
	tjRe := regexp.MustCompile(`([\d.]+) ([\d.]+) Td\s*\(([^)]+)\)\s*Tj`)
	tjArrayRe := regexp.MustCompile(`([\d.]+) ([\d.]+) Td\s*\[([^\]]+)\]\s*TJ`)
	innerStrRe := regexp.MustCompile(`\(([^)]*)\)`)

	rest := pdf
	for {
		i := bytes.Index(rest, []byte("\nstream\n"))
		if i < 0 {
			return -1
		}
		j := bytes.Index(rest[i:], []byte("\nendstream"))
		if j < 0 {
			return -1
		}
		raw := rest[i+8 : i+j]
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err == nil {
			decoded, _ := io.ReadAll(zr)
			zr.Close()
			s := string(decoded)

			for _, m := range tjRe.FindAllStringSubmatch(s, -1) {
				if m[3] == text {
					if y, perr := strconv.ParseFloat(m[2], 64); perr == nil {
						return y
					}
				}
			}
			for _, m := range tjArrayRe.FindAllStringSubmatch(s, -1) {
				var sb strings.Builder
				for _, sm := range innerStrRe.FindAllStringSubmatch(m[3], -1) {
					sb.WriteString(sm[1])
				}
				if sb.String() == text {
					if y, perr := strconv.ParseFloat(m[2], 64); perr == nil {
						return y
					}
				}
			}
		}
		rest = rest[i+j+10:]
	}
}
