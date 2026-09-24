package main

import (
	"fmt"
	"math"

	"github.com/gomlx/gomlx/core/graph"
	"github.com/gomlx/gomlx/core/tensors"
)

// Neural-net words on top of the tensor-logic operators.
//
// nn-attention is scaled dot-product attention, the core of a transformer
// block. nn-tab-attend is the tabular special case those foundation models
// use: each query row attends over the training rows and the labels are mixed
// by those weights, which is one forward pass of in-context prediction.
// nn-fit-logit trains a linear classifier on a numeric table with GoMLX
// reverse-mode gradients.

func nnAttention(q, k, v *Tensor) (*Tensor, error) {
	if q.rank() != 2 || k.rank() != 2 || v.rank() != 2 {
		return nil, fmt.Errorf("nn-attention: Q, K and V must be rank 2")
	}
	if q.Shape[1] != k.Shape[1] || k.Shape[0] != v.Shape[0] {
		return nil, fmt.Errorf("nn-attention: shapes Q %v K %v V %v do not contract", q.Shape, k.Shape, v.Shape)
	}
	scale := 1 / math.Sqrt(float64(q.Shape[1]))
	outAxes := []string{"q", "d"}
	outShape := []int{q.Shape[0], v.Shape[1]}
	return evalGraph(outAxes, outShape, func(in []*graph.Node) *graph.Node {
		scores := graph.MulScalar(graph.Einsum("qd,kd->qk", in[0], in[1]), scale)
		weights := graph.Softmax(scores, 1)
		return graph.Einsum("qk,kd->qd", weights, in[2])
	}, q, k, v)
}

func nnLinear(x, w, b *Tensor) (*Tensor, error) {
	if x.rank() != 2 || w.rank() != 2 {
		return nil, fmt.Errorf("nn-linear: X and W must be rank 2")
	}
	if x.Shape[1] != w.Shape[0] {
		return nil, fmt.Errorf("nn-linear: X is %v and W is %v", x.Shape, w.Shape)
	}
	outShape := []int{x.Shape[0], w.Shape[1]}
	if b.rank() == 1 {
		if b.Shape[0] != outShape[1] {
			return nil, fmt.Errorf("nn-linear: bias length %d, outputs %d", b.Shape[0], outShape[1])
		}
	} else if b.rank() != 0 {
		return nil, fmt.Errorf("nn-linear: bias must be a vector or a scalar")
	}
	return evalGraph([]string{"n", "o"}, outShape, func(in []*graph.Node) *graph.Node {
		y := graph.Einsum("ni,io->no", in[0], in[1])
		bias := in[2]
		if bias.Rank() == 1 {
			bias = graph.ExpandAxes(bias, 0)
		}
		return graph.Add(y, graph.BroadcastToShape(bias, y.Shape()))
	}, x, w, b)
}

// nnTabAttend predicts each query row as a softmax-weighted sum of the
// training labels. Similarity is the scaled dot product of the rows.
func nnTabAttend(trainX, trainY, queryX *Tensor) (*Tensor, error) {
	if trainX.rank() != 2 || queryX.rank() != 2 || trainY.rank() != 1 {
		return nil, fmt.Errorf("nn-tab-attend: train X and query X are matrices, train y is a vector")
	}
	if trainX.Shape[0] != trainY.Shape[0] || trainX.Shape[1] != queryX.Shape[1] {
		return nil, fmt.Errorf("nn-tab-attend: shapes train %v y %v query %v", trainX.Shape, trainY.Shape, queryX.Shape)
	}
	return evalGraph([]string{"m"}, []int{queryX.Shape[0]}, func(in []*graph.Node) *graph.Node {
		// Cosine attention. Raw dots favour long rows, which is the wrong
		// notion of "nearby" for a numeric table.
		rows := graph.L2Normalize(in[0], 1)
		queries := graph.L2Normalize(in[2], 1)
		scores := graph.MulScalar(graph.Einsum("md,nd->mn", queries, rows), 8)
		weights := graph.Softmax(scores, 1)
		return graph.Einsum("mn,n->m", weights, in[1])
	}, trainX, trainY, queryX)
}

func stableBCE(logits, y *graph.Node) *graph.Node {
	zeros := graph.ZerosLike(logits)
	abs := graph.Abs(logits)
	per := graph.Add(
		graph.Sub(graph.Max(logits, zeros), graph.Mul(logits, y)),
		graph.Log1p(graph.Exp(graph.Neg(abs))),
	)
	return graph.ReduceAllMean(per)
}

