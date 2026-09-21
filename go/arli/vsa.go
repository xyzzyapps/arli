package main

import (
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// VSA (Vector Symbolic Architecture) subsystem — self-contained FHRR engine.
//
// Ported from lvsp (VSA-Lisp / Velo) as an additive arli feature: no existing
// value, builtin, arity entry or repr changes. VSA values are reachable only
// through the `vsa-*` primitives, and each Evaluator owns one lazily created
// engine (see Evaluator.vsa in eval.go).
// ---------------------------------------------------------------------------

const (
	vsaDefaultDim        = 4096
	vsaMinDim            = 4
	vsaMaxDim            = 65536
	vsaCleanupThreshold  = 0.3
	vsaFpePeriod         = 1000000.0
	vsaDefaultStreamSeed = 1

	vsaRoleSeedCar  uint32 = 0x00C0FFEE
	vsaRoleSeedCdr  uint32 = 0x00CDCDCD
	vsaSeedNil      uint32 = 0x00000A11
	vsaSeedTrue     uint32 = 0x00000072
	vsaSeedFalse    uint32 = 0x000000F0
	vsaSeedFpeBasis uint32 = 0x00F9E001
)

// ---------------------------------------------------------------------------
// Value types
// ---------------------------------------------------------------------------

// ArliVSAVec is an FHRR vector: D float32 pairs (re, im) on the unit circle.
type ArliVSAVec struct {
	re []float32
	im []float32
}

func (v *ArliVSAVec) ArliRepr() string { return "<vsa-vec>" }

// ArliVSAPair is a VSA cons cell. vec lazily caches the encoding of the cell.
type ArliVSAPair struct {
	car ArliValue
	cdr ArliValue
	vec *ArliVSAVec
}

func (p *ArliVSAPair) ArliRepr() string {
	var parts []string
	var node ArliValue = p
	for {
		pair, ok := node.(*ArliVSAPair)
		if !ok {
			break
		}
		parts = append(parts, pair.car.ArliRepr())
		node = pair.cdr
	}
	body := strings.Join(parts, " ")
	if vsaIsNil(node) {
		return "(" + body + ")"
	}
	return "(" + body + " . " + node.ArliRepr() + ")"
}

func vsaIsNil(v ArliValue) bool {
	if v == nil {
		return true
	}
	_, ok := v.(ArliNil)
	return ok
}

// vsaVecEqual compares two vectors componentwise (exact, no tolerance).
func vsaVecEqual(a, b *ArliVSAVec) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a.re) != len(b.re) || len(a.im) != len(b.im) {
		return false
	}
	for k := range a.re {
		if a.re[k] != b.re[k] || a.im[k] != b.im[k] {
			return false
		}
	}
	return true
}

// vsaPairEqual compares two pairs car first, then cdr, recursively.
func vsaPairEqual(a, b *ArliVSAPair) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return arliEqual(a.car, b.car) && arliEqual(a.cdr, b.cdr)
}

// vsaSameIdentity is the `is` / `===` test used by cleanup memory and
// vsa-factorize: the same value, not an equal one.
func vsaSameIdentity(a, b ArliValue) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch av := a.(type) {
	case *ArliVSAVec:
		bv, ok := b.(*ArliVSAVec)
		return ok && av == bv
	case *ArliVSAPair:
		bv, ok := b.(*ArliVSAPair)
		return ok && av == bv
	case ArliList:
		bv, ok := b.(ArliList)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		if len(av) == 0 {
			return true
		}
		return &av[0] == &bv[0]
	default:
		// Comparable atoms (ArliInt, ArliFloat, ArliString, ArliSymbol,
		// ArliBool, ArliNil) and pointer types (ArliFn, ArliBuiltin, GoValue).
		return a == b
	}
}

// ---------------------------------------------------------------------------
// PRNG — xoshiro128** over uint32 lanes seeded through splitmix32.
// ---------------------------------------------------------------------------

type vsaRng struct {
	s [4]uint32
}

