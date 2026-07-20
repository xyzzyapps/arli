package main

import (
	"fmt"
	"math"
	"reflect"
)

// ---------------------------------------------------------------------------
// Builtin registry
// ---------------------------------------------------------------------------

func getBuiltins() map[string]*HyaBuiltin {
	b := make(map[string]*HyaBuiltin)

	add := func(name string, fn func([]HyaValue, *Evaluator) (HyaValue, error), arity int) {
		b[name] = &HyaBuiltin{Name: name, Fn: fn, Arity: arity}
	}

	// Arithmetic — return int when both args are int
	add("+", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ai, aok := args[0].(HyaInt); aok {
			if bi, bok := args[1].(HyaInt); bok {
				return HyaInt(int64(ai) + int64(bi)), nil
			}
		}
		a, b := toFloat(args[0]), toFloat(args[1])
		return HyaFloat(a + b), nil
	}, 2)
	add("-", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ai, aok := args[0].(HyaInt); aok {
			if bi, bok := args[1].(HyaInt); bok {
				return HyaInt(int64(ai) - int64(bi)), nil
			}
		}
		a, b := toFloat(args[0]), toFloat(args[1])
		return HyaFloat(a - b), nil
	}, 2)
	add("*", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ai, aok := args[0].(HyaInt); aok {
			if bi, bok := args[1].(HyaInt); bok {
				return HyaInt(int64(ai) * int64(bi)), nil
			}
		}
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
		if ai, ok := args[0].(HyaInt); ok {
			return HyaInt(-int64(ai)), nil
		}
		return HyaFloat(-toFloat(args[0])), nil
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
	add("and", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Short-circuit AND — variadic
		for _, arg := range args {
			if !IsTruthy(arg) {
				return arg, nil
			}
		}
		if len(args) > 0 {
			return args[len(args)-1], nil
		}
		return True, nil
	}, -1)
	add("or", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Short-circuit OR — variadic
		for _, arg := range args {
			if IsTruthy(arg) {
				return arg, nil
			}
		}
		return Nil, nil
	}, -1)
	add("not", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return boolResult(!IsTruthy(args[0])), nil
	}, 1)

	// Stack operations (Forth-like)
	add("dup", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev != nil {
			ev.Stack = append(ev.Stack, args[0])
		}
		return args[0], nil
	}, 1)
	add("swap", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev != nil {
			// Remove both, push back in swapped order
			ev.Stack = ev.Stack[:len(ev.Stack)-2]
			ev.Stack = append(ev.Stack, args[1], args[0])
		}
		return args[1], nil
	}, 2)
	add("drop", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return Nil, nil
	}, 1)
	add("over", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev != nil {
			ev.Stack = append(ev.Stack, args[0])
		}
		return args[0], nil
	}, 2)
	add("rot", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev != nil {
			// (a b c) -> (b c a): args = [a, b, c]
			// Remove all three, push b, c, a
			ev.Stack = ev.Stack[:len(ev.Stack)-3]
			ev.Stack = append(ev.Stack, args[1], args[2], args[0])
		}
		return args[2], nil
	}, 3)
	add("nip", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev != nil {
			// (a b) -> (b): args = [a, b], keep b only
			ev.Stack = ev.Stack[:len(ev.Stack)-2]
			ev.Stack = append(ev.Stack, args[1])
		}
		return args[1], nil
	}, 2)
	add("tuck", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev != nil {
			// (a b) -> (b a b): args = [a, b]
			ev.Stack = ev.Stack[:len(ev.Stack)-2]
			ev.Stack = append(ev.Stack, args[1], args[0], args[1])
		}
		return args[1], nil
	}, 2)

	// Stack shorthand: pick and roll
	add("pick", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Copy nth element (0=top) to top. pick 0 = dup, pick 1 = over.
		if ev != nil {
			n := int(toInt(args[0]))
			stackLen := len(ev.Stack)
			if n >= 0 && n < stackLen {
				srcIdx := stackLen - 1 - n
				val := ev.Stack[srcIdx]
				ev.Stack = append(ev.Stack, val)
				return val, nil
			}
		}
		return args[0], nil
	}, 1)
	add("roll", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Rotate nth element (0=top) to top. roll 1 = swap, roll 2 = rot.
		if ev != nil {
			depth := int(toInt(args[0]))
			stackLen := len(ev.Stack)
			if depth >= 0 && depth < stackLen {
				rollIdx := stackLen - 1 - depth
				val := ev.Stack[rollIdx]
				ev.Stack = append(ev.Stack[:rollIdx], ev.Stack[rollIdx+1:]...)
				ev.Stack = append(ev.Stack, val)
				return val, nil
			}
		}
		return args[0], nil
	}, 1)

	// List operations
	add("cons", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		a, b := args[0], args[1]
		if list, ok := b.(HyaList); ok {
			result := make(HyaList, len(list)+1)
			result[0] = a
			copy(result[1:], list)
			return result, nil
		}
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
	add("list", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return HyaList(args), nil
	}, -1)

	// Type predicates
	add("nil?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok := args[0].(HyaNil)
		return boolResult(ok), nil
	}, 1)
	add("list?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok := args[0].(HyaList)
		return boolResult(ok), nil
	}, 1)
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
	add("fn?", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		_, ok1 := args[0].(*HyaFn)
		_, ok2 := args[0].(*HyaBuiltin)
		return boolResult(ok1 || ok2), nil
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
	}, 1)
	add(".", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		// Forth-style dot — print value (same as print with arity 1)
		if len(args) > 0 {
			fmt.Print(args[0].HyaRepr())
		}
		if len(args) > 0 {
			return args[0], nil
		}
		return Nil, nil
	}, 1)
	add("read", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		var s string
		_, err := fmt.Scanln(&s)
		if err != nil {
			return Nil, nil
		}
		return HyaString(s), nil
	}, 0)

	// Sequence operations
	add("map", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		fn := args[0]
		lst, ok := args[1].(HyaList)
		if !ok || ev == nil {
			return args[1], nil
		}
		result := make(HyaList, len(lst))
		for i, x := range lst {
			v, err := ev.apply(fn, []HyaValue{x})
			if err != nil {
				return nil, err
			}
			result[i] = v
		}
		return result, nil
	}, 2)
	add("filter", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		fn := args[0]
		lst, ok := args[1].(HyaList)
		if !ok || ev == nil {
			return args[1], nil
		}
		result := make(HyaList, 0)
		for _, x := range lst {
			v, err := ev.apply(fn, []HyaValue{x})
			if err != nil {
				return nil, err
			}
			if IsTruthy(v) {
				result = append(result, x)
			}
		}
		return result, nil
	}, 2)
	add("reduce", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		fn := args[0]
		init := args[1]
		lst, ok := args[2].(HyaList)
		if !ok || len(lst) == 0 || ev == nil {
			return init, nil
		}
		acc := init
		var err error
		for _, x := range lst {
			acc, err = ev.apply(fn, []HyaValue{acc, x})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}, 3)

	// Hash-map creation
	add("hash-map", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		result := make(map[string]HyaValue)
		for i := 0; i < len(args)-1; i += 2 {
			key := args[i]
			val := args[i+1]
			if sym, ok := key.(HyaSymbol); ok {
				result[string(sym)] = val
			} else {
				result[key.HyaRepr()] = val
			}
		}
		return &GoValue{Value: reflect.ValueOf(result)}, nil
	}, -1)

	// Result type constructors
	add("Ok", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return HyaList{HyaSymbol("Ok"), args[0]}, nil
	}, 1)
	add("Err", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		return HyaList{HyaSymbol("Err"), args[0]}, nil
	}, 1)

	// Result type combinators
	add("map-ok", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev == nil {
			return args[0], nil
		}
		result := args[0]
		fn := args[1]
		if list, ok := result.(HyaList); ok && len(list) == 2 {
			if sym, ok := list[0].(HyaSymbol); ok && string(sym) == "Ok" {
				v, err := ev.apply(fn, []HyaValue{list[1]})
				if err != nil {
					return nil, err
				}
				return HyaList{HyaSymbol("Ok"), v}, nil
			}
		}
		return result, nil
	}, 2)
	add("and-then", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev == nil {
			return args[0], nil
		}
		result := args[0]
		fn := args[1]
		if list, ok := result.(HyaList); ok && len(list) == 2 {
			if sym, ok := list[0].(HyaSymbol); ok && string(sym) == "Ok" {
				return ev.apply(fn, []HyaValue{list[1]})
			}
		}
		return result, nil
	}, 2)
	add("or-else", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev == nil {
			return args[0], nil
		}
		result := args[0]
		fn := args[1]
		if list, ok := result.(HyaList); ok && len(list) == 2 {
			if sym, ok := list[0].(HyaSymbol); ok && string(sym) == "Ok" {
				return result, nil
			}
		}
		return ev.apply(fn, []HyaValue{})

	}, 2)

	// Evaluation control: eval builtin (for f-expressions)
	add("eval", func(args []HyaValue, ev *Evaluator) (HyaValue, error) {
		if ev == nil || len(args) == 0 {
			return Nil, nil
		}
		return ev.evalExpr(args[0])
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
