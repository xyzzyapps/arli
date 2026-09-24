package main

import (
	"fmt"
	"strings"
)

// Tensor is a dense row-major array with named axes.
//
// Axis names are the indices of a tensor-logic equation. tensor-join
// multiplies two tensors on the axes whose names match (Hadamard when every
// axis is shared, outer product when none are) and keeps the shared axes.
// tensor-project sums those axes away. tensor-einsum is the same pair of
// operations written as a NumPy-style subscript, "ij,jk->ik".
//
// Axes whose names start with "_" are private: they never match another
// tensor's axes. (tensor '(2 3) ...) builds private axes _0, _1.
type Tensor struct {
	Axes  []string
	Shape []int
	Data  []float64
}

func (t *Tensor) ArliRepr() string {
	if len(t.Axes) == 0 {
		return fmt.Sprintf("<tensor %g>", t.Data[0])
	}
	parts := make([]string, len(t.Axes))
	for i, name := range t.Axes {
		parts[i] = fmt.Sprintf("%s:%d", name, t.Shape[i])
	}
	return "<tensor " + strings.Join(parts, " ") + ">"
}

func (t *Tensor) rank() int { return len(t.Shape) }

func (t *Tensor) strides() []int {
	s := make([]int, len(t.Shape))
	acc := 1
	for i := len(t.Shape) - 1; i >= 0; i-- {
		s[i] = acc
		acc *= t.Shape[i]
	}
	return s
}

func (t *Tensor) offset(idx []int) int {
	st := t.strides()
	off := 0
	for i, v := range idx {
		off += v * st[i]
	}
	return off
}

func (t *Tensor) clone() *Tensor {
	axes := append([]string{}, t.Axes...)
	shape := append([]int{}, t.Shape...)
	data := append([]float64{}, t.Data...)
	return &Tensor{Axes: axes, Shape: shape, Data: data}
}

func (t *Tensor) axisIndex(name string) (int, bool) {
	for i, a := range t.Axes {
		if a == name {
			return i, true
		}
	}
	return 0, false
}