func vsaSplitmix32(x uint32) (uint32, uint32) {
	x += 0x9E3779B9
	z := x
	z = (z ^ (z >> 16)) * 0x21F0AAAD
	z = (z ^ (z >> 15)) * 0x735A2D97
	return x, z ^ (z >> 15)
}

func vsaNewRng(seed uint32) *vsaRng {
	r := &vsaRng{}
	x := seed
	for i := 0; i < 4; i++ {
		x, r.s[i] = vsaSplitmix32(x)
	}
	if r.s[0]|r.s[1]|r.s[2]|r.s[3] == 0 {
		r.s[0] = 1
	}
	return r
}

func (r *vsaRng) nextU32() uint32 {
	result := bits.RotateLeft32(r.s[1]*5, 7) * 9
	t := r.s[1] << 9
	r.s[2] ^= r.s[0]
	r.s[3] ^= r.s[1]
	r.s[1] ^= r.s[2]
	r.s[0] ^= r.s[3]
	r.s[2] ^= t
	r.s[3] = bits.RotateLeft32(r.s[3], 11)
	return result
}

// nextFloat returns a float64 in [0, 1).
func (r *vsaRng) nextFloat() float64 {
	return float64(r.nextU32()) / 4294967296.0
}

// vsaFnv1a32 is 32-bit FNV-1a over the UTF-8 bytes of s.
func vsaFnv1a32(s string) uint32 {
	h := uint32(0x811C9DC5)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 0x01000193
	}
	return h
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

type vsaMemoryEntry struct {
	vec   *ArliVSAVec
	value ArliValue
}

type vsaEngine struct {
	dim        int
	streamSeed uint32 // seed the stream restarts from on vsa-reset
	rng        *vsaRng
	carRole    *ArliVSAVec
	cdrRole    *ArliVSAVec
	nilVec     *ArliVSAVec
	trueVec    *ArliVSAVec
	falseVec   *ArliVSAVec
	fpeFreq    []int
	symVecs    map[string]*ArliVSAVec
	strVecs    map[string]*ArliVSAVec
	memory     []vsaMemoryEntry
}

func vsaNewEngine(dim int) *vsaEngine {
	e := &vsaEngine{dim: dim, streamSeed: vsaDefaultStreamSeed, rng: vsaNewRng(vsaDefaultStreamSeed)}
	e.initSpecials()
	return e
}

// initSpecials (re)builds the dimension-dependent state, caches and memory.
// The random stream is deliberately left alone — it is owned by vsa-seed.
func (e *vsaEngine) initSpecials() {
	d := e.dim
	e.carRole = vsaRandomVecFromSeed(vsaRoleSeedCar, d)
	e.cdrRole = vsaRandomVecFromSeed(vsaRoleSeedCdr, d)
	e.nilVec = vsaRandomVecFromSeed(vsaSeedNil, d)
	e.trueVec = vsaRandomVecFromSeed(vsaSeedTrue, d)
	e.falseVec = vsaRandomVecFromSeed(vsaSeedFalse, d)

	basis := vsaNewRng(vsaSeedFpeBasis)
	e.fpeFreq = make([]int, d)
	for k := range e.fpeFreq {
		e.fpeFreq[k] = 1 + int(basis.nextFloat()*float64(d/2))
	}

	e.symVecs = make(map[string]*ArliVSAVec)
	e.strVecs = make(map[string]*ArliVSAVec)
	e.memory = nil
}

// reset reinitializes the engine at dimension dim: memory and caches are
// cleared and the random stream restarts from its current seed.
func (e *vsaEngine) reset(dim int) {
	e.dim = dim
	e.rng = vsaNewRng(e.streamSeed)
	e.initSpecials()
}

func vsaRandomVecFromSeed(seed uint32, d int) *ArliVSAVec {
	rng := vsaNewRng(seed)
	v := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		phase := 2.0 * math.Pi * rng.nextFloat()
		v.re[k] = float32(math.Cos(phase))
		v.im[k] = float32(math.Sin(phase))
	}
	return v
}

// randomVec draws a fresh vector from the engine's stream.
func (e *vsaEngine) randomVec() *ArliVSAVec {
	d := e.dim
	v := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		phase := 2.0 * math.Pi * e.rng.nextFloat()
		v.re[k] = float32(math.Cos(phase))
		v.im[k] = float32(math.Sin(phase))
	}
	return v
}

