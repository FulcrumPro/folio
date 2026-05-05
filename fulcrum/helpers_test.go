package fulcrum

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"

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
// `(<text>) Tj` operator whose preceding `Td` set Y to some value,
// and returns that Y. Used by tests that want to assert relative
// positions of two pieces of text without brittle absolute coordinates.
//
// Returns -1 when the text isn't found. Conservative — finds the
// FIRST occurrence; tests that need every-occurrence behavior should
// extend this rather than reuse it.
func findTextY(pdf []byte, text string) float64 {
	tdAndTj := regexp.MustCompile(`([\d.]+) ([\d.]+) Td\s*\(([^)]+)\)\s*Tj`)
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
			for _, m := range tdAndTj.FindAllStringSubmatch(string(decoded), -1) {
				if m[3] == text {
					y, perr := strconv.ParseFloat(m[2], 64)
					if perr == nil {
						return y
					}
				}
			}
		}
		rest = rest[i+j+10:]
	}
}