func numel(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

func newTensor(axes []string, shape []int, data []float64) (*Tensor, error) {
	if len(axes) != len(shape) {
		return nil, fmt.Errorf("tensor: %d axes for rank %d", len(axes), len(shape))
	}
	for i, d := range shape {
		if d <= 0 {
			return nil, fmt.Errorf("tensor: axis %s has size %d", axes[i], d)
		}
	}
	if len(data) != numel(shape) {
		return nil, fmt.Errorf("tensor: got %d elements, shape needs %d", len(data), numel(shape))
	}
	seen := map[string]bool{}
	for _, a := range axes {
		if seen[a] {
			return nil, fmt.Errorf("tensor: repeated axis %s", a)
		}
		seen[a] = true
	}
	return &Tensor{Axes: axes, Shape: append([]int{}, shape...), Data: data}, nil
}

func isPublicAxis(name string) bool {
	return name != "" && !strings.HasPrefix(name, "_")
}

// ---------------------------------------------------------------------------
// Construction from arli values
// ---------------------------------------------------------------------------

func asNumberList(v ArliValue) ([]float64, bool) {
	lst, ok := v.(ArliList)
	if !ok {
		return nil, false
	}
	out := make([]float64, len(lst))
	for i, el := range lst {
		switch n := el.(type) {
		case ArliInt:
			out[i] = float64(n)
		case ArliFloat:
			out[i] = float64(n)
		default:
			return nil, false
		}
	}
	return out, true
}

func asIntList(v ArliValue) ([]int, bool) {
	lst, ok := v.(ArliList)
	if !ok {
		return nil, false
	}
	out := make([]int, len(lst))
	for i, el := range lst {
		n, ok := el.(ArliInt)
		if !ok || n <= 0 {
			return nil, false
		}
		out[i] = int(n)
	}
	return out, true
}

func asSymbolList(v ArliValue) ([]string, bool) {
	lst, ok := v.(ArliList)
	if !ok {
		return nil, false
	}
	out := make([]string, len(lst))
	for i, el := range lst {
		s, ok := el.(ArliSymbol)
		if !ok {
			return nil, false
		}
		out[i] = string(s)
	}
	return out, true
}

func numbersFromArgs(args []ArliValue) ([]float64, error) {
	out := make([]float64, 0, len(args))
	for _, a := range args {
		switch n := a.(type) {
		case ArliInt:
			out = append(out, float64(n))
		case ArliFloat:
			out = append(out, float64(n))
		case ArliList:
			flat, ok := asNumberList(n)
			if !ok {
				return nil, fmt.Errorf("tensor: expected numbers")
			}
			out = append(out, flat...)
		default:
			return nil, fmt.Errorf("tensor: expected numbers, got %s", a.ArliRepr())
		}
	}
	return out, nil
}

// tensorArgs accepts:
//
//	(tensor 1 2 3)                  vector, private axis _0
//	(tensor 'i 1 2 3)               vector named i
//	(tensor '(2 3) 1 2 3 4 5 6)     shape, private axes
//	(tensor '(i j) '(2 3) ...)      named axes and shape
//	(tensor '() 3.5)                rank-0 scalar
func tensorFromArgs(args []ArliValue) (*Tensor, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("tensor: expected data")
	}
	if s, ok := args[0].(ArliSymbol); ok {
		data, err := numbersFromArgs(args[1:])
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("tensor: expected data")
		}
		return newTensor([]string{string(s)}, []int{len(data)}, data)
	}
	if axes, ok := asSymbolList(args[0]); ok {
		if len(args) >= 2 {
			if shape, ok := asIntList(args[1]); ok {
				data, err := numbersFromArgs(args[2:])
				if err != nil {
					return nil, err
				}
				return newTensor(axes, shape, data)
			}
		}
		data, err := numbersFromArgs(args[1:])
		if err != nil {
			return nil, err
		}
		if len(axes) == 0 {
			if len(data) != 1 {
				return nil, fmt.Errorf("tensor: a scalar takes one number")
			}
			return newTensor(nil, nil, data)
		}
		if len(axes) != 1 {
			return nil, fmt.Errorf("tensor: rank %d needs an explicit shape", len(axes))
		}
		return newTensor(axes, []int{len(data)}, data)
	}
	if shape, ok := asIntList(args[0]); ok {
		data, err := numbersFromArgs(args[1:])
		if err != nil {
			return nil, err
		}
		axes := make([]string, len(shape))
		for i := range shape {
			axes[i] = fmt.Sprintf("_%d", i)
		}
		return newTensor(axes, shape, data)
	}
	data, err := numbersFromArgs(args)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("tensor: expected data")
	}
	return newTensor([]string{"_0"}, []int{len(data)}, data)
}

func needTensor(v ArliValue, what string) (*Tensor, error) {
	t, ok := v.(*Tensor)
	if !ok {
		return nil, fmt.Errorf("%s: expected a tensor, got %s", what, v.ArliRepr())
	}
	return t, nil
}

func axisNames(v ArliValue) ([]string, error) {
	if s, ok := v.(ArliSymbol); ok {
		return []string{string(s)}, nil
	}
	if names, ok := asSymbolList(v); ok {
		return names, nil
	}
	return nil, fmt.Errorf("expected an axis name or a list of them, got %s", v.ArliRepr())
}

// ---------------------------------------------------------------------------
// Join and projection — the two tensor-logic operators
// ---------------------------------------------------------------------------

// privatize renames non-public axes so two independently built tensors
// do not join on an accidental _0.
func (t *Tensor) privatize(prefix string) *Tensor {
	out := t.clone()
	used := map[string]bool{}
	for _, a := range out.Axes {
		used[a] = true
	}
	for i, a := range out.Axes {
		if isPublicAxis(a) {
			continue
		}
		n := 0
		for {
			name := fmt.Sprintf("_%s%d", prefix, n)
			n++
			if !used[name] {
				used[name] = true
				out.Axes[i] = name
				break
			}
		}
	}
	return out
}

