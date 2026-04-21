// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"errors"
	"testing"
)

func TestSizeRegressionGuard_CommitsOnShrink(t *testing.T) {
	baseline := bytes.Repeat([]byte("A"), 100)
	candidate := bytes.Repeat([]byte("B"), 50)

	out, err := sizeRegressionGuard(baseline, func() ([]byte, error) {
		return candidate, nil
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !bytes.Equal(out, candidate) {
		t.Error("guard did not return candidate output on shrink")
	}
}

func TestSizeRegressionGuard_RevertsOnGrow(t *testing.T) {
	baseline := bytes.Repeat([]byte("A"), 50)
	candidate := bytes.Repeat([]byte("B"), 100)

	out, err := sizeRegressionGuard(baseline, func() ([]byte, error) {
		return candidate, nil
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !bytes.Equal(out, baseline) {
		t.Error("guard did not revert to baseline on grow")
	}
}

func TestSizeRegressionGuard_RevertsOnEqualSize(t *testing.T) {
	// Equal sizes mean the candidate gives zero benefit; the guard
	// must keep the baseline so the byte-stable artifact is not
	// perturbed for no reason.
	baseline := bytes.Repeat([]byte("A"), 50)
	candidate := bytes.Repeat([]byte("B"), 50)

	out, err := sizeRegressionGuard(baseline, func() ([]byte, error) {
		return candidate, nil
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !bytes.Equal(out, baseline) {
		t.Error("guard committed a same-size candidate; should keep baseline")
	}
}

func TestSizeRegressionGuard_FallsBackOnError(t *testing.T) {
	baseline := []byte("kept")
	sentinel := errors.New("transform exploded")

	out, err := sizeRegressionGuard(baseline, func() ([]byte, error) {
		return nil, sentinel
	})

	if !bytes.Equal(out, baseline) {
		t.Error("guard did not fall back to baseline on candidate error")
	}
	if err == nil {
		t.Fatal("guard hid the candidate error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("returned err missing candidate sentinel: %v", err)
	}
	if !errors.Is(err, errSizeGuardCandidateFailed) {
		t.Errorf("returned err missing fallback sentinel: %v", err)
	}
}

func TestSizeRegressionGuard_EmptyBaseline(t *testing.T) {
	// An empty baseline means the candidate must produce strictly
	// fewer than zero bytes to win — impossible. Empty input must
	// return empty output without error.
	candidate := []byte("any non-empty output")
	out, err := sizeRegressionGuard(nil, func() ([]byte, error) {
		return candidate, nil
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(out) != 0 {
		t.Errorf("out = %q, want empty (baseline)", out)
	}
}

func TestSizeRegressionGuard_EmptyCandidate(t *testing.T) {
	// A candidate that produces zero bytes is a strict shrink against
	// any non-empty baseline. The guard must commit it (even if the
	// caller might find it surprising, the contract is byte count).
	baseline := []byte("non-empty")
	out, err := sizeRegressionGuard(baseline, func() ([]byte, error) {
		return []byte{}, nil
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(out) != 0 {
		t.Errorf("out = %q, want empty (candidate)", out)
	}
}

func TestSizeRegressionGuard_CandidateInvokedOnce(t *testing.T) {
	// The candidate transform must run exactly once. A naive
	// implementation might re-run it for the size check.
	calls := 0
	_, _ = sizeRegressionGuard([]byte("base"), func() ([]byte, error) {
		calls++
		return []byte("a"), nil
	})
	if calls != 1 {
		t.Errorf("candidate invoked %d times, want 1", calls)
	}
}