// ---------------------------------------------------------------------------
// Element-wise operations (computed in float64, stored as float32)
// ---------------------------------------------------------------------------

func (e *vsaEngine) bind(a, b *ArliVSAVec) *ArliVSAVec {
	d := e.dim
	out := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		ar, ai := float64(a.re[k]), float64(a.im[k])
		br, bi := float64(b.re[k]), float64(b.im[k])
		out.re[k] = float32(ar*br - ai*bi)
		out.im[k] = float32(ar*bi + ai*br)
	}
	return out
}

// unbind returns a * conj(b).
func (e *vsaEngine) unbind(a, b *ArliVSAVec) *ArliVSAVec {
	d := e.dim
	out := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		ar, ai := float64(a.re[k]), float64(a.im[k])
		br, bi := float64(b.re[k]), float64(b.im[k])
		out.re[k] = float32(ar*br + ai*bi)
		out.im[k] = float32(ai*br - ar*bi)
	}
	return out
}

// bundle sums the vectors componentwise and normalizes every component back
// onto the unit circle (FHRR centroid: sre[k], sim[k] divided by their own
// magnitude).
func (e *vsaEngine) bundle(vs []*ArliVSAVec) *ArliVSAVec {
	d := e.dim
	out := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		sre := 0.0
		sim := 0.0
		for _, v := range vs {
			sre += float64(v.re[k])
			sim += float64(v.im[k])
		}
		mag := math.Sqrt(sre*sre + sim*sim)
		if mag < 1e-12 {
			continue // all-zero component
		}
		out.re[k] = float32(sre / mag)
		out.im[k] = float32(sim / mag)
	}
	return out
}

// similarity is the cosine similarity in [-1, 1].
func (e *vsaEngine) similarity(a, b *ArliVSAVec) float64 {
	s := 0.0
	for k := 0; k < e.dim; k++ {
		s += float64(a.re[k])*float64(b.re[k]) + float64(a.im[k])*float64(b.im[k])
	}
	s /= float64(e.dim)
	if s > 1 {
		return 1
	}
	if s < -1 {
		return -1
	}
	return s
}

// permute shifts the vector by n positions; out[(k + n mod D) mod D] = v[k].
func (e *vsaEngine) permute(v *ArliVSAVec, n int64) *ArliVSAVec {
	d := e.dim
	shift := int(((n % int64(d)) + int64(d)) % int64(d))
	out := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		j := (k + shift) % d
		out.re[j] = v.re[k]
		out.im[j] = v.im[k]
	}
	return out
}

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

func (e *vsaEngine) encode(value ArliValue) (*ArliVSAVec, error) {
	switch v := value.(type) {
	case *ArliVSAVec:
		return v, nil

	case *ArliVSAPair:
		if v.vec == nil {
			carVec, err := e.encode(v.car)
			if err != nil {
				return nil, err
			}
			cdrVec, err := e.encode(v.cdr)
			if err != nil {
				return nil, err
			}
			v.vec = e.bundle([]*ArliVSAVec{e.bind(e.carRole, carVec), e.bind(e.cdrRole, cdrVec)})
		}
		return v.vec, nil

	case ArliNil:
		return e.nilVec, nil

	case ArliBool:
		if bool(v) {
			return e.trueVec, nil
		}
		return e.falseVec, nil

	case ArliInt:
		return e.encodeNumber(float64(v)), nil

	case ArliFloat:
		return e.encodeNumber(float64(v)), nil

	case ArliString:
		text := string(v)
		if vec, ok := e.strVecs[text]; ok {
			return vec, nil
		}
		vec := vsaRandomVecFromSeed(vsaFnv1a32("s:"+text), e.dim)
		e.strVecs[text] = vec
		return vec, nil

	case ArliSymbol:
		name := string(v)
		if vec, ok := e.symVecs[name]; ok {
			return vec, nil
		}
		vec := vsaRandomVecFromSeed(vsaFnv1a32("y:"+name), e.dim)
		e.symVecs[name] = vec
		return vec, nil

	case ArliList:
		var acc ArliValue = Nil
		for i := len(v) - 1; i >= 0; i-- {
			acc = &ArliVSAPair{car: v[i], cdr: acc}
		}
		return e.encode(acc)

	case *GoValue:
		if v.Value.Kind() == reflect.Map {
			return nil, fmt.Errorf("vsa: cannot encode map")
		}
		return nil, fmt.Errorf("vsa: cannot encode unknown")

	case *ArliBuiltin:
		return nil, fmt.Errorf("vsa: cannot encode builtin")

	case *ArliFn:
		return nil, fmt.Errorf("vsa: cannot encode fn")

	default:
		return nil, fmt.Errorf("vsa: cannot encode unknown")
	}
}

