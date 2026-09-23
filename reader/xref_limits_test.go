// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// xrefStreamPDF writes objs as objects 1, 2, ... and then an xref stream
// with /W [1 4 2] that lists them, plus compressed entries (object number
// to object stream number and index). Object 1 must be the catalog.
func xrefStreamPDF(t *testing.T, objs []testObj, compressed map[int][2]int, size int) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n<< %s /Length %d >>\nstream\n", i+1, o.dict, len(o.data))
		if o.data == nil {
			b.Truncate(offsets[i])
			fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o.dict)
			continue
		}
		b.Write(o.data)
		b.WriteString("\nendstream\nendobj\n")
	}
	xsNum, xsOff := len(objs)+1, b.Len()
	var rows bytes.Buffer
	row := func(typ byte, f2 uint32, f3 uint16) {
		rows.WriteByte(typ)
		binary.Write(&rows, binary.BigEndian, f2)
		binary.Write(&rows, binary.BigEndian, f3)
	}
	for n := range size {
		switch c, ok := compressed[n]; {
		case n == 0:
			row(0, 0, 65535)
		case n <= len(objs):
			row(1, uint32(offsets[n-1]), 0)
		case n == xsNum:
			row(1, uint32(xsOff), 0)
		case ok:
			row(2, uint32(c[0]), uint16(c[1]))
		default:
			row(0, 0, 0)
		}
	}
	z := deflate(t, rows.Bytes())
	fmt.Fprintf(&b, "%d 0 obj\n<< /Type /XRef /Size %d /W [1 4 2] /Root 1 0 R /Filter /FlateDecode /Length %d >>\nstream\n", xsNum, size, len(z))
	b.Write(z)
	fmt.Fprintf(&b, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xsOff)
	return b.Bytes()
}

// oneByteXrefPDF is a PDF whose xref stream has 1-byte entries (/W [0 0 1]):
// n entries from n zero bytes, which deflate to about n/1000 bytes.
func oneByteXrefPDF(t *testing.T, n int) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	z := deflate(t, make([]byte, n))
	off := b.Len()
	fmt.Fprintf(&b, "2 0 obj\n<< /Type /XRef /Size %d /W [0 0 1] /Root 1 0 R /Filter /FlateDecode /Length %d >>\nstream\n", n, len(z))
	b.Write(z)
	fmt.Fprintf(&b, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", off)
	return b.Bytes()
}

// TestXrefStreamEntryLimit: MaxObjectCount is enforced while the xref
// fills, not after. 8M 1-byte entries (an 8 KB file) used to fill the map
// to 8M entries (841 MiB peak) before the count check.
func TestXrefStreamEntryLimit(t *testing.T) {
	data := oneByteXrefPDF(t, 200_000)
	var err error
	// Decoding the 200 KB xref stream allocates about 0.3 MB.
	assertAllocBelow(t, 2<<20, func() {
		_, err = ParseWithOptions(data, ReadOptions{
			Strictness:   StrictnessStrict,
			MemoryLimits: MemoryLimits{MaxObjectCount: 1000},
		})
	})
	wantLimitErr(t, err, "xref has more than 1000 objects")
}

// TestXrefTableEntryLimit: the same for a classic xref table.
func TestXrefTableEntryLimit(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	off := b.Len()
	b.WriteString("xref\n0 200000\n")
	b.WriteString(strings.Repeat("0000000009 00000 n \n", 200_000))
	fmt.Fprintf(&b, "trailer\n<< /Size 200000 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", off)
	data := b.Bytes()
	var err error
	assertAllocBelow(t, 2<<20, func() {
		_, err = ParseWithOptions(data, ReadOptions{
			Strictness:   StrictnessStrict,
			MemoryLimits: MemoryLimits{MaxObjectCount: 1000},
		})
	})
	wantLimitErr(t, err, "xref has more than 1000 objects")
}