// join is the tensor-logic join: one element per pair of elements that
// agree on the shared axes, and that element is their product.
func joinTensors(a, b *Tensor) (*Tensor, error) {
	a = a.privatize("a")
	b = b.privatize("b")
	bPos := map[string]int{}
	for i, name := range b.Axes {
		bPos[name] = i
	}
	aPos := map[string]int{}
	for i, name := range a.Axes {
		aPos[name] = i
		if j, ok := bPos[name]; ok && a.Shape[i] != b.Shape[j] {
			return nil, fmt.Errorf("tensor-join: axis %s is %d in the first tensor and %d in the second", name, a.Shape[i], b.Shape[j])
		}
	}
	outAxes := append([]string{}, a.Axes...)
	for _, name := range b.Axes {
		if _, ok := aPos[name]; !ok {
			outAxes = append(outAxes, name)
		}
	}
	outShape := make([]int, len(outAxes))
	for i, name := range outAxes {
		if j, ok := aPos[name]; ok {
			outShape[i] = a.Shape[j]
		} else {
			outShape[i] = b.Shape[bPos[name]]
		}
	}
	return mlJoin(a, b, outAxes, outShape)
}

// project reduces axes by sum, max, or mean. Sum is database-style
// projection; max and mean are the paper's max= and avg= aggregators.
func projectTensor(t *Tensor, axes []string, mode string) (*Tensor, error) {
	drop := map[string]bool{}
	for _, a := range axes {
		if _, ok := t.axisIndex(a); !ok {
			return nil, fmt.Errorf("tensor-%s: tensor has no axis %s", mode, a)
		}
		drop[a] = true
	}
	outAxes := make([]string, 0, t.rank())
	outShape := make([]int, 0, t.rank())
	for i, name := range t.Axes {
		if !drop[name] {
			outAxes = append(outAxes, name)
			outShape = append(outShape, t.Shape[i])
		}
	}
	dropIdx := make([]int, 0, len(axes))
	for i, name := range t.Axes {
		if drop[name] {
			dropIdx = append(dropIdx, i)
		}
	}
	return mlReduce(t, dropIdx, mode, outAxes, outShape)
}

// ---------------------------------------------------------------------------
// Einsum: join followed by sum-projection, written as a subscript
// ---------------------------------------------------------------------------

func einsum(spec string, inputs []*Tensor) (*Tensor, error) {
	spec = strings.ReplaceAll(spec, " ", "")
	parts := strings.Split(spec, "->")
	if len(parts) != 2 || parts[0] == "" {
		return nil, fmt.Errorf("tensor-einsum: expected \"inputs->output\", got %q", spec)
	}
	inSpecs := strings.Split(parts[0], ",")
	if len(inSpecs) == 1 && inSpecs[0] == "" {
		inSpecs = nil
	}
	if len(inSpecs) != len(inputs) {
		return nil, fmt.Errorf("tensor-einsum: %q has %d inputs, got %d tensors", spec, len(inSpecs), len(inputs))
	}
	outSpec := parts[1]
	sizes := map[byte]int{}
	seen := map[byte]bool{}
	for i, in := range inSpecs {
		if len(in) != inputs[i].rank() {
			return nil, fmt.Errorf("tensor-einsum: %q has %d indices but the tensor has rank %d", in, len(in), inputs[i].rank())
		}
		for k := 0; k < len(in); k++ {
			c := in[k]
			if c < 'a' || c > 'z' {
				return nil, fmt.Errorf("tensor-einsum: index %q is not a letter", string(c))
			}
			if s, ok := sizes[c]; ok && s != inputs[i].Shape[k] {
				return nil, fmt.Errorf("tensor-einsum: index %q is both %d and %d", string(c), s, inputs[i].Shape[k])
			}
			sizes[c] = inputs[i].Shape[k]
			seen[c] = true
		}
	}
	for i := 0; i < len(outSpec); i++ {
		c := outSpec[i]
		if !seen[c] {
			return nil, fmt.Errorf("tensor-einsum: output index %q is not an input index", string(c))
		}
	}
	outAxes := make([]string, len(outSpec))
	outShape := make([]int, len(outSpec))
	for i := 0; i < len(outSpec); i++ {
		outAxes[i] = string(outSpec[i])
		outShape[i] = sizes[outSpec[i]]
	}
	return mlEinsum(spec, inputs, outAxes, outShape)
}