// encodeNumber is fractional power encoding of x.
func (e *vsaEngine) encodeNumber(x float64) *ArliVSAVec {
	d := e.dim
	out := &ArliVSAVec{re: make([]float32, d), im: make([]float32, d)}
	for k := 0; k < d; k++ {
		t := x * float64(e.fpeFreq[k]) / vsaFpePeriod
		t -= math.Floor(t)
		phase := 2.0 * math.Pi * t
		out.re[k] = float32(math.Cos(phase))
		out.im[k] = float32(math.Sin(phase))
	}
	return out
}

// ---------------------------------------------------------------------------
// Cleanup (associative) memory
// ---------------------------------------------------------------------------

// remember encodes value and stores it; an entry holding the same value by
// identity is updated in place instead of duplicated.
func (e *vsaEngine) remember(value ArliValue) error {
	vec, err := e.encode(value)
	if err != nil {
		return err
	}
	for i := range e.memory {
		if vsaSameIdentity(e.memory[i].value, value) {
			e.memory[i].vec = vec
			return nil
		}
	}
	e.memory = append(e.memory, vsaMemoryEntry{vec: vec, value: value})
	return nil
}

// lookup returns the value whose vector is most similar to vec, when that
// similarity reaches the cleanup threshold.
func (e *vsaEngine) lookup(vec *ArliVSAVec) ArliValue {
	if len(e.memory) == 0 {
		return Nil
	}
	best := math.Inf(-1)
	bestValue := ArliValue(Nil)
	for _, entry := range e.memory {
		score := e.similarity(vec, entry.vec)
		if score > best {
			best = score
			bestValue = entry.value
		}
	}
	if best >= vsaCleanupThreshold {
		return bestValue
	}
	return Nil
}

// nearest returns the highest-similarity candidate, with no threshold.
func (e *vsaEngine) nearest(vec *ArliVSAVec, candidates ArliList) (ArliValue, error) {
	best := math.Inf(-1)
	bestValue := ArliValue(Nil)
	for _, item := range candidates {
		itemVec, err := e.encode(item)
		if err != nil {
			return nil, err
		}
		if score := e.similarity(vec, itemVec); score > best {
			best = score
			bestValue = item
		}
	}
	return bestValue, nil
}

// ---------------------------------------------------------------------------
// vsa-match walker
// ---------------------------------------------------------------------------

type vsaStep bool

const (
	vsaStepCar vsaStep = false
	vsaStepCdr vsaStep = true
)

// walk follows steps without cleaning up intermediate nodes; only a VSAVec
// leaf is resolved through the cleanup memory.
func (e *vsaEngine) walk(node ArliValue, steps []vsaStep) ArliValue {
	for _, step := range steps {
		switch n := node.(type) {
		case *ArliVSAPair:
			if step == vsaStepCar {
				node = n.car
			} else {
				node = n.cdr
			}
		case ArliList:
			if step == vsaStepCar {
				if len(n) > 0 {
					node = n[0]
				} else {
					node = Nil
				}
			} else {
				if len(n) > 1 {
					node = n[1:]
				} else {
					node = Nil
				}
			}
		case ArliNil:
			node = Nil
		case *ArliVSAVec:
			if step == vsaStepCar {
				node = e.unbind(n, e.carRole) // raw: keep the vector
			} else {
				node = e.unbind(n, e.cdrRole)
			}
		default:
			node = Nil // atom: cannot descend further
		}
	}
	if vec, ok := node.(*ArliVSAVec); ok {
		return e.lookup(vec) // the single cleanup
	}
	return node
}

