package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/gomlx/compute"
	"github.com/gomlx/gomlx/core/graph"
	"github.com/gomlx/gomlx/core/tensors"

	// Registers the pure-Go backend ("go") and, on this platform, XLA.
	// WebGPU is the ONNX Runtime provider (GOMLX_BACKEND=onnx:webgpu); that
	// provider is built into GoMLX for Linux and WebAssembly, not native Windows.
	_ "github.com/gomlx/gomlx/backends/default"
)

func init() {
	// The portable backend needs no GPU driver. Set GOMLX_BACKEND to
	// "xla:cuda" or "onnx:webgpu" to run the same graphs on a device.
	if os.Getenv("GOMLX_BACKEND") == "" {
		os.Setenv("GOMLX_BACKEND", "go")
	}
}

var (
	mlOnce    sync.Once
	mlBackend compute.Backend
	mlErr     error
)

func backend() (compute.Backend, error) {
	mlOnce.Do(func() {
		mlBackend, mlErr = compute.New()
	})
	return mlBackend, mlErr
}

// call runs one GoMLX graph and copies the flat result back. GoMLX reports
// shape errors by panicking; those become ordinary errors.
func call(fn func(inputs []*graph.Node) *graph.Node, inputs ...*tensors.Tensor) ([]float64, error) {
	b, err := backend()
	if err != nil {
		return nil, err
	}
	args := make([]any, len(inputs))
	for i, t := range inputs {
		args[i] = t
	}
	var out *tensors.Tensor
	func() {
		defer func() {
			if r := recover(); r != nil && err == nil {
				err = fmt.Errorf("gomlx: %v", r)
			}
		}()
		out, err = graph.CallOnce(b, fn, args...)
	}()
	if err != nil {
		return nil, err
	}
	defer out.FinalizeAll()
	var flat []float64
	err = out.ConstFlatData(func(v any) {
		switch d := v.(type) {
		case []float64:
			flat = append([]float64(nil), d...)
		case float64:
			flat = []float64{d}
		default:
			err = fmt.Errorf("gomlx: unexpected result type %T", v)
		}
	})
	if err != nil {
		return nil, err
	}
	return flat, nil
}

func upload(t *Tensor) *tensors.Tensor {
	return tensors.FromFlatDataAndDimensions(t.Data, t.Shape...)
}

func evalGraph(axes []string, shape []int, fn func(inputs []*graph.Node) *graph.Node, inputs ...*Tensor) (*Tensor, error) {
	uploaded := make([]*tensors.Tensor, len(inputs))
	for i, t := range inputs {
		uploaded[i] = upload(t)
		defer uploaded[i].FinalizeAll()
	}
	data, err := call(fn, uploaded...)
	if err != nil {
		return nil, err
	}
	if len(shape) == 0 {
		if len(data) != 1 {
			return nil, fmt.Errorf("gomlx: scalar result has %d values", len(data))
		}
	} else if len(data) != numel(shape) {
		return nil, fmt.Errorf("gomlx: result has %d values, shape needs %d", len(data), numel(shape))
	}
	return &Tensor{Axes: axes, Shape: shape, Data: data}, nil
}

func lettersFor(groups ...[]string) (map[string]string, error) {
	seen := map[string]string{}
	n := 0
	for _, g := range groups {
		for _, name := range g {
			if _, ok := seen[name]; ok {
				continue
			}
			if n >= 26 {
				return nil, fmt.Errorf("tensor: more than 26 distinct axes")
			}
			seen[name] = string(rune('a' + n))
			n++
		}
	}
	return seen, nil
}

func eqOf(axes []string, letters map[string]string) string {
	s := ""
	for _, a := range axes {
		s += letters[a]
	}
	return s
}

func mlJoin(a, b *Tensor, outAxes []string, outShape []int) (*Tensor, error) {
	letters, err := lettersFor(a.Axes, b.Axes, outAxes)
	if err != nil {
		return nil, err
	}
	eq := eqOf(a.Axes, letters) + "," + eqOf(b.Axes, letters) + "->" + eqOf(outAxes, letters)
	return evalGraph(outAxes, outShape, func(in []*graph.Node) *graph.Node {
		return graph.Einsum(eq, in[0], in[1])
	}, a, b)
}

func mlReduce(t *Tensor, axes []int, mode string, outAxes []string, outShape []int) (*Tensor, error) {
	return evalGraph(outAxes, outShape, func(in []*graph.Node) *graph.Node {
		switch mode {
		case "max":
			return graph.ReduceMax(in[0], axes...)
		case "mean":
			return graph.ReduceMean(in[0], axes...)
		default:
			return graph.ReduceSum(in[0], axes...)
		}
	}, t)
}

func mlBinary(a, b *Tensor, name string, outAxes []string, outShape []int) (*Tensor, error) {
	return evalGraph(outAxes, outShape, func(in []*graph.Node) *graph.Node {
		lhs, rhs := in[0], in[1]
		if rhs.Rank() == 0 && lhs.Rank() > 0 {
			rhs = graph.BroadcastToShape(rhs, lhs.Shape())
		} else if lhs.Rank() == 0 && rhs.Rank() > 0 {
			lhs = graph.BroadcastToShape(lhs, rhs.Shape())
		}
		switch name {
		case "tensor-sub":
			return graph.Sub(lhs, rhs)
		case "tensor-mul":
			return graph.Mul(lhs, rhs)
		default:
			return graph.Add(lhs, rhs)
		}
	}, a, b)
}

func mlMap(t *Tensor, which string) (*Tensor, error) {
	return evalGraph(t.Axes, t.Shape, func(in []*graph.Node) *graph.Node {
		x := in[0]
		switch which {
		case "step":
			pos := graph.GreaterThan(x, graph.ScalarZero(x.Graph(), x.DType()))
			return graph.Where(pos, graph.OnesLike(x), graph.ZerosLike(x))
		case "relu":
			return graph.Max(x, graph.ZerosLike(x))
		case "sig":
			return graph.Sigmoid(x)
		default:
			return graph.Exp(x)
		}
	}, t)
}

func mlSoftmax(t *Tensor, axis int) (*Tensor, error) {
	return evalGraph(t.Axes, t.Shape, func(in []*graph.Node) *graph.Node {
		return graph.Softmax(in[0], axis)
	}, t)
}

func mlLnorm(t *Tensor, axis int) (*Tensor, error) {
	return evalGraph(t.Axes, t.Shape, func(in []*graph.Node) *graph.Node {
		x := in[0]
		mean := graph.ReduceAndKeep(x, graph.ReduceMean, axis)
		centered := graph.Sub(x, mean)
		variance := graph.ReduceAndKeep(graph.Mul(centered, centered), graph.ReduceMean, axis)
		scale := graph.Sqrt(graph.AddScalar(variance, 1e-5))
		return graph.Div(centered, scale)
	}, t)
}

func mlEinsum(spec string, inputs []*Tensor, outAxes []string, outShape []int) (*Tensor, error) {
	if len(inputs) != 2 {
		return nil, fmt.Errorf("tensor-einsum: GoMLX contracts two tensors at a time, got %d", len(inputs))
	}
	return evalGraph(outAxes, outShape, func(in []*graph.Node) *graph.Node {
		return graph.Einsum(spec, in[0], in[1])
	}, inputs...)
}