// TestRepairXrefEntryLimit: xref repair stops at MaxObjectCount too.
func TestRepairXrefEntryLimit(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	for i := 1; i <= 2000; i++ {
		fmt.Fprintf(&b, "%d 0 obj\nnull\nendobj\n", i)
	}
	b.WriteString("trailer\n<< /Size 2001 /Root 1 0 R >>\nstartxref\n999999\n%%EOF\n")
	_, err := repairXref(b.Bytes(), MemoryLimits{MaxObjectCount: 100})
	wantLimitErr(t, err, "xref has more than 100 objects")
	if table, err := repairXref(b.Bytes(), MemoryLimits{}); err != nil || len(table.entries) != 2000 {
		t.Errorf("default limits: err %v", err)
	}
}

// TestXrefStreamSizeLimit: an xref stream is decoded under the configured
// MaxXrefSize. It used to be decoded under the 32 MB default always.
func TestXrefStreamSizeLimit(t *testing.T) {
	_, err := ParseWithOptions(oneByteXrefPDF(t, 200_000), ReadOptions{
		Strictness:   StrictnessStrict,
		MemoryLimits: MemoryLimits{MaxXrefSize: 10_000, MaxObjectCount: -1},
	})
	wantLimitErr(t, err, "exceeded 10000 bytes")
}

// TestXrefLimitsKeepNormalFiles: files within the limits parse as before.
func TestXrefLimitsKeepNormalFiles(t *testing.T) {
	for _, data := range [][]byte{generateTestPDF(t), makePDF(t, "x", 5)} {
		if _, err := ParseWithOptions(data, ReadOptions{Strictness: StrictnessStrict}); err != nil {
			t.Errorf("parse: %v", err)
		}
	}
}

// objectStreamPDF has object stream 3 whose /N claims n pairs: first,
// then n-1 filler "0 0" pairs. The object at /First is the string (found),
// and the xref puts object 5 in the stream at index 0.
func objectStreamPDF(t *testing.T, first string, n int) []byte {
	pairs := first + strings.Repeat("0 0 ", n-1)
	objs := []testObj{
		{dict: "<< /Type /Catalog /Pages 2 0 R >>"},
		{dict: "<< /Type /Pages /Kids [] /Count 0 >>"},
		{dict: fmt.Sprintf("/Type /ObjStm /N %d /First %d /Filter /FlateDecode", n, len(pairs)), data: deflate(t, []byte(pairs+"(found)"))},
	}
	return xrefStreamPDF(t, objs, map[int][2]int{5: {3, 0}}, 6)
}

// TestObjectStreamHeaderReadToIndex: resolving an object reads the header
// pairs up to its index only. The reader used to allocate 16 bytes for
// each of the N pairs first: 2 GB for a 256 MB stream that claims the
// largest /N the size check allows.
func TestObjectStreamHeaderReadToIndex(t *testing.T) {
	const n = 250_000 // 1 MB of header pairs
	r, err := Parse(objectStreamPDF(t, "5 0 ", n))
	if err != nil {
		t.Fatal(err)
	}
	// Decode the object stream first; the resolver caches it, so the
	// measure below covers the header read and the object parse only.
	if _, err := r.ResolveObject(core.NewPdfIndirectReference(3, 0)); err != nil {
		t.Fatal(err)
	}
	var obj core.PdfObject
	assertAllocBelow(t, 1<<20, func() { obj, err = r.ResolveObject(core.NewPdfIndirectReference(5, 0)) })
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := obj.(*core.PdfString); !ok || s.Text() != "found" {
		t.Errorf("object 5 = %v, want (found)", obj)
	}
}

// TestObjectStreamHeaderStillChecked: a bad pair before the index is an error.
func TestObjectStreamHeaderStillChecked(t *testing.T) {
	r, err := Parse(objectStreamPDF(t, "x 0 ", 3))
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.ResolveObject(core.NewPdfIndirectReference(5, 0))
	if err == nil || errors.Is(err, ErrMemoryLimitExceeded) || !strings.Contains(err.Error(), "invalid header at entry 0") {
		t.Errorf("error = %v, want invalid header at entry 0", err)
	}
}