// fitLogit runs gradient descent on a linear classifier.
// X is [n, d], y is [n] with 0/1 labels. Returns weights [d], a scalar
// bias, and the loss after the last step.
func fitLogit(x, y *Tensor, steps int, lr float64) (*Tensor, *Tensor, float64, error) {
	if x.rank() != 2 || y.rank() != 1 || x.Shape[0] != y.Shape[0] {
		return nil, nil, 0, fmt.Errorf("nn-fit-logit: X is [n,d] and y is [n]")
	}
	if steps < 1 {
		return nil, nil, 0, fmt.Errorf("nn-fit-logit: steps must be positive")
	}
	b, err := backend()
	if err != nil {
		return nil, nil, 0, err
	}
	xT := upload(x)
	yT := upload(y)
	defer xT.FinalizeAll()
	defer yT.FinalizeAll()
	w := make([]float64, x.Shape[1])
	bias := []float64{0}
	wT := tensors.FromFlatDataAndDimensions(w, x.Shape[1])
	bT := tensors.FromFlatDataAndDimensions(bias)

	var execErr error
	exec := graph.MustNewExec(b, func(x, y, w, bias *graph.Node) (*graph.Node, *graph.Node, *graph.Node) {
		logits := graph.Add(
			graph.Einsum("nd,d->n", x, w),
			graph.BroadcastToShape(bias, graph.Einsum("nd,d->n", x, w).Shape()),
		)
		loss := stableBCE(logits, y)
		grads := graph.Gradient(loss, w, bias)
		newW := graph.Sub(w, graph.MulScalar(grads[0], lr))
		newB := graph.Sub(bias, graph.MulScalar(grads[1], lr))
		return loss, newW, newB
	})
	defer exec.Finalize()

	var loss float64
	for i := 0; i < steps; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil && execErr == nil {
					execErr = fmt.Errorf("gomlx: %v", r)
				}
			}()
			if execErr != nil {
				return
			}
			outs, err := exec.Call(xT, yT, wT, bT)
			if err != nil {
				execErr = err
				return
			}
			loss, err = scalarOfTensor(outs[0])
			if err != nil {
				execErr = err
				return
			}
			nextW := outs[1]
			nextB := outs[2]
			wT.FinalizeAll()
			bT.FinalizeAll()
			outs[0].FinalizeAll()
			wT = nextW
			bT = nextB
		}()
		if execErr != nil {
			return nil, nil, 0, execErr
		}
	}
	wd, err := flatOfTensor(wT)
	if err != nil {
		return nil, nil, 0, err
	}
	bd, err := flatOfTensor(bT)
	if err != nil {
		return nil, nil, 0, err
	}
	wT.FinalizeAll()
	bT.FinalizeAll()
	wt := &Tensor{Axes: []string{"d"}, Shape: []int{len(wd)}, Data: wd}
	bt := &Tensor{Axes: nil, Shape: nil, Data: bd}
	return wt, bt, loss, nil
}

func scalarOfTensor(t *tensors.Tensor) (float64, error) {
	flat, err := flatOfTensor(t)
	if err != nil {
		return 0, err
	}
	if len(flat) != 1 {
		return 0, fmt.Errorf("expected a scalar, got %d values", len(flat))
	}
	return flat[0], nil
}

func flatOfTensor(t *tensors.Tensor) ([]float64, error) {
	var flat []float64
	var err error
	readErr := t.ConstFlatData(func(v any) {
		switch d := v.(type) {
		case []float64:
			flat = append([]float64(nil), d...)
		case float64:
			flat = []float64{d}
		default:
			err = fmt.Errorf("gomlx: unexpected result type %T", v)
		}
	})
	if readErr != nil {
		return nil, readErr
	}
	return flat, err
}

func getNNBuiltins() map[string]*ArliBuiltin {
	b := make(map[string]*ArliBuiltin)
	add := func(name string, arity int, fn func([]ArliValue, *Evaluator) (ArliValue, error)) {
		b[name] = &ArliBuiltin{Name: name, Arity: arity, Fn: fn}
	}
	add("nn-linear", 3, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		x, err := needTensor(args[0], "nn-linear")
		if err != nil {
			return nil, err
		}
		w, err := needTensor(args[1], "nn-linear")
		if err != nil {
			return nil, err
		}
		bias, err := needTensor(args[2], "nn-linear")
		if err != nil {
			return nil, err
		}
		return nnLinear(x, w, bias)
	})
	add("nn-attention", 3, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		q, err := needTensor(args[0], "nn-attention")
		if err != nil {
			return nil, err
		}
		k, err := needTensor(args[1], "nn-attention")
		if err != nil {
			return nil, err
		}
		v, err := needTensor(args[2], "nn-attention")
		if err != nil {
			return nil, err
		}
		return nnAttention(q, k, v)
	})
	add("nn-tab-attend", 3, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		x, err := needTensor(args[0], "nn-tab-attend")
		if err != nil {
			return nil, err
		}
		y, err := needTensor(args[1], "nn-tab-attend")
		if err != nil {
			return nil, err
		}
		q, err := needTensor(args[2], "nn-tab-attend")
		if err != nil {
			return nil, err
		}
		return nnTabAttend(x, y, q)
	})
	add("demo-zero-temp", 0, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		text, err := zeroTempReport()
		if err != nil {
			return nil, err
		}
		fmt.Print(text)
		return nil, nil
	})
	add("nn-fit-logit", 4, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		x, err := needTensor(args[0], "nn-fit-logit")
		if err != nil {
			return nil, err
		}
		y, err := needTensor(args[1], "nn-fit-logit")
		if err != nil {
			return nil, err
		}
		steps, ok := args[2].(ArliInt)
		if !ok {
			return nil, fmt.Errorf("nn-fit-logit: steps must be an integer")
		}
		lr, ok := scalarOf(args[3])
		if !ok {
			return nil, fmt.Errorf("nn-fit-logit: learning rate must be a number")
		}
		w, bias, loss, err := fitLogit(x, y, int(steps), lr)
		if err != nil {
			return nil, err
		}
		return ArliList{w, bias, ArliFloat(loss)}, nil
	})
	return b
}
