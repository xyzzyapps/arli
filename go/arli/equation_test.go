package main

import (
	"fmt"
	"math"
	"testing"
)

func TestEquationAncestor(t *testing.T) {
	src := `
		tensor-def 'Parent (tensor '(x y) '(4 4)
			0 1 0 0
			0 0 1 0
			0 0 0 0
			0 0 0 0)
		(tensor-eq 'Ancestor '(x z) 'step
			'(Parent x z)
			'((Ancestor x y) (Parent y z)))
		tensor-run 4
		tensor-get 'Ancestor
	`
	got := evalAll(t, src).(*Tensor)
	if got.Data[0*4+2] != 1 {
		t.Fatalf("Alice is not an ancestor of Charlie: %v", got.Data)
	}
	if got.Data[0*4+1] != 1 {
		t.Fatalf("Alice is not an ancestor of Bob: %v", got.Data)
	}
	if got.Data[0*4+3] != 0 {
		t.Fatalf("Dana was entailed: %v", got.Data)
	}
}

func TestSigmoidTemperature(t *testing.T) {
	src := `
		tensor-def 'S (tensor '() -1)
		(tensor-eq 'Out '() '(sig 0) '(S))
		tensor-run 1
		tensor-get 'Out
	`
	got := evalAll(t, src).(*Tensor).Data[0]
	if got != 0 {
		t.Fatalf("sig at temperature 0 should be the step function, got %v", got)
	}
	src = `
		tensor-def 'S (tensor '() -1)
		(tensor-eq 'Out '() '(sig 1) '(S))
		tensor-run 1
		tensor-get 'Out
	`
	got = evalAll(t, src).(*Tensor).Data[0]
	want := 1 / (1 + math.Exp(1))
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("sig(x/T) = %v, want %v", got, want)
	}
}

func TestEmbedRelation(t *testing.T) {
	// Dana's embedding has cosine 0.95 with Charlie, so Parent(Bob, Charlie)
	// leaks onto Parent(Bob, Dana). Parent(Alice, Bob) stays exact.
	cos := 0.95
	orth := math.Sqrt(1 - cos*cos)
	src := `
		define R (tensor '(x y) '(4 4)
			0 1 0 0
			0 0 1 0
			0 0 0 0
			0 0 0 0)
		define E (tensor '(p d) '(4 4)
			1 0 0 0
			0 1 0 0
			0 0 1 0
			0 0 ` + format(cos) + ` ` + format(orth) + `)
		tensor-embed-get tensor-embed-rel R E E
	`
	got := evalAll(t, src).(*Tensor)
	if math.Abs(got.Data[0*4+1]-1) > 1e-6 {
		t.Fatalf("Alice–Bob retrieval = %v", got.Data[0*4+1])
	}
	if math.Abs(got.Data[1*4+3]-cos) > 1e-5 {
		t.Fatalf("Bob–Dana leak = %v, want %v", got.Data[1*4+3], cos)
	}
}

func format(v float64) string {
	return fmt.Sprintf("%g", v)
}

func TestLogitEquation(t *testing.T) {
	src := `
		tensor-def 'X (tensor '(n d) '(2 2) 1 2  3 4)
		tensor-def 'W (tensor 'd 1 0)
		tensor-def 'B (tensor '() 0)
		(tensor-eq 'Logit '(n) 'id '((X n d) (W d)) '(B))
		tensor-run 1
		tensor->list tensor-get 'Logit
	`
	last := evalAll(t, src).(ArliList)
	got := []float64{float64(last[0].(ArliFloat)), float64(last[1].(ArliFloat))}
	if math.Abs(got[0]-1) > 1e-6 || math.Abs(got[1]-3) > 1e-6 {
		t.Fatalf("logit = %v, want [1 3]", got)
	}
}

func TestFitEquation(t *testing.T) {
	src := `
		tensor-def 'X (tensor '(n d) '(6 2)
			0 0  0.2 0.1  0.1 0.3
			1 1  0.9 1.1  1.2 0.8)
		tensor-def 'Y (tensor 'n 0 0 0 1 1 1)
		tensor-def 'W (tensor 'd 0 0)
		tensor-def 'B (tensor '() 0)
		tensor-learn 'W
		tensor-learn 'B
		(tensor-eq 'Logit '(n) 'id '((X n d) (W d)) '(B))
		(tensor-loss 'bce 'Logit 'Y)
		(list tensor-fit 80 0.8 tensor-get 'W tensor-get 'B)
	`
	lst := evalAll(t, src).(ArliList)
	loss := float64(lst[0].(ArliFloat))
	w := lst[1].(*Tensor)
	b := lst[2].(*Tensor).Data[0]
	if loss > 0.25 {
		t.Fatalf("equation fit loss %v weights %v bias %v", loss, w.Data, b)
	}
	xs := [][]float64{{0, 0}, {0.2, 0.1}, {0.1, 0.3}, {1, 1}, {0.9, 1.1}, {1.2, 0.8}}
	ys := []float64{0, 0, 0, 1, 1, 1}
	for i, row := range xs {
		z := w.Data[0]*row[0] + w.Data[1]*row[1] + b
		pred := 0.0
		if z > 0 {
			pred = 1
		}
		if pred != ys[i] {
			t.Fatalf("row %d logit %v predicted %v (loss %v)", i, z, pred, loss)
		}
	}
}