// ---------------------------------------------------------------------------
// Pointwise ops and the nonlinearities the paper uses
// ---------------------------------------------------------------------------

func sameShape(a, b *Tensor) bool {
	if len(a.Shape) != len(b.Shape) {
		return false
	}
	for i := range a.Shape {
		if a.Shape[i] != b.Shape[i] || a.Axes[i] != b.Axes[i] {
			return false
		}
	}
	return true
}

func pointwise(a, b *Tensor, op func(float64, float64) float64, name string) (*Tensor, error) {
	if a.rank() == 0 {
		a, b = b, a
	}
	if b.rank() != 0 && !sameShape(a, b) {
		return nil, fmt.Errorf("%s: tensors differ in shape or axis names", name)
	}
	_ = op
	axes, shape := a.Axes, a.Shape
	if a.rank() == 0 {
		axes, shape = b.Axes, b.Shape
	}
	return mlBinary(a, b, name, axes, shape)
}

func softmaxAxis(t *Tensor, axis string) (*Tensor, error) {
	ax, ok := t.axisIndex(axis)
	if !ok {
		return nil, fmt.Errorf("tensor-softmax: tensor has no axis %s", axis)
	}
	return mlSoftmax(t, ax)
}

func lnormAxis(t *Tensor, axis string) (*Tensor, error) {
	ax, ok := t.axisIndex(axis)
	if !ok {
		return nil, fmt.Errorf("tensor-lnorm: tensor has no axis %s", axis)
	}
	return mlLnorm(t, ax)
}

func tensorToList(t *Tensor) ArliList {
	out := make(ArliList, len(t.Data))
	for i, v := range t.Data {
		out[i] = ArliFloat(v)
	}
	return out
}

// ---------------------------------------------------------------------------
// Builtins
// ---------------------------------------------------------------------------

func getTensorBuiltins() map[string]*ArliBuiltin {
	b := make(map[string]*ArliBuiltin)
	add := func(name string, arity int, fn func([]ArliValue, *Evaluator) (ArliValue, error)) {
		b[name] = &ArliBuiltin{Name: name, Arity: arity, Fn: fn}
	}

	add("tensor", -1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorFromArgs(args)
	})
	add("tensor?", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(*Tensor)
		return boolResult(ok), nil
	})
	add("tensor-shape", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		t, err := needTensor(args[0], "tensor-shape")
		if err != nil {
			return nil, err
		}
		out := make(ArliList, len(t.Shape))
		for i, d := range t.Shape {
			out[i] = ArliInt(d)
		}
		return out, nil
	})
	add("tensor-axes", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		t, err := needTensor(args[0], "tensor-axes")
		if err != nil {
			return nil, err
		}
		out := make(ArliList, len(t.Axes))
		for i, a := range t.Axes {
			out[i] = ArliSymbol(a)
		}
		return out, nil
	})
	add("tensor->list", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		t, err := needTensor(args[0], "tensor->list")
		if err != nil {
			return nil, err
		}
		return tensorToList(t), nil
	})
	add("tensor-at", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		t, err := needTensor(args[0], "tensor-at")
		if err != nil {
			return nil, err
		}
		idx, err := indexList(args[1], t)
		if err != nil {
			return nil, err
		}
		return ArliFloat(t.Data[t.offset(idx)]), nil
	})
	add("tensor-set!", 3, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		t, err := needTensor(args[0], "tensor-set!")
		if err != nil {
			return nil, err
		}
		idx, err := indexList(args[1], t)
		if err != nil {
			return nil, err
		}
		switch n := args[2].(type) {
		case ArliInt:
			t.Data[t.offset(idx)] = float64(n)
		case ArliFloat:
			t.Data[t.offset(idx)] = float64(n)
		default:
			return nil, fmt.Errorf("tensor-set!: expected a number")
		}
		return t, nil
	})

	add("tensor-add", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorBinary(args, "tensor-add", func(x, y float64) float64 { return x + y })
	})
	add("tensor-sub", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorBinary(args, "tensor-sub", func(x, y float64) float64 { return x - y })
	})
	add("tensor-mul", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorBinary(args, "tensor-mul", func(x, y float64) float64 { return x * y })
	})

	add("tensor-join", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, err := needTensor(args[0], "tensor-join")
		if err != nil {
			return nil, err
		}
		b, err := needTensor(args[1], "tensor-join")
		if err != nil {
			return nil, err
		}
		return joinTensors(a, b)
	})
	add("tensor-project", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorReduce(args, "sum")
	})
	add("tensor-max", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorReduce(args, "max")
	})
	add("tensor-mean", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorReduce(args, "mean")
	})
	add("tensor-einsum", -1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("tensor-einsum: expected a subscript and at least one tensor")
		}
		spec, ok := args[0].(ArliString)
		if !ok {
			return nil, fmt.Errorf("tensor-einsum: first argument must be a subscript string")
		}
		ts := make([]*Tensor, len(args)-1)
		for i, a := range args[1:] {
			t, err := needTensor(a, "tensor-einsum")
			if err != nil {
				return nil, err
			}
			ts[i] = t
		}
		return einsum(string(spec), ts)
	})

	add("tensor-step", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorUnary(args, "tensor-step", "step")
	})
	add("tensor-relu", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorUnary(args, "tensor-relu", "relu")
	})
	add("tensor-sig", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorUnary(args, "tensor-sig", "sig")
	})
	add("tensor-exp", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorUnary(args, "tensor-exp", "exp")
	})
	add("tensor-softmax", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorAlong(args, "tensor-softmax", softmaxAxis)
	})
	add("tensor-lnorm", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return tensorAlong(args, "tensor-lnorm", lnormAxis)
	})
	return b
}

