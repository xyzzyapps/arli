package main

import (
	"fmt"
	"sort"

	"github.com/gomlx/compute/dtypes"
	"github.com/gomlx/compute/shapes"
	"github.com/gomlx/gomlx/core/graph"
	"github.com/gomlx/gomlx/core/tensors"
)

// An equation store is a tensor-logic program: a set of tensor equations
// executed by forward chaining. Equations that share a left-hand side and a
// nonlinearity are summed, then the nonlinearity runs once. sig at temperature
// 0 is the step function. tensor-embed-rel is the section-5 relation embedding.

type factor struct {
	name string
	idx  []string
}

type equation struct {
	lhs    string
	idx    []string
	nonlin string
	temp   float64
	terms  [][]factor
}

type lossSpec struct {
	kind   string // "bce", "mse", "sum", or ""
	pred   string
	target string
}

type eqStore struct {
	cur   map[string]*Tensor
	init  map[string]*Tensor
	learn map[string]bool
	eqs   []equation
	loss  lossSpec
	chain int
}

func newEqStore() *eqStore {
	return &eqStore{
		cur:   map[string]*Tensor{},
		init:  map[string]*Tensor{},
		learn: map[string]bool{},
		chain: 1,
	}
}

func (ev *Evaluator) eqs() *eqStore {
	if ev.eqEngine == nil {
		ev.eqEngine = newEqStore()
	}
	return ev.eqEngine
}

func (st *eqStore) define(name string, t *Tensor) {
	c := t.clone()
	st.cur[name] = c
	st.init[name] = t.clone()
}

func (st *eqStore) grouped() []equation {
	var out []equation
	for _, eq := range st.eqs {
		found := -1
		for i := range out {
			if out[i].lhs == eq.lhs && sameIdx(out[i].idx, eq.idx) && out[i].nonlin == eq.nonlin && out[i].temp == eq.temp {
				found = i
				break
			}
		}
		if found < 0 {
			cp := eq
			cp.terms = append([][]factor{}, eq.terms...)
			out = append(out, cp)
			continue
		}
		out[found].terms = append(out[found].terms, eq.terms...)
	}
	return out
}

