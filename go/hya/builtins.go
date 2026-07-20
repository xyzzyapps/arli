package main

import (
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Builtin registry
// ---------------------------------------------------------------------------

func getBuiltins() map[string]*HyaBuiltin {
	b := make(map[string]*HyaBuiltin)

	add := func(name string, fn func([]HyaValue, *Evaluator) (HyaValue, error), arity int) {
		b[name] = &HyaBuiltin{Name: name, Fn: fn, Arity: arity}
	}

	// Arithmetic
	add("+", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return HyaFloat(a + b), nil
	}, 2)
	add("-", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return HyaFloat(a - b), nil
	}, 2)
	add("*", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return HyaFloat(a * b), nil
	}, 2)
	add("/", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return HyaFloat(a / b), nil
	}, 2)
	add("//", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toInt(args[0]), toInt(args[1])
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return HyaInt(a / b), nil
	}, 2)
	add("%", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toInt(args[0]), toInt(args[1])
		if b == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return HyaInt(a % b), nil
	}, 2)
	add("neg", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a := toFloat(args[0])
		return HyaFloat(-a), nil
	}, 1)

	// Comparison
	add("=", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a == b), nil
	}, 2)
	add("<", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a < b), nil
	}, 2)
	add(">", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a > b), nil
	}, 2)
	add("<=", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a <= b), nil
	}, 2)
	add(">=", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a >= b), nil
	}, 2)
	add("!=", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a != b), nil
	}, 2)

	// Logic
	add("not", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return boolResult(!IsTruthy(args[0])), nil
	}, 1)

	// Stack operations
	add("dup", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		ev.Stack = append(ev.Stack, args[0])
		return args[0], nil
	}, 1)
	add("swap", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Returns (b, a) — just arranges them on the stack
		return args[1], nil
	}, 2)
	add("drop", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return Nil, nil
	}, 1)
	add("over", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		ev.Stack = append(ev.Stack, args[0])
		return args[1], nil
	}, 2)
	add("rot", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Returns (b, c, a) from (a, b, c)
		return args[2], nil
	}, 3)
	add("nip", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return args[1], nil
	}, 2)
	add("tuck", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		ev.Stack = append(ev.Stack, args[1], args[0], args[1])
		return args[1], nil
	}, 2)

	// List operations
	add("cons", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := args[0], args[1]
		if list, ok := b.(HyaList); ok {
			result := make(HyaList, len(list)+1)
			result[0] = a
			copy(result[1:], list)
			return result, nil
		}
		// Check if b is nil
		if _, ok := b.(HyaNil); ok {
			return HyaList{a}, nil
		}
		return HyaList{a, b}, nil
	}, 2)
	add("car", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		list, ok := args[0].(HyaList)
		if !ok || len(list) == 0 {
			return Nil, nil
		}
		return list[0], nil
	}, 1)
	add("cdr", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		list, ok := args[0].(HyaList)
		if !ok || len(list) <= 1 {
			return Nil, nil
		}
		result := make(HyaList, len(list)-1)
		copy(result, list[1:])
		return result, nil
	}, 1)

	// I/O
	add("print", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		for _, a := range args {
			fmt.Println(a.HyaRepr())
		}
		if len(args) > 0 {
			return args[0], nil
		}
		return Nil, nil
	}, -1)

	// Type predicates
	add("number?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok1 := args[0].(HyaInt)
		_, ok2 := args[0].(HyaFloat)
		return boolResult(ok1 || ok2), nil
	}, 1)
	add("string?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok := args[0].(HyaString)
		return boolResult(ok), nil
	}, 1)
	add("symbol?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok := args[0].(HyaSymbol)
		return boolResult(ok), nil
	}, 1)
	add("nil?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok := args[0].(HyaNil)
		return boolResult(ok), nil
	}, 1)
	add("list?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok := args[0].(HyaList)
		return boolResult(ok), nil
	}, 1)
	add("fn?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok1 := args[0].(*HyaFn)
		_, ok2 := args[0].(*HyaBuiltin)
		return boolResult(ok1 || ok2), nil
	}, 1)

	return b
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toFloat(v HyaValue) float64 {
	switch n := v.(type) {
	case HyaInt:
		return float64(n)
	case HyaFloat:
		return float64(n)
	default:
		return math.NaN()
	}
}

func toInt(v HyaValue) int64 {
	switch n := v.(type) {
	case HyaInt:
		return int64(n)
	case HyaFloat:
		return int64(n)
	default:
		return 0
	}
}

func boolResult(b bool) HyaValue {
	if b {
		return True
	}
	return False
}