// pathTo extends prefix with i cdr steps followed by one car step.
func vsaPathTo(prefix []vsaStep, i int) []vsaStep {
	path := make([]vsaStep, 0, len(prefix)+i+1)
	path = append(path, prefix...)
	for j := 0; j < i; j++ {
		path = append(path, vsaStepCdr)
	}
	return append(path, vsaStepCar)
}

// match binds every `?name` of a proper-list pattern against value.
func (e *vsaEngine) match(pattern, value ArliValue) ArliList {
	bindings := make(ArliList, 0)
	var visit func(pat ArliValue, path []vsaStep)
	visit = func(pat ArliValue, path []vsaStep) {
		switch p := pat.(type) {
		case ArliSymbol:
			if strings.HasPrefix(string(p), "?") {
				bindings = append(bindings, ArliList{p, e.walk(value, path)})
			}
		case ArliList:
			for i, sub := range p {
				visit(sub, vsaPathTo(path, i))
			}
		}
	}
	visit(pattern, nil)
	return bindings
}

// ---------------------------------------------------------------------------
// vsa-type
// ---------------------------------------------------------------------------

func vsaTypeName(value ArliValue) ArliSymbol {
	switch v := value.(type) {
	case *ArliVSAVec:
		return ArliSymbol("vsa-vec")
	case *ArliVSAPair:
		return ArliSymbol("pair")
	case ArliNil:
		return ArliSymbol("nil")
	case ArliBool:
		return ArliSymbol("bool")
	case ArliInt, ArliFloat:
		return ArliSymbol("number")
	case ArliString:
		return ArliSymbol("string")
	case ArliSymbol:
		return ArliSymbol("symbol")
	case ArliList:
		return ArliSymbol("list")
	case *GoValue:
		if v.Value.Kind() == reflect.Map {
			return ArliSymbol("map")
		}
		return ArliSymbol("unknown")
	case *ArliBuiltin:
		return ArliSymbol("builtin")
	case *ArliFn:
		return ArliSymbol("fn")
	}
	return ArliSymbol("unknown")
}

// ---------------------------------------------------------------------------
// Primitives
// ---------------------------------------------------------------------------

type vsaFn func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error)