func sameIdx(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// indexSizes binds every index name to a dimension using tensors that already
// have values. Left-hand sides that are only computed get their shape from
// those bindings.
func (st *eqStore) indexSizes() (map[string]int, error) {
	sizes := map[string]int{}
	known := map[string]bool{}
	for name, t := range st.cur {
		known[name] = true
		_ = t
	}
	progress := true
	for progress {
		progress = false
		for _, eq := range st.eqs {
			for _, term := range eq.terms {
				for _, f := range term {
					t, ok := st.cur[f.name]
					if !ok {
						continue
					}
					if t.rank() != len(f.idx) {
						return nil, fmt.Errorf("tensor-eq: %s has rank %d but is used with indices %v", f.name, t.rank(), f.idx)
					}
					for i, id := range f.idx {
						if sz, ok := sizes[id]; ok && sz != t.Shape[i] {
							return nil, fmt.Errorf("tensor-eq: index %s is both %d and %d", id, sz, t.Shape[i])
						}
						if _, ok := sizes[id]; !ok {
							sizes[id] = t.Shape[i]
							progress = true
						}
					}
				}
			}
			if _, ok := known[eq.lhs]; ok {
				continue
			}
			ready := true
			shape := make([]int, len(eq.idx))
			for i, id := range eq.idx {
				sz, ok := sizes[id]
				if !ok {
					ready = false
					break
				}
				shape[i] = sz
			}
			if !ready {
				continue
			}
			z := make([]float64, numel(shape))
			st.cur[eq.lhs] = &Tensor{Axes: append([]string{}, eq.idx...), Shape: shape, Data: z}
			known[eq.lhs] = true
			progress = true
		}
	}
	for _, eq := range st.eqs {
		for _, term := range eq.terms {
			for _, f := range term {
				if _, ok := st.cur[f.name]; !ok {
					return nil, fmt.Errorf("tensor-eq: %s has no value and is not computed by an equation", f.name)
				}
			}
		}
	}
	return sizes, nil
}

func (st *eqStore) run(steps int) (int, error) {
	if steps < 1 {
		return 0, fmt.Errorf("tensor-run: steps must be positive")
	}
	st.chain = steps
	if _, err := st.indexSizes(); err != nil {
		return 0, err
	}
	ran := 0
	for i := 0; i < steps; i++ {
		next, err := st.oneRound()
		if err != nil {
			return ran, err
		}
		ran++
		stable := true
		for name, t := range next {
			if !sameData(st.cur[name], t) {
				stable = false
			}
			st.cur[name] = t
		}
		if stable {
			break
		}
	}
	return ran, nil
}

func (st *eqStore) oneRound() (map[string]*Tensor, error) {
	names := st.inputNames()
	ins := make([]*Tensor, len(names))
	for i, n := range names {
		ins[i] = st.cur[n]
	}
	lhs := st.outputNames()
	outs, err := eqCall(func(in []*graph.Node) []*graph.Node {
		nodes := map[string]*graph.Node{}
		for i, n := range names {
			nodes[n] = in[i]
		}
		updated, loss, err := st.apply(nodes, 1)
		if err != nil {
			panic(err)
		}
		var result []*graph.Node
		for _, n := range lhs {
			result = append(result, updated[n])
		}
		if loss != nil {
			result = append(result, loss)
		}
		return result
	}, ins)
	if err != nil {
		return nil, err
	}
	got := map[string]*Tensor{}
	for i, n := range lhs {
		eq := st.eqByLHS(n)
		got[n] = &Tensor{Axes: append([]string{}, eq.idx...), Shape: st.lhsShape(n), Data: mustFlat(outs[i])}
		outs[i].FinalizeAll()
	}
	if st.loss.kind != "" {
		li := len(lhs)
		flat := mustFlat(outs[li])
		outs[li].FinalizeAll()
		got["Loss"] = &Tensor{Data: []float64{flat[0]}}
	}
	return got, nil
}

func (st *eqStore) eqByLHS(name string) equation {
	for _, eq := range st.grouped() {
		if eq.lhs == name {
			return eq
		}
	}
	return equation{lhs: name}
}

func (st *eqStore) lhsShape(name string) []int {
	t := st.cur[name]
	if t == nil {
		return nil
	}
	return append([]int{}, t.Shape...)
}

func (st *eqStore) inputNames() []string {
	seen := map[string]bool{}
	var names []string
	add := func(n string) {
		if n == "" || seen[n] || st.cur[n] == nil {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	for _, eq := range st.eqs {
		for _, term := range eq.terms {
			for _, f := range term {
				add(f.name)
			}
		}
	}
	if st.loss.target != "" {
		add(st.loss.target)
	}
	if st.loss.pred != "" && st.cur[st.loss.pred] != nil && !st.isLHS(st.loss.pred) {
		add(st.loss.pred)
	}
	sort.Strings(names)
	return names
}

func (st *eqStore) isLHS(name string) bool {
	for _, eq := range st.eqs {
		if eq.lhs == name {
			return true
		}
	}
	return false
}

func (st *eqStore) outputNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, eq := range st.grouped() {
		if seen[eq.lhs] {
			continue
		}
		seen[eq.lhs] = true
		names = append(names, eq.lhs)
	}
	sort.Strings(names)
	return names
}

// apply evaluates every equation for `chain` Jacobi rounds. nodes holds the
// starting value of every named tensor. Computed names are overwritten.
func (st *eqStore) apply(nodes map[string]*graph.Node, chain int) (map[string]*graph.Node, *graph.Node, error) {
	if len(nodes) == 0 && len(st.eqs) == 0 {
		return nil, nil, fmt.Errorf("tensor-eq: no equations")
	}
	var g *graph.Graph
	for _, n := range nodes {
		g = n.Graph()
		break
	}
	if g == nil {
		return nil, nil, fmt.Errorf("tensor-eq: empty program")
	}
	sizes, err := st.boundSizes()
	if err != nil {
		return nil, nil, err
	}
	for _, eq := range st.grouped() {
		if _, ok := nodes[eq.lhs]; ok {
			continue
		}
		nodes[eq.lhs] = graph.Zeros(g, shapes.Make(dtypes.Float64, st.lhsShape(eq.lhs)...))
	}
	for round := 0; round < chain; round++ {
		snap := map[string]*graph.Node{}
		for k, v := range nodes {
			snap[k] = v
		}
		updated := map[string]*graph.Node{}
		for _, eq := range st.grouped() {
			v, err := st.evalEq(snap, eq, sizes)
			if err != nil {
				return nil, nil, err
			}
			if prev, ok := updated[eq.lhs]; ok {
				v = graph.Add(prev, v)
			}
			updated[eq.lhs] = v
		}
		for k, v := range updated {
			nodes[k] = v
		}
	}
	loss, err := st.evalLoss(nodes)
	if err != nil {
		return nil, nil, err
	}
	return nodes, loss, nil
}

func (st *eqStore) boundSizes() (map[string]int, error) {
	sizes := map[string]int{}
	for _, eq := range st.eqs {
		for _, term := range eq.terms {
			for _, f := range term {
				t := st.cur[f.name]
				if t == nil {
					continue
				}
				if t.rank() != len(f.idx) {
					return nil, fmt.Errorf("tensor-eq: %s has rank %d but is used with %d indices", f.name, t.rank(), len(f.idx))
				}
				for i, id := range f.idx {
					if sz, ok := sizes[id]; ok && sz != t.Shape[i] {
						return nil, fmt.Errorf("tensor-eq: index %s is both %d and %d", id, sz, t.Shape[i])
					}
					sizes[id] = t.Shape[i]
				}
			}
		}
	}
	return sizes, nil
}

func (st *eqStore) evalEq(nodes map[string]*graph.Node, eq equation, sizes map[string]int) (*graph.Node, error) {
	var acc *graph.Node
	for _, term := range eq.terms {
		v, err := st.evalTerm(nodes, term, eq.idx, sizes)
		if err != nil {
			return nil, err
		}
		if acc == nil {
			acc = v
			continue
		}
		acc = graph.Add(acc, v)
	}
	if acc == nil {
		return nil, fmt.Errorf("tensor-eq: %s has no terms", eq.lhs)
	}
	return applyNonlin(acc, eq.nonlin, eq.temp), nil
}

func (st *eqStore) evalTerm(nodes map[string]*graph.Node, term []factor, lhs []string, sizes map[string]int) (*graph.Node, error) {
	if len(term) == 0 {
		return nil, fmt.Errorf("tensor-eq: empty term")
	}
	var acc *graph.Node
	var axes []string
	for _, f := range term {
		n, ok := nodes[f.name]
		if !ok {
			return nil, fmt.Errorf("tensor-eq: %s is not defined", f.name)
		}
		if n.Rank() != len(f.idx) {
			return nil, fmt.Errorf("tensor-eq: %s has rank %d but is used with indices %v", f.name, n.Rank(), f.idx)
		}
		if acc == nil {
			acc = n
			axes = append([]string{}, f.idx...)
			continue
		}
		var err error
		acc, axes, err = graphJoin(acc, axes, n, f.idx)
		if err != nil {
			return nil, err
		}
	}
	keep := map[string]bool{}
	for _, id := range lhs {
		keep[id] = true
	}
	var drop []int
	var kept []string
	for i, id := range axes {
		if keep[id] {
			kept = append(kept, id)
			continue
		}
		drop = append(drop, i)
	}
	if len(drop) > 0 {
		acc = graph.ReduceSum(acc, drop...)
	}
	return alignToLHS(acc, kept, lhs, sizes)
}

func graphJoin(a *graph.Node, aAxes []string, b *graph.Node, bAxes []string) (*graph.Node, []string, error) {
	letters := map[string]byte{}
	n := byte(0)
	assign := func(ids []string) (string, error) {
		s := ""
		for _, id := range ids {
			c, ok := letters[id]
			if !ok {
				if n >= 26 {
					return "", fmt.Errorf("tensor-eq: more than 26 indices")
				}
				c = 'a' + n
				n++
				letters[id] = c
			}
			s += string(c)
		}
		return s, nil
	}
	as, err := assign(aAxes)
	if err != nil {
		return nil, nil, err
	}
	bs, err := assign(bAxes)
	if err != nil {
		return nil, nil, err
	}
	var outAxes []string
	seen := map[string]bool{}
	for _, id := range aAxes {
		if !seen[id] {
			seen[id] = true
			outAxes = append(outAxes, id)
		}
	}
	for _, id := range bAxes {
		if !seen[id] {
			seen[id] = true
			outAxes = append(outAxes, id)
		}
	}
	os, err := assign(outAxes)
	if err != nil {
		return nil, nil, err
	}
	// assign() on outAxes finds existing letters; but n may have been used.
	// Recompute os from letters directly.
	os = ""
	for _, id := range outAxes {
		os += string(letters[id])
	}
	return graph.Einsum(as+","+bs+"->"+os, a, b), outAxes, nil
}

func alignToLHS(n *graph.Node, axes, lhs []string, sizes map[string]int) (*graph.Node, error) {
	if len(lhs) == 0 {
		if n.Rank() != 0 {
			n = graph.ReduceAllSum(n)
		}
		return n, nil
	}
	pos := map[string]int{}
	for i, id := range lhs {
		pos[id] = i
	}
	for _, id := range axes {
		if _, ok := pos[id]; !ok {
			return nil, fmt.Errorf("tensor-eq: index %s is not on the left-hand side", id)
		}
	}
	perm := make([]int, len(axes))
	for i := range axes {
		perm[i] = i
	}
	sort.SliceStable(perm, func(i, j int) bool { return pos[axes[perm[i]]] < pos[axes[perm[j]]] })
	ident := true
	for i, p := range perm {
		if p != i {
			ident = false
			break
		}
	}
	if !ident {
		n = graph.TransposeAllDims(n, perm...)
	}
	ordered := make([]string, len(axes))
	for i, p := range perm {
		ordered[i] = axes[p]
	}
	var insert []int
	oi := 0
	for i, id := range lhs {
		if oi < len(ordered) && ordered[oi] == id {
			oi++
			continue
		}
		insert = append(insert, i)
	}
	if len(insert) > 0 {
		n = graph.ExpandAxes(n, insert...)
	}
	target := make([]int, len(lhs))
	for i, id := range lhs {
		sz, ok := sizes[id]
		if !ok {
			return nil, fmt.Errorf("tensor-eq: index %s has no size", id)
		}
		target[i] = sz
	}
	return graph.BroadcastToShape(n, shapes.Make(n.DType(), target...)), nil
}

func applyNonlin(n *graph.Node, kind string, temp float64) *graph.Node {
	switch kind {
	case "id", "":
		return n
	case "relu":
		return graph.Max(n, graph.ZerosLike(n))
	case "sig":
		if temp > 0 {
			return graph.Sigmoid(graph.DivScalar(n, temp))
		}
		fallthrough
	default:
		pos := graph.GreaterThan(n, graph.ScalarZero(n.Graph(), n.DType()))
		return graph.Where(pos, graph.OnesLike(n), graph.ZerosLike(n))
	}
}

func (st *eqStore) evalLoss(nodes map[string]*graph.Node) (*graph.Node, error) {
	if st.loss.kind == "" {
		return nil, nil
	}
	pred, ok := nodes[st.loss.pred]
	if !ok {
		return nil, fmt.Errorf("tensor-loss: %s is not defined", st.loss.pred)
	}
	switch st.loss.kind {
	case "sum":
		if pred.Rank() == 0 {
			return pred, nil
		}
		return graph.ReduceAllSum(pred), nil
	case "mse":
		target, ok := nodes[st.loss.target]
		if !ok {
			return nil, fmt.Errorf("tensor-loss: %s is not defined", st.loss.target)
		}
		d := graph.Sub(pred, target)
		return graph.ReduceAllMean(graph.Mul(d, d)), nil
	case "bce":
		target, ok := nodes[st.loss.target]
		if !ok {
			return nil, fmt.Errorf("tensor-loss: %s is not defined", st.loss.target)
		}
		return stableBCE(pred, target), nil
	default:
		return nil, fmt.Errorf("tensor-loss: unknown kind %s", st.loss.kind)
	}
}

func (st *eqStore) fit(steps int, lr float64) (float64, error) {
	if steps < 1 {
		return 0, fmt.Errorf("tensor-fit: steps must be positive")
	}
	if st.loss.kind == "" {
		return 0, fmt.Errorf("tensor-fit: set a loss with tensor-loss")
	}
	if st.chain < 1 {
		st.chain = 1
	}
	if _, err := st.indexSizes(); err != nil {
		return 0, err
	}
	var learn []string
	for name := range st.learn {
		if st.isLHS(name) {
			return 0, fmt.Errorf("tensor-fit: %s is computed by an equation and also marked learnable", name)
		}
		if st.cur[name] == nil {
			return 0, fmt.Errorf("tensor-fit: %s has no value", name)
		}
		learn = append(learn, name)
	}
	sort.Strings(learn)
	if len(learn) == 0 {
		return 0, fmt.Errorf("tensor-fit: nothing is marked with tensor-learn")
	}
	chain := st.chain
	b, err := backend()
	if err != nil {
		return 0, err
	}
	exec := graph.MustNewExec(b, func(in []*graph.Node) []*graph.Node {
		nodes := map[string]*graph.Node{}
		g := in[0].Graph()
		for i, name := range learn {
			nodes[name] = in[i]
		}
		for name, t := range st.cur {
			if _, ok := nodes[name]; ok || st.isLHS(name) {
				continue
			}
			nodes[name] = graph.ConstTensor(g, upload(t))
		}
		for _, eq := range st.grouped() {
			if _, ok := nodes[eq.lhs]; ok {
				continue
			}
			start := st.init[eq.lhs]
			if start == nil {
				nodes[eq.lhs] = graph.Zeros(g, shapes.Make(dtypes.Float64, st.lhsShape(eq.lhs)...))
				continue
			}
			nodes[eq.lhs] = graph.ConstTensor(g, upload(start))
		}
		updated, loss, err := st.apply(nodes, chain)
		if err != nil {
			panic(err)
		}
		if loss.Rank() > 0 {
			loss = graph.ReduceAllSum(loss)
		}
		grads := graph.Gradient(loss, paramsOf(updated, learn)...)
		out := []*graph.Node{loss}
		for i, name := range learn {
			out = append(out, graph.Sub(updated[name], graph.MulScalar(grads[i], lr)))
		}
		return out
	})
	defer exec.Finalize()

	var loss float64
	for i := 0; i < steps; i++ {
		args := make([]any, len(learn))
		held := make([]*tensors.Tensor, len(learn))
		for j, name := range learn {
			held[j] = upload(st.cur[name])
			args[j] = held[j]
		}
		var outs []*tensors.Tensor
		func() {
			defer func() {
				if r := recover(); r != nil && err == nil {
					err = fmt.Errorf("gomlx: %v", r)
				}
			}()
			outs, err = exec.Call(args...)
		}()
		for _, h := range held {
			h.FinalizeAll()
		}
		if err != nil {
			return 0, err
		}
		loss, err = scalarOfTensor(outs[0])
		if err != nil {
			return 0, err
		}
		outs[0].FinalizeAll()
		for j, name := range learn {
			flat, ferr := flatOfTensor(outs[j+1])
			outs[j+1].FinalizeAll()
			if ferr != nil {
				return 0, ferr
			}
			prev := st.cur[name]
			st.cur[name] = &Tensor{Axes: append([]string{}, prev.Axes...), Shape: append([]int{}, prev.Shape...), Data: flat}
		}
	}
	return loss, nil
}

func paramsOf(nodes map[string]*graph.Node, names []string) []*graph.Node {
	out := make([]*graph.Node, len(names))
	for i, n := range names {
		out[i] = nodes[n]
	}
	return out
}

func eqCall(fn func([]*graph.Node) []*graph.Node, inputs []*Tensor) ([]*tensors.Tensor, error) {
	b, err := backend()
	if err != nil {
		return nil, err
	}
	var outs []*tensors.Tensor
	func() {
		defer func() {
			if r := recover(); r != nil && err == nil {
				err = fmt.Errorf("gomlx: %v", r)
			}
		}()
		exec := graph.MustNewExec(b, fn)
		defer exec.Finalize()
		args := make([]any, len(inputs))
		held := make([]*tensors.Tensor, len(inputs))
		for i, t := range inputs {
			held[i] = upload(t)
			args[i] = held[i]
		}
		outs, err = exec.Call(args...)
		for _, h := range held {
			h.FinalizeAll()
		}
	}()
	return outs, err
}

func mustFlat(t *tensors.Tensor) []float64 {
	flat, err := flatOfTensor(t)
	if err != nil {
		panic(err)
	}
	return flat
}

// ---------------------------------------------------------------------------
// Relation embedding (paper section 5)
// EmbR[i,j] = R[x,y] Emb[x,i] Emb[y,j]
// D[a,b]    = EmbR[i,j] Emb[a,i] Emb[b,j]
// ---------------------------------------------------------------------------

func embedRel(r, emb *Tensor) (*Tensor, error) {
	if r.rank() != 2 || emb.rank() != 2 {
		return nil, fmt.Errorf("tensor-embed-rel: relation and embeddings are matrices")
	}
	if r.Shape[0] != emb.Shape[0] || r.Shape[1] != emb.Shape[0] {
		return nil, fmt.Errorf("tensor-embed-rel: relation is %v and embeddings are %v", r.Shape, emb.Shape)
	}
	d := emb.Shape[1]
	return evalGraph([]string{"i", "j"}, []int{d, d}, func(in []*graph.Node) *graph.Node {
		tmp := graph.Einsum("xy,xi->yi", in[0], in[1])
		return graph.Einsum("yi,yj->ij", tmp, in[1])
	}, r, emb)
}

func embedGet(embR, emb *Tensor) (*Tensor, error) {
	if embR.rank() != 2 || emb.rank() != 2 {
		return nil, fmt.Errorf("tensor-embed-get: both arguments are matrices")
	}
	if embR.Shape[0] != emb.Shape[1] || embR.Shape[1] != emb.Shape[1] {
		return nil, fmt.Errorf("tensor-embed-get: embedding is %v and relation embedding is %v", emb.Shape, embR.Shape)
	}
	n := emb.Shape[0]
	return evalGraph([]string{"a", "b"}, []int{n, n}, func(in []*graph.Node) *graph.Node {
		tmp := graph.Einsum("ij,ai->aj", in[0], in[1])
		return graph.Einsum("aj,bj->ab", tmp, in[1])
	}, embR, emb)
}

// ---------------------------------------------------------------------------
// Words
// ---------------------------------------------------------------------------

func getEqBuiltins() map[string]*ArliBuiltin {
	b := make(map[string]*ArliBuiltin)
	add := func(name string, arity int, fn func([]ArliValue, *Evaluator) (ArliValue, error)) {
		b[name] = &ArliBuiltin{Name: name, Arity: arity, Fn: fn}
	}
	add("tensor-def", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		name, ok := args[0].(ArliSymbol)
		if !ok {
			return nil, fmt.Errorf("tensor-def: name must be a symbol")
		}
		t, err := needTensor(args[1], "tensor-def")
		if err != nil {
			return nil, err
		}
		ev.eqs().define(string(name), t)
		return t, nil
	})
	add("tensor-learn", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		name, ok := args[0].(ArliSymbol)
		if !ok {
			return nil, fmt.Errorf("tensor-learn: name must be a symbol")
		}
		if ev.eqs().cur[string(name)] == nil {
			return nil, fmt.Errorf("tensor-learn: %s is not defined", name)
		}
		ev.eqs().learn[string(name)] = true
		return args[0], nil
	})
	add("tensor-chain", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		n, ok := args[0].(ArliInt)
		if !ok || n < 1 {
			return nil, fmt.Errorf("tensor-chain: expected a positive integer")
		}
		ev.eqs().chain = int(n)
		return n, nil
	})
	add("tensor-eq", -1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		eq, err := parseEq(args)
		if err != nil {
			return nil, err
		}
		ev.eqs().eqs = append(ev.eqs().eqs, eq)
		return ArliSymbol(eq.lhs), nil
	})
	add("tensor-loss", -1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		spec, err := parseLoss(args)
		if err != nil {
			return nil, err
		}
		ev.eqs().loss = spec
		return ArliSymbol("Loss"), nil
	})
	add("tensor-run", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		n, ok := args[0].(ArliInt)
		if !ok {
			return nil, fmt.Errorf("tensor-run: expected a step count")
		}
		ran, err := ev.eqs().run(int(n))
		if err != nil {
			return nil, err
		}
		return ArliInt(ran), nil
	})
	add("tensor-get", 1, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		name, ok := args[0].(ArliSymbol)
		if !ok {
			return nil, fmt.Errorf("tensor-get: name must be a symbol")
		}
		t := ev.eqs().cur[string(name)]
		if t == nil {
			return nil, fmt.Errorf("tensor-get: %s is not defined", name)
		}
		return t.clone(), nil
	})
	add("tensor-fit", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		steps, ok := args[0].(ArliInt)
		if !ok {
			return nil, fmt.Errorf("tensor-fit: steps must be an integer")
		}
		lr, ok := scalarOf(args[1])
		if !ok {
			return nil, fmt.Errorf("tensor-fit: learning rate must be a number")
		}
		loss, err := ev.eqs().fit(int(steps), lr)
		if err != nil {
			return nil, err
		}
		return ArliFloat(loss), nil
	})
	add("tensor-embed-rel", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		r, err := needTensor(args[0], "tensor-embed-rel")
		if err != nil {
			return nil, err
		}
		e, err := needTensor(args[1], "tensor-embed-rel")
		if err != nil {
			return nil, err
		}
		return embedRel(r, e)
	})
	add("tensor-embed-get", 2, func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		r, err := needTensor(args[0], "tensor-embed-get")
		if err != nil {
			return nil, err
		}
		e, err := needTensor(args[1], "tensor-embed-get")
		if err != nil {
			return nil, err
		}
		return embedGet(r, e)
	})
	return b
}

