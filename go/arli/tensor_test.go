package main

import (
	"math"
	"testing"
)

func nearly(t *testing.T, got []float64, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len %d, want %d (%v vs %v)", len(got), len(want), got, want)
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("element %d: got %v want %v (full %v)", i, got[i], want[i], got)
		}
	}
}

func dataOf(t *testing.T, v ArliValue) []float64 {
	t.Helper()
	ten, ok := v.(*Tensor)
	if !ok {
		t.Fatalf("not a tensor: %s", v.ArliRepr())
	}
	return ten.Data
}

func evalAll(t *testing.T, source string) ArliValue {
	t.Helper()
	ev := NewEvaluator()
	tokens := Tokenize(source)
	stream := NewTokenStream(tokens)
	var last ArliValue
	for !stream.IsEOF() {
		expr := ev.parser.parseExpr(stream, true)
		if expr == nil {
			continue
		}
		result, err := ev.Eval(expr)
		if err != nil {
			t.Fatalf("eval %q: %v", source, err)
		}
		last = result
	}
	return last
}

func TestPerceptronStep(t *testing.T) {
	// Y = step(W[i] X[i]) from the paper: W = [0.2, 1.9, -0.7, 3], X = [0, 1, 1, 0].
	src := `
		define W (tensor 'i 0.2 1.9 -0.7 3)
		define X (tensor 'i 0 1 1 0)
		tensor-step tensor-project tensor-join W X 'i
	`
	got := dataOf(t, evalAll(t, src))
	nearly(t, got, []float64{1})
}

func TestEinsumMatmul(t *testing.T) {
	src := `
		define A (tensor '(i j) '(2 2) 1 2 3 4)
		define B (tensor '(j k) '(2 2) 5 6 7 8)
		(tensor-einsum "ij,jk->ik" A B)
	`
	got := dataOf(t, evalAll(t, src))
	nearly(t, got, []float64{19, 22, 43, 50})
}

func TestJoinThenProjectMatchesEinsum(t *testing.T) {
	src := `
		define A (tensor '(i j) '(2 2) 1 2 3 4)
		define B (tensor '(j k) '(2 2) 5 6 7 8)
		tensor-project tensor-join A B 'j
	`
	got := dataOf(t, evalAll(t, src))
	nearly(t, got, []float64{19, 22, 43, 50})
}

func TestBooleanJoinIsDatalog(t *testing.T) {
	// Aunt(x,z) <- Sister(x,y), Parent(y,z), then the step function.
	// Sister(0,1) and Parent(1,2) imply Aunt(0,2).
	src := `
		define S (tensor '(x y) '(3 3) 0 1 0  0 0 0  0 0 0)
		define P (tensor '(y z) '(3 3) 0 0 0  0 0 1  0 0 0)
		tensor-step tensor-project tensor-join S P 'y
	`
	got := dataOf(t, evalAll(t, src))
	want := make([]float64, 9)
	want[2] = 1 // x=0, z=2
	nearly(t, got, want)
}

func TestSoftmax(t *testing.T) {
	src := `tensor-softmax (tensor 'p 1 2 3) 'p`
	got := dataOf(t, evalAll(t, src))
	sum := got[0] + got[1] + got[2]
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("softmax sums to %v", sum)
	}
	if !(got[0] < got[1] && got[1] < got[2]) {
		t.Fatalf("softmax not ordered: %v", got)
	}
}

func TestMaxAndMean(t *testing.T) {
	src := `
		define M (tensor '(i j) '(2 2) 1 5 3 4)
		tensor->list tensor-max M 'j
	`
	last := evalAll(t, src)
	lst := last.(ArliList)
	nearly(t, []float64{float64(lst[0].(ArliFloat)), float64(lst[1].(ArliFloat))}, []float64{5, 4})

	src = `tensor->list tensor-mean (tensor '(i j) '(2 2) 1 5 3 4) 'j`
	last = evalAll(t, src)
	lst = last.(ArliList)
	nearly(t, []float64{float64(lst[0].(ArliFloat)), float64(lst[1].(ArliFloat))}, []float64{3, 3.5})
}

func TestOuterProduct(t *testing.T) {
	// No shared axis: join is the tensor product.
	src := `tensor->list tensor-join (tensor 'i 2 3) (tensor 'j 4 5)`
	last := evalAll(t, src)
	lst := last.(ArliList)
	got := make([]float64, len(lst))
	for i, v := range lst {
		got[i] = float64(v.(ArliFloat))
	}
	nearly(t, got, []float64{8, 10, 12, 15})
}
