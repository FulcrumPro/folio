package fulcrum

import (
	"bytes"

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