func parseEq(args []ArliValue) (equation, error) {
	if len(args) < 4 {
		return equation{}, fmt.Errorf("tensor-eq: expected a name, indices, a nonlinearity, and at least one term")
	}
	name, ok := args[0].(ArliSymbol)
	if !ok {
		return equation{}, fmt.Errorf("tensor-eq: name must be a symbol")
	}
	idx, err := symbolNames(args[1])
	if err != nil {
		return equation{}, fmt.Errorf("tensor-eq: %w", err)
	}
	nonlin, temp, err := parseNonlin(args[2])
	if err != nil {
		return equation{}, err
	}
	var terms [][]factor
	for _, a := range args[3:] {
		term, err := parseTerm(a)
		if err != nil {
			return equation{}, err
		}
		terms = append(terms, term)
	}
	return equation{lhs: string(name), idx: idx, nonlin: nonlin, temp: temp, terms: terms}, nil
}

func parseNonlin(v ArliValue) (string, float64, error) {
	if s, ok := v.(ArliSymbol); ok {
		switch string(s) {
		case "step", "id", "relu":
			return string(s), 0, nil
		case "sig":
			return "sig", 1, nil
		default:
			return "", 0, fmt.Errorf("tensor-eq: unknown nonlinearity %s", s)
		}
	}
	lst, ok := v.(ArliList)
	if !ok || len(lst) != 2 {
		return "", 0, fmt.Errorf("tensor-eq: nonlinearity must be step, id, relu, sig, or (sig temperature)")
	}
	s, ok := lst[0].(ArliSymbol)
	if !ok || string(s) != "sig" {
		return "", 0, fmt.Errorf("tensor-eq: nonlinearity list must be (sig temperature)")
	}
	temp, ok := scalarOf(lst[1])
	if !ok || temp < 0 {
		return "", 0, fmt.Errorf("tensor-eq: temperature must be a non-negative number")
	}
	return "sig", temp, nil
}

