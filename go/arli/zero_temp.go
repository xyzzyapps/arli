package main

import (
	"fmt"
	"math"
	"strings"
)

// Zero-temperature reasoning, as in Domingos, "Tensor Logic", section 5.
//
// Facts are Parent(Alice, Bob) and Parent(Bob, Charlie). Forward chaining
// with the step function (the T → 0 limit of the rule sigmoid) derives
// Ancestor(Alice, Charlie) and leaves every other atom at 0. Dana is not in
// a fact, but her embedding sits near Charlie. At T = 0 the Gram matrix is
// sharpened to the identity, so Dana does not inherit Charlie's entailment.
// At high T she does, in proportion to that similarity.

const (
	ztAlice = iota
	ztBob
	ztCharlie
	ztDana
	ztN
)

func ztNames() []string { return []string{"Alice", "Bob", "Charlie", "Dana"} }

func ztParent() *Tensor {
	data := make([]float64, ztN*ztN)
	data[ztAlice*ztN+ztBob] = 1
	data[ztBob*ztN+ztCharlie] = 1
	t, err := newTensor([]string{"x", "y"}, []int{ztN, ztN}, data)
	if err != nil {
		panic(err)
	}
	return t
}

// ancestorClosure applies
//
//	Ancestor(x,z) = step( Parent(x,z) + Ancestor(x,y) Parent(y,z) )
//
// until it stops growing. Step is the Heaviside limit of σ(x/T) at T = 0.
func ancestorClosure(parent *Tensor) (*Tensor, error) {
	cur, err := mlMap(parent, "step")
	if err != nil {
		return nil, err
	}
	py := &Tensor{Axes: []string{"y", "z"}, Shape: append([]int{}, parent.Shape...), Data: append([]float64{}, parent.Data...)}
	for i := 0; i < ztN; i++ {
		joined, err := mlJoin(cur, py, []string{"x", "y", "z"}, []int{ztN, ztN, ztN})
		if err != nil {
			return nil, err
		}
		hop, err := mlReduce(joined, []int{1}, "sum", []string{"x", "z"}, []int{ztN, ztN})
		if err != nil {
			return nil, err
		}
		hop.Axes = []string{"x", "y"}
		hop, err = mlMap(hop, "step")
		if err != nil {
			return nil, err
		}
		sum, err := mlBinary(cur, hop, "tensor-add", []string{"x", "y"}, []int{ztN, ztN})
		if err != nil {
			return nil, err
		}
		next, err := mlMap(sum, "step")
		if err != nil {
			return nil, err
		}
		if sameData(cur, next) {
			return next, nil
		}
		cur = next
	}
	return cur, nil
}

func sameData(a, b *Tensor) bool {
	if len(a.Data) != len(b.Data) {
		return false
	}
	for i := range a.Data {
		if a.Data[i] != b.Data[i] {
			return false
		}
	}
	return true
}

// ztEmbed is the identity on Alice, Bob and Charlie. Dana is a unit vector
// whose cosine with Charlie is 0.95 and whose self-similarity is 1, so the
// T → 0 softmax of the Gram matrix still selects Dana herself.
func ztEmbed() [][]float64 {
	e := make([][]float64, ztN)
	for i := 0; i < ztN; i++ {
		e[i] = make([]float64, ztN)
		e[i][i] = 1
	}
	const cos = 0.95
	orth := math.Sqrt(1 - cos*cos)
	e[ztDana] = []float64{0, 0, cos, orth}
	return e
}

func ztGram(emb [][]float64) [][]float64 {
	g := make([][]float64, ztN)
	for i := 0; i < ztN; i++ {
		g[i] = make([]float64, ztN)
		for j := 0; j < ztN; j++ {
			s := 0.0
			for d := 0; d < ztN; d++ {
				s += emb[i][d] * emb[j][d]
			}
			g[i][j] = s
		}
	}
	return g
}

// ztSharpen is softmax(G / T). At T = 0 it is the identity: each object
// matches only itself, which is what the paper means by the Gram matrix
// collapsing. Any T > 0 lets a neighbour borrow mass.
func ztSharpen(g [][]float64, temp float64) [][]float64 {
	w := make([][]float64, ztN)
	for i := 0; i < ztN; i++ {
		w[i] = make([]float64, ztN)
		if temp == 0 {
			w[i][i] = 1
			continue
		}
		max := g[i][0]
		for j := 1; j < ztN; j++ {
			if g[i][j] > max {
				max = g[i][j]
			}
		}
		sum := 0.0
		for j := 0; j < ztN; j++ {
			w[i][j] = math.Exp((g[i][j] - max) / temp)
			sum += w[i][j]
		}
		for j := 0; j < ztN; j++ {
			w[i][j] /= sum
		}
	}
	return w
}

// ztBorrow reads the deductive closure through a softened identity.
// score(a,b) = Σ_{x,y} W(a,x) Ancestor(x,y) W(b,y).
func ztBorrow(anc []float64, w [][]float64, a, b int) float64 {
	s := 0.0
	for x := 0; x < ztN; x++ {
		for y := 0; y < ztN; y++ {
			s += w[a][x] * anc[x*ztN+y] * w[b][y]
		}
	}
	return s
}

func fmtAtom(v float64) string {
	if v == 0 || v == 1 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.3f", v)
}

func zeroTempReport() (string, error) {
	parent := ztParent()
	anc, err := ancestorClosure(parent)
	if err != nil {
		return "", err
	}
	at := func(t *Tensor, i, j int) float64 { return t.Data[i*ztN+j] }
	g := ztGram(ztEmbed())
	var b strings.Builder
	fmt.Fprintf(&b, "Facts: Parent(Alice, Bob), Parent(Bob, Charlie).\n")
	fmt.Fprintf(&b, "Dana is in no fact. Her embedding has cosine %.2f with Charlie.\n", g[ztDana][ztCharlie])
	fmt.Fprintf(&b, "\nT = 0   step function, deductive closure\n")
	fmt.Fprintf(&b, "  Ancestor(Alice, Bob)      %s   stored parent, so ancestor\n", fmtAtom(at(anc, ztAlice, ztBob)))
	fmt.Fprintf(&b, "  Ancestor(Alice, Charlie)  %s   entailed by the rule, not stored\n", fmtAtom(at(anc, ztAlice, ztCharlie)))
	fmt.Fprintf(&b, "  Parent(Alice, Charlie)    %s   not a fact, and the rule does not add one\n", fmtAtom(at(parent, ztAlice, ztCharlie)))
	fmt.Fprintf(&b, "  Ancestor(Alice, Dana)     %s   nearest neighbour of Charlie, still false\n", fmtAtom(at(anc, ztAlice, ztDana)))
	fmt.Fprintf(&b, "\nSame closure, read through softmax(Gram / T)\n")
	for _, temp := range []float64{1, 0.01, 0} {
		label := fmt.Sprintf("T = %g", temp)
		if temp == 0 {
			label = "T = 0"
		}
		w := ztSharpen(g, temp)
		dana := ztBorrow(anc.Data, w, ztAlice, ztDana)
		charlie := ztBorrow(anc.Data, w, ztAlice, ztCharlie)
		fmt.Fprintf(&b, "  %-6s  Ancestor(Alice, Charlie) %s    Ancestor(Alice, Dana) %s\n",
			label, fmtAtom(charlie), fmtAtom(dana))
	}
	fmt.Fprintf(&b, "At T = 1 Dana and Charlie look alike. At T = 0 only Charlie remains.\n")
	return b.String(), nil
}

