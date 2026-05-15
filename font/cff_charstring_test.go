// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"testing"
)

// Type 2 helper encoders. Mirroring TN #5177 §3.2 encoding rules,
// these produce the byte sequence that the walker should decode back
// to the same integer value. The walker is the unit under test so the
// encoders are intentionally trivial: just enough to drive the
// branches we care about.

func t2Int(v int64) []byte {
	switch {
	case v >= -107 && v <= 107:
		return []byte{byte(v + 139)}
	case v >= 108 && v <= 1131:
		v -= 108
		return []byte{byte(v/256) + 247, byte(v % 256)}
	case v >= -1131 && v <= -108:
		v = -v - 108
		return []byte{byte(v/256) + 251, byte(v % 256)}
	case v >= -32768 && v <= 32767:
		return []byte{28, byte(v >> 8), byte(v)}
	}
	panic("t2Int: value out of range")
}

func appendOps(out []byte, parts ...[]byte) []byte {
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// buildSubrIndex wraps the given subroutine bodies in a cffIndex
// suitable for the walker. The returned INDEX aliases the supplied
// bodies; Object(i) returns the i-th body unmodified.
func buildSubrIndex(t *testing.T, bodies [][]byte) *cffIndex {
	t.Helper()
	raw := writeIndex(t, bodies, 2)
	idx, err := parseCFFIndex(raw, 0)
	if err != nil {
		t.Fatalf("buildSubrIndex parse: %v", err)
	}
	return idx
}

func TestCharstringWalkerNoSubrCalls(t *testing.T) {
	// A charstring that pushes args then issues a moveto-equivalent
	// operator. No subr should be marked reached.
	gsubr := buildSubrIndex(t, [][]byte{{0x0E}, {0x0E}})
	lsubr := buildSubrIndex(t, [][]byte{{0x0E}, {0x0E}})
	w := newCharstringWalker(gsubr, []*cffIndex{lsubr})

	cs := []byte{}
	cs = append(cs, t2Int(0)...)
	cs = append(cs, t2Int(0)...)
	cs = append(cs, t2OpHmoveto)
	cs = append(cs, t2OpEndchar)
	w.Trace(cs, 0)

	for i := range gsubr.count {
		if w.GsubrReached(i) {
			t.Errorf("gsubr %d unexpectedly reached", i)
		}
	}
	for i := range lsubr.count {
		if w.LsubrReached(0, i) {
			t.Errorf("lsubr %d unexpectedly reached", i)
		}
	}
}

func TestCharstringWalkerDirectGsubrCall(t *testing.T) {
	// Build a global subr index with two entries; charstring calls
	// gsubr 1 → biased index = 1 - 107 = -106.
	gsubr := buildSubrIndex(t, [][]byte{
		{t2OpReturn},
		{t2OpReturn},
	})
	w := newCharstringWalker(gsubr, nil)

	cs := appendOps(nil, t2Int(-106), []byte{t2OpCallgsubr, t2OpEndchar})
	w.Trace(cs, 0)

	if !w.GsubrReached(1) {
		t.Error("expected gsubr 1 reached after callgsubr")
	}
	if w.GsubrReached(0) {
		t.Error("gsubr 0 must not be reached")
	}
}

func TestCharstringWalkerTransitiveCalls(t *testing.T) {
	// gsubr 0 calls gsubr 1; charstring calls gsubr 0. Both should
	// end up reached.
	gsubr := buildSubrIndex(t, [][]byte{
		appendOps(nil, t2Int(-106), []byte{t2OpCallgsubr, t2OpReturn}), // gsubr 0 → call gsubr 1
		{t2OpReturn}, // gsubr 1
	})
	w := newCharstringWalker(gsubr, nil)

	cs := appendOps(nil, t2Int(-107), []byte{t2OpCallgsubr, t2OpEndchar}) // call gsubr 0
	w.Trace(cs, 0)

	if !w.GsubrReached(0) || !w.GsubrReached(1) {
		t.Errorf("expected gsubrs 0 and 1 reached, got 0=%v 1=%v",
			w.GsubrReached(0), w.GsubrReached(1))
	}
}

func TestCharstringWalkerLocalSubrCallsRespectFD(t *testing.T) {
	// Two FDs with separate local subrs. Charstring in FD 0 calls
	// lsubr 1; FD 1's local subrs must stay untouched.
	lsubr0 := buildSubrIndex(t, [][]byte{
		{t2OpReturn},
		{t2OpReturn},
	})
	lsubr1 := buildSubrIndex(t, [][]byte{
		{t2OpReturn},
	})
	w := newCharstringWalker(nil, []*cffIndex{lsubr0, lsubr1})

	cs := appendOps(nil, t2Int(-106), []byte{t2OpCallsubr, t2OpEndchar})
	w.Trace(cs, 0)

	if !w.LsubrReached(0, 1) {
		t.Error("expected FD 0 lsubr 1 reached")
	}
	if w.LsubrReached(0, 0) {
		t.Error("FD 0 lsubr 0 must not be reached")
	}
	if w.LsubrReached(1, 0) {
		t.Error("FD 1 lsubrs must not be reached by FD 0 walk")
	}
}

func TestCharstringWalkerHintmaskSkipsMaskBytes(t *testing.T) {
	// 3 hstem pairs (6 operands) + hstemhm establishes 3 stems, then
	// hintmask must skip ceil(3/8)=1 mask byte. After hintmask, a
	// callgsubr must still be located correctly.
	gsubr := buildSubrIndex(t, [][]byte{{t2OpReturn}})
	w := newCharstringWalker(gsubr, nil)

	var cs []byte
	for i := range 6 {
		cs = append(cs, t2Int(int64(i))...)
	}
	cs = append(cs, t2OpHstemhm)
	cs = append(cs, t2OpHintmask)
	cs = append(cs, 0xAA) // 1 mask byte
	cs = append(cs, t2Int(-107)...)
	cs = append(cs, t2OpCallgsubr)
	cs = append(cs, t2OpEndchar)
	w.Trace(cs, 0)

	if !w.GsubrReached(0) {
		t.Error("walker mis-skipped hintmask bytes; gsubr 0 should be reached")
	}
}

func TestCharstringWalkerHintmaskWithImplicitVstem(t *testing.T) {
	// 2 hstem pairs (1 hstem) then hintmask with 2 vstem args on
	// stack (implicit vstem shortcut, TN #5177 §4.3). Total stems
	// = 1 + 1 = 2 → 1 mask byte.
	gsubr := buildSubrIndex(t, [][]byte{{t2OpReturn}})
	w := newCharstringWalker(gsubr, nil)

	var cs []byte
	for i := range 2 {
		cs = append(cs, t2Int(int64(i))...)
	}
	cs = append(cs, t2OpHstem) // 1 hstem
	cs = append(cs, t2Int(0)...)
	cs = append(cs, t2Int(1)...)
	cs = append(cs, t2OpHintmask) // 1 implicit vstem on stack
	cs = append(cs, 0xCC)         // 1 mask byte for 2 stems total
	cs = append(cs, t2Int(-107)...)
	cs = append(cs, t2OpCallgsubr)
	cs = append(cs, t2OpEndchar)
	w.Trace(cs, 0)

	if !w.GsubrReached(0) {
		t.Error("implicit-vstem shortcut mis-counted; gsubr 0 should be reached")
	}
}

func TestCharstringWalkerCyclicSubrTerminates(t *testing.T) {
	// gsubr 0 calls gsubr 0 in an infinite loop. The walker must not
	// recurse forever — the visited-check must short-circuit the
	// second entry.
	gsubr := buildSubrIndex(t, [][]byte{
		appendOps(nil, t2Int(-107), []byte{t2OpCallgsubr, t2OpReturn}),
	})
	w := newCharstringWalker(gsubr, nil)

	cs := appendOps(nil, t2Int(-107), []byte{t2OpCallgsubr, t2OpEndchar})
	w.Trace(cs, 0)

	if !w.GsubrReached(0) {
		t.Error("cyclic subr must still be marked reached on first visit")
	}
	// Bounded work: at most one walk into the top-level charstring +
	// one walk into gsubr 0. If the visited-check regresses, this
	// counter explodes — the test would otherwise just hang and time
	// out the suite. The +2 slack absorbs depth-guard fallback paths
	// that might add a single extra entry.
	if w.walkCalls > 4 {
		t.Errorf("walk invoked %d times on cyclic subr; expected <= 4", w.walkCalls)
	}
}

func TestCharstringWalkerUndecidableCallTriggersFallback(t *testing.T) {
	// A subr that issues a callgsubr without pushing its own
	// argument — relies on caller's stack. Walking the subr in
	// isolation yields an empty stack at the call site; the walker
	// must flip to pessimistic gsubr reachability.
	gsubr := buildSubrIndex(t, [][]byte{
		{t2OpCallgsubr, t2OpReturn}, // gsubr 0: bare callgsubr
		{t2OpReturn},
		{t2OpReturn},
	})
	w := newCharstringWalker(gsubr, nil)

	cs := appendOps(nil, t2Int(-107), []byte{t2OpCallgsubr, t2OpEndchar})
	w.Trace(cs, 0)

	// All gsubrs should now be marked reachable (allGReached set).
	for i := range gsubr.count {
		if !w.GsubrReached(i) {
			t.Errorf("gsubr %d should be marked under pessimistic fallback", i)
		}
	}
}

func TestCharstringWalkerEscapedOperatorClearsStack(t *testing.T) {
	// A 2-byte operator (12 9 = abs) sits between an integer push
	// and a callgsubr. The 2-byte op must consume the operand so
	// callgsubr sees an empty stack and triggers fallback.
	gsubr := buildSubrIndex(t, [][]byte{{t2OpReturn}})
	w := newCharstringWalker(gsubr, nil)

	cs := []byte{}
	cs = append(cs, t2Int(-107)...) // push, would be a valid call arg
	cs = append(cs, t2OpEscape, 9)  // abs — consumes operands
	cs = append(cs, t2OpCallgsubr)  // no arg → fallback
	cs = append(cs, t2OpEndchar)
	w.Trace(cs, 0)

	if !w.allGReached {
		t.Error("expected pessimistic gsubr fallback after 2-byte op cleared stack")
	}
}

func TestCharstringWalkerEndcharStopsScan(t *testing.T) {
	// After endchar, a callgsubr later in the buffer must not be
	// taken (return-from-walk semantics).
	gsubr := buildSubrIndex(t, [][]byte{{t2OpReturn}})
	w := newCharstringWalker(gsubr, nil)

	cs := []byte{t2OpEndchar}
	cs = append(cs, t2Int(-107)...)
	cs = append(cs, t2OpCallgsubr)
	w.Trace(cs, 0)

	if w.GsubrReached(0) {
		t.Error("walk past endchar must not mark trailing subrs")
	}
}

func TestSubrBiasBoundaries(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 107},
		{1, 107},
		{1239, 107},
		{1240, 1131},
		{33899, 1131},
		{33900, 32768},
		{100000, 32768},
	}
	for _, tc := range cases {
		if got := subrBias(tc.n); got != tc.want {
			t.Errorf("subrBias(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// Type 2 hmoveto operator code; not in the const block because no
// other walker logic depends on it (it's just "any operator that
// clears the stack").
const t2OpHmoveto = 22