func parseTerm(v ArliValue) ([]factor, error) {
	lst, ok := v.(ArliList)
	if !ok || len(lst) == 0 {
		return nil, fmt.Errorf("tensor-eq: a term must be a list of factors")
	}
	if _, ok := lst[0].(ArliSymbol); ok {
		f, err := parseFactor(lst)
		if err != nil {
			return nil, err
		}
		return []factor{f}, nil
	}
	var term []factor
	for _, el := range lst {
		elList, ok := el.(ArliList)
		if !ok {
			return nil, fmt.Errorf("tensor-eq: factor must be a list")
		}
		f, err := parseFactor(elList)
		if err != nil {
			return nil, err
		}
		term = append(term, f)
	}
	return term, nil
}

func parseFactor(lst ArliList) (factor, error) {
	if len(lst) == 0 {
		return factor{}, fmt.Errorf("tensor-eq: empty factor")
	}
	name, ok := lst[0].(ArliSymbol)
	if !ok {
		return factor{}, fmt.Errorf("tensor-eq: factor name must be a symbol")
	}
	var idx []string
	for _, el := range lst[1:] {
		s, ok := el.(ArliSymbol)
		if !ok {
			return factor{}, fmt.Errorf("tensor-eq: index must be a symbol")
		}
		idx = append(idx, string(s))
	}
	return factor{name: string(name), idx: idx}, nil
}