func getVsaBuiltins() map[string]*ArliBuiltin {
	builtins := make(map[string]*ArliBuiltin)

	add := func(name string, arity int, fn vsaFn) {
		builtins[name] = &ArliBuiltin{
			Name:  name,
			Arity: arity,
			Fn: func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
				if ev == nil {
					return nil, fmt.Errorf("%s: no evaluator", name)
				}
				return fn(ev.vsa(), args, ev)
			},
		}
	}

	// --- stream and dimensions ---

	add("vsa-dim", 0, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return ArliInt(e.dim), nil
	})

	add("vsa-reset", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		n, ok := vsaIntArg(args[0])
		if !ok || n < vsaMinDim || n > vsaMaxDim {
			return nil, fmt.Errorf("vsa-reset: dimension out of range")
		}
		e.reset(int(n))
		return ArliInt(n), nil
	})

	add("vsa-seed", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		n, ok := vsaIntArg(args[0])
		if !ok {
			return nil, fmt.Errorf("vsa-seed: expected integer")
		}
		e.streamSeed = uint32(n)
		e.rng = vsaNewRng(e.streamSeed)
		return ArliInt(n), nil
	})

	add("vsa-random", 0, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return e.randomVec(), nil
	})

	// --- vector algebra ---

	add("vsa-bind", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		b, err := e.encode(args[1])
		if err != nil {
			return nil, err
		}
		return e.bind(a, b), nil
	})

	add("vsa-bundle", -1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return vsaBundleArgs(e, args, "vsa-bundle")
	})

	add("vsa-majority", -1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return vsaBundleArgs(e, args, "vsa-majority")
	})

	add("vsa-unbind", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		b, err := e.encode(args[1])
		if err != nil {
			return nil, err
		}
		return e.unbind(a, b), nil
	})

	add("vsa-similarity", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		b, err := e.encode(args[1])
		if err != nil {
			return nil, err
		}
		return ArliFloat(e.similarity(a, b)), nil
	})

	add("vsa-permute", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		v, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		shift, ok := vsaIntArg(args[1])
		if !ok {
			return nil, fmt.Errorf("vsa-permute: expected integer shift")
		}
		return e.permute(v, shift), nil
	})

	add("vsa-encode", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return e.encode(args[0])
	})

	// --- pairs ---

	add("vsa-cons", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return &ArliVSAPair{car: args[0], cdr: args[1]}, nil
	})

	add("vsa-car", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return e.car(args[0])
	})

	add("vsa-cdr", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return e.cdr(args[0])
	})

	add("vsa-list", -1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		var acc ArliValue = Nil
		for i := len(args) - 1; i >= 0; i-- {
			acc = &ArliVSAPair{car: args[i], cdr: acc}
		}
		return acc, nil
	})

	add("vsa->list", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		switch v := args[0].(type) {
		case *ArliVSAPair:
			out := make(ArliList, 0, 4)
			var node ArliValue = v
			for {
				pair, ok := node.(*ArliVSAPair)
				if !ok {
					break
				}
				out = append(out, pair.car)
				node = pair.cdr
			}
			if !vsaIsNil(node) {
				return nil, fmt.Errorf("vsa->list: improper list")
			}
			return out, nil
		case ArliList:
			out := make(ArliList, len(v))
			copy(out, v)
			return out, nil
		case ArliNil:
			return ArliList{}, nil
		default:
			return nil, fmt.Errorf("vsa->list: expected vsa-pair or list")
		}
	})

	add("vsa-pair?", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(*ArliVSAPair)
		return boolResult(ok), nil
	})

	add("vsa-vec?", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(*ArliVSAVec)
		return boolResult(ok), nil
	})

	add("vsa-type", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return vsaTypeName(args[0]), nil
	})

	// --- cleanup memory ---

	add("vsa-register", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if err := e.remember(args[0]); err != nil {
			return nil, err
		}
		return args[0], nil
	})

	add("vsa-cleanup", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		vec, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		return e.lookup(vec), nil
	})

	add("vsa-query", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		vec, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		entries := make([]vsaMemoryEntry, len(e.memory))
		copy(entries, e.memory)
		scores := make([]float64, len(entries))
		for i, entry := range entries {
			scores[i] = e.similarity(vec, entry.vec)
		}
		order := make([]int, len(entries))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })

		k := int(toInt(args[1]))
		if k > 0 && k < len(order) {
			order = order[:k]
		}
		out := make(ArliList, 0, len(order))
		for _, idx := range order {
			out = append(out, &ArliVSAPair{car: ArliFloat(scores[idx]), cdr: entries[idx].value})
		}
		return out, nil
	})

	add("vsa-clear", 0, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		e.memory = nil
		return Nil, nil
	})

	// --- resonator factorization ---

	add("vsa-factorize", -1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("vsa-factorize: needs at least 2 arguments")
		}
		bound, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		codebooks := make([]ArliList, len(args)-1)
		est := make([]ArliValue, len(args)-1)
		for i, arg := range args[1:] {
			codebook, ok := arg.(ArliList)
			if !ok {
				return nil, fmt.Errorf("vsa-factorize: expected codebook list")
			}
			if len(codebook) == 0 {
				return nil, fmt.Errorf("vsa-factorize: empty codebook")
			}
			codebooks[i] = codebook
			vectors := make([]*ArliVSAVec, len(codebook))
			for j, item := range codebook {
				itemVec, err := e.encode(item)
				if err != nil {
					return nil, err
				}
				vectors[j] = itemVec
			}
			est[i] = e.bundle(vectors)
		}

		for round := 0; round < 32; round++ {
			changed := false
			for i := range est {
				others := make([]*ArliVSAVec, 0, len(est)-1)
				for j := range est {
					if j == i {
						continue
					}
					otherVec, err := e.encode(est[j])
					if err != nil {
						return nil, err
					}
					others = append(others, otherVec)
				}
				cand := e.unbind(bound, e.bundle(others))
				next, err := e.nearest(cand, codebooks[i])
				if err != nil {
					return nil, err
				}
				if !vsaSameIdentity(next, est[i]) {
					est[i] = next
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		return ArliList(est), nil
	})

	// --- pattern matching ---

	add("vsa-match", 2, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return e.match(args[0], args[1]), nil
	})

	// --- raw float bridge ---

	add("vsa->floats", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		vec, err := e.encode(args[0])
		if err != nil {
			return nil, err
		}
		out := make(ArliList, 0, 2*e.dim)
		for k := 0; k < e.dim; k++ {
			out = append(out, ArliFloat(float64(vec.re[k])), ArliFloat(float64(vec.im[k])))
		}
		return out, nil
	})

	add("floats->vsa", 1, func(e *vsaEngine, args []ArliValue, ev *Evaluator) (ArliValue, error) {
		list, ok := args[0].(ArliList)
		if !ok || len(list) != 2*e.dim {
			return nil, fmt.Errorf("floats->vsa: expected 2*D floats")
		}
		for _, item := range list {
			if !isNumeric(item) {
				return nil, fmt.Errorf("floats->vsa: expected 2*D floats")
			}
		}
		vec := &ArliVSAVec{re: make([]float32, e.dim), im: make([]float32, e.dim)}
		for k := 0; k < e.dim; k++ {
			vec.re[k] = float32(toFloat(list[2*k]))
			vec.im[k] = float32(toFloat(list[2*k+1]))
		}
		return vec, nil
	})

	return builtins
}

