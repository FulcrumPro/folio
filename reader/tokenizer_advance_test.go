// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
	"time"
)

// brokenXrefPDF is a small PDF whose xref offset is wrong, so Parse falls
// back to repairXref, which tokenizes the whole file. The stray delimiter
// sits between objects, where binary stream data often puts one.
func brokenXrefPDF(stray string) []byte {
	return []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		stray + "\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>\nendobj\n" +
		"trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n999999\n%%EOF\n")
}

// within fails the test if f does not return within the limit, instead of
// letting a hang run into the package timeout.
func within(t *testing.T, limit time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Fatalf("did not return within %v", limit)
	}
}

// TestTokenizerAdvancesPastStrayDelimiters: Next on ')', '{' or '}' consumes
// the byte and returns it as a keyword. It used to return an empty keyword
// without moving, so every caller that loops until EOF spun forever.
func TestTokenizerAdvancesPastStrayDelimiters(t *testing.T) {
	for _, stray := range []string{")", "{", "}"} {
		t.Run(stray, func(t *testing.T) {
			tok := NewTokenizer([]byte("1 " + stray + " 2"))
			var got []Token
			within(t, 2*time.Second, func() {
				for {
					tk := tok.Next()
					if tk.Type == TokenEOF {
						return
					}
					got = append(got, tk)
				}
			})
			if len(got) != 3 || got[1].Type != TokenKeyword || got[1].Value != stray || got[2].Value != "2" {
				t.Errorf("tokens = %+v, want 1, the keyword %q, 2", got, stray)
			}
		})
	}
}

// TestParseRepairsXrefPastStrayDelimiters: a PDF with a broken xref and a
// stray ')', '{' or '}' comes back from Parse, through repairXref, instead
// of hanging. Parse returns what it returns for the same file without the
// stray byte.
func TestParseRepairsXrefPastStrayDelimiters(t *testing.T) {
	_, controlErr := Parse(brokenXrefPDF("x"))
	for _, stray := range []string{")", "{", "}"} {
		t.Run(stray, func(t *testing.T) {
			var err error
			within(t, 5*time.Second, func() { _, err = Parse(brokenXrefPDF(stray)) })
			if (err == nil) != (controlErr == nil) {
				t.Errorf("Parse error = %v, want the control's %v", err, controlErr)
			}
		})
	}
}

// TestParseContentStreamPastStrayDelimiters: a content stream with a stray
// ')', '{' or '}' parses, the stray byte becomes an operator of its own, and
// the operators around it keep their operands.
func TestParseContentStreamPastStrayDelimiters(t *testing.T) {
	for _, stray := range []string{")", "{", "}"} {
		t.Run(stray, func(t *testing.T) {
			var ops []ContentOp
			within(t, 2*time.Second, func() { ops = ParseContentStream([]byte("q 1 0 0 1 5 5 cm " + stray + " /Im0 Do Q")) })
			var names []string
			for _, op := range ops {
				names = append(names, op.Operator)
			}
			want := []string{"q", "cm", stray, "Do", "Q"}
			if len(names) != len(want) {
				t.Fatalf("operators = %q, want %q", names, want)
			}
			for i := range want {
				if names[i] != want[i] {
					t.Fatalf("operators = %q, want %q", names, want)
				}
			}
			if len(ops[1].Operands) != 6 || len(ops[3].Operands) != 1 || ops[3].Operands[0].Value != "Im0" {
				t.Errorf("cm operands = %d, Do operands = %+v, want 6 and /Im0", len(ops[1].Operands), ops[3].Operands)
			}
		})
	}
}