func symbolNames(v ArliValue) ([]string, error) {
	lst, ok := v.(ArliList)
	if !ok {
		return nil, fmt.Errorf("indices must be a list of symbols")
	}
	out := make([]string, len(lst))
	for i, el := range lst {
		s, ok := el.(ArliSymbol)
		if !ok {
			return nil, fmt.Errorf("indices must be symbols")
		}
		out[i] = string(s)
	}
	return out, nil
}

func parseLoss(args []ArliValue) (lossSpec, error) {
	if len(args) < 2 {
		return lossSpec{}, fmt.Errorf("tensor-loss: expected a kind and a tensor name")
	}
	kind, ok := args[0].(ArliSymbol)
	if !ok {
		return lossSpec{}, fmt.Errorf("tensor-loss: kind must be mse, bce, or sum")
	}
	pred, ok := args[1].(ArliSymbol)
	if !ok {
		return lossSpec{}, fmt.Errorf("tensor-loss: tensor name must be a symbol")
	}
	spec := lossSpec{kind: string(kind), pred: string(pred)}
	switch spec.kind {
	case "sum":
		return spec, nil
	case "mse", "bce":
		if len(args) != 3 {
			return lossSpec{}, fmt.Errorf("tensor-loss: %s needs a prediction and a target", spec.kind)
		}
		target, ok := args[2].(ArliSymbol)
		if !ok {
			return lossSpec{}, fmt.Errorf("tensor-loss: target must be a symbol")
		}
		spec.target = string(target)
		return spec, nil
	default:
		return lossSpec{}, fmt.Errorf("tensor-loss: unknown kind %s", spec.kind)
	}
}
