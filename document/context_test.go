// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/carlos7ags/folio/core"
)

func docWithContent() *Document {
	d := NewDocument(PageSizeLetter)
	_ = d.AddHTML("<html><body><p>hello world</p></body></html>", nil)
	return d
}

// TestWriteToWithContextCancelled verifies WriteToWithContext fails closed on
// a cancelled context.
func TestWriteToWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	_, err := docWithContent().WriteToWithContext(ctx, &buf, WriteOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// TestWriteToWithContextBackgroundOK confirms a live context writes a normal
// document (the WriteToWithOptions/WriteTo delegation path).
func TestWriteToWithContextBackgroundOK(t *testing.T) {
	var buf bytes.Buffer
	n, err := docWithContent().WriteToWithContext(context.Background(), &buf, WriteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n == 0 || buf.Len() == 0 {
		t.Fatal("expected bytes to be written with a live context")
	}
}

// TestAddHTMLWithContextCancelled verifies the HTML conversion entry point
// honors cancellation.
func TestAddHTMLWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := NewDocument(PageSizeLetter)
	err := d.AddHTMLWithContext(ctx, "<html><body><p>x</p><p>y</p></body></html>", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// TestWriterObjectLoopContextCancelled verifies the serialization loop checks
// the context at object boundaries.
func TestWriterObjectLoopContextCancelled(t *testing.T) {
	w := NewWriter("1.7")
	w.SetRoot(w.AddObject(core.NewPdfInteger(1)))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.ctx = ctx

	_, err := w.WriteToWithOptions(io.Discard, WriteOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