func indexList(v ArliValue, t *Tensor) ([]int, error) {
	lst, ok := v.(ArliList)
	if !ok {
		return nil, fmt.Errorf("expected an index list, got %s", v.ArliRepr())
	}
	if len(lst) != t.rank() {
		return nil, fmt.Errorf("expected %d indices, got %d", t.rank(), len(lst))
	}
	idx := make([]int, len(lst))
	for i, el := range lst {
		n, ok := el.(ArliInt)
		if !ok {
			return nil, fmt.Errorf("index must be an integer")
		}
		if int(n) < 0 || int(n) >= t.Shape[i] {
			return nil, fmt.Errorf("index %d is outside axis %s of size %d", n, t.Axes[i], t.Shape[i])
		}
		idx[i] = int(n)
	}
	return idx, nil
}

func tensorBinary(args []ArliValue, name string, op func(float64, float64) float64) (ArliValue, error) {
	a, aok := args[0].(*Tensor)
	b, bok := args[1].(*Tensor)
	if !aok {
		if n, ok := scalarOf(args[0]); ok && bok {
			a = &Tensor{Data: []float64{n}}
			aok = true
		}
	}
	if !bok {
		if n, ok := scalarOf(args[1]); ok && aok {
			b = &Tensor{Data: []float64{n}}
			bok = true
		}
	}
	if !aok || !bok {
		return nil, fmt.Errorf("%s: expected tensors or a tensor and a number", name)
	}
	return pointwise(a, b, op, name)
}

func scalarOf(v ArliValue) (float64, bool) {
	switch n := v.(type) {
	case ArliInt:
		return float64(n), true
	case ArliFloat:
		return float64(n), true
	default:
		return 0, false
	}
}

func tensorUnary(args []ArliValue, name string, which string) (ArliValue, error) {
	t, err := needTensor(args[0], name)
	if err != nil {
		return nil, err
	}
	return mlMap(t, which)
}

func tensorReduce(args []ArliValue, mode string) (ArliValue, error) {
	t, err := needTensor(args[0], "tensor-"+mode)
	if err != nil {
		return nil, err
	}
	names, err := axisNames(args[1])
	if err != nil {
		return nil, err
	}
	return projectTensor(t, names, mode)
}

func tensorAlong(args []ArliValue, name string, fn func(*Tensor, string) (*Tensor, error)) (ArliValue, error) {
	t, err := needTensor(args[0], name)
	if err != nil {
		return nil, err
	}
	s, ok := args[1].(ArliSymbol)
	if !ok {
		return nil, fmt.Errorf("%s: expected an axis name", name)
	}
	return fn(t, string(s))
}
