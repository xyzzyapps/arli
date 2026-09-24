package main

import (
	"math"
	"testing"
)

func TestAttentionPrefersMatchingKey(t *testing.T) {
	// Query matches the first key. The value pulled back should be near [10, 0].
	src := `
		define Q (tensor '(q d) '(1 2) 1 0)
		define K (tensor '(k d) '(2 2) 1 0  0 1)
		define V (tensor '(k d) '(2 2) 10 0  0 1)
		tensor->list nn-attention Q K V
	`
	last := evalAll(t, src).(ArliList)
	got := []float64{float64(last[0].(ArliFloat)), float64(last[1].(ArliFloat))}
	if got[0] < got[1]*10 {
		t.Fatalf("attention did not attend to the matching key: %v", got)
	}
}

func TestLinear(t *testing.T) {
	src := `tensor->list nn-linear (tensor '(n i) '(1 2) 1 2) (tensor '(i o) '(2 1) 3 4) (tensor 'o 1)`
	last := evalAll(t, src).(ArliList)
	got := float64(last[0].(ArliFloat))
	if math.Abs(got-12) > 1e-6 {
		t.Fatalf("linear: got %v", got)
	}
}

func TestTabularAttend(t *testing.T) {
	// Two clusters. A query next to the class-1 rows should come back near 1.
	src := `
		define X (tensor '(n d) '(4 2) 1 0  0.9 0.1  0 1  0.1 0.9)
		define y (tensor 'n 0 0 1 1)
		define q (tensor '(m d) '(2 2) 0.05 1  1 0.05)
		tensor->list nn-tab-attend X y q
	`
	last := evalAll(t, src).(ArliList)
	hi := float64(last[0].(ArliFloat))
	lo := float64(last[1].(ArliFloat))
	if hi < 0.8 || lo > 0.2 {
		t.Fatalf("tabular attention: class1=%v class0=%v", hi, lo)
	}
}

func TestFitLogitSeparates(t *testing.T) {
	src := `
		define X (tensor '(n d) '(6 2)
			0 0  0.2 0.1  0.1 0.3
			1 1  0.9 1.1  1.2 0.8)
		define y (tensor 'n 0 0 0 1 1 1)
		nn-fit-logit X y 80 0.8
	`
	last := evalAll(t, src).(ArliList)
	loss := float64(last[2].(ArliFloat))
	if loss > 0.2 {
		t.Fatalf("logit did not fit, loss %v", loss)
	}
	w := last[0].(*Tensor)
	b := last[1].(*Tensor).Data[0]
	// Every training row should be on the correct side of the decision line.
	xs := [][]float64{{0, 0}, {0.2, 0.1}, {0.1, 0.3}, {1, 1}, {0.9, 1.1}, {1.2, 0.8}}
	ys := []float64{0, 0, 0, 1, 1, 1}
	for i, row := range xs {
		z := w.Data[0]*row[0] + w.Data[1]*row[1] + b
		pred := 0.0
		if z > 0 {
			pred = 1
		}
		if pred != ys[i] {
			t.Fatalf("row %d logit %v predicted %v", i, z, pred)
		}
	}
}