// vsaIntArg reads an integer argument: ints, or floats with no fractional
// part. Everything else (strings, symbols, non-integral floats) is rejected.
func vsaIntArg(value ArliValue) (int64, bool) {
	switch n := value.(type) {
	case ArliInt:
		return int64(n), true
	case ArliFloat:
		f := float64(n)
		// Reject non-integral, non-finite and out-of-int64-range values so the
		// conversion below is always well defined.
		if math.IsNaN(f) || f != math.Trunc(f) || math.Abs(f) > 1<<62 {
			return 0, false
		}
		return int64(f), true
	}
	return 0, false
}

// vsaBundleArgs is shared by vsa-bundle and vsa-majority (FHRR centroid).
func vsaBundleArgs(e *vsaEngine, args []ArliValue, name string) (ArliValue, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%s: needs at least 1 argument", name)
	}
	vectors := make([]*ArliVSAVec, len(args))
	for i, arg := range args {
		vec, err := e.encode(arg)
		if err != nil {
			return nil, err
		}
		vectors[i] = vec
	}
	return e.bundle(vectors), nil
}

// car implements §8's vsa-car rules.
func (e *vsaEngine) car(value ArliValue) (ArliValue, error) {
	switch v := value.(type) {
	case *ArliVSAPair:
		return v.car, nil
	case *ArliVSAVec:
		return e.lookup(e.unbind(v, e.carRole)), nil
	case ArliList:
		if len(v) == 0 {
			return Nil, nil
		}
		return v[0], nil
	case ArliNil:
		return Nil, nil
	default:
		return nil, fmt.Errorf("vsa-car: expected vsa-pair, vsa-vec, or list")
	}
}

// cdr implements §8's vsa-cdr rules.
func (e *vsaEngine) cdr(value ArliValue) (ArliValue, error) {
	switch v := value.(type) {
	case *ArliVSAPair:
		return v.cdr, nil
	case *ArliVSAVec:
		return e.lookup(e.unbind(v, e.cdrRole)), nil
	case ArliList:
		if len(v) <= 1 {
			return Nil, nil
		}
		out := make(ArliList, len(v)-1)
		copy(out, v[1:])
		return out, nil
	case ArliNil:
		return Nil, nil
	default:
		return nil, fmt.Errorf("vsa-cdr: expected vsa-pair, vsa-vec, or list")
	}
}
