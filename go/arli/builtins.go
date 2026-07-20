package main

import (
	"fmt"
	"math"
	"reflect"
)

// ---------------------------------------------------------------------------
// Builtin registry
// ---------------------------------------------------------------------------

func getBuiltins() map[string]*ArliBuiltin {
	b := make(map[string]*ArliBuiltin)

	add := func(name string, fn func([]ArliValue, *Evaluator) (ArliValue, error), arity int) {
		b[name] = &ArliBuiltin{Name: name, Fn: fn, Arity: arity}
	}

	// Arithmetic â€” return int when both args are int
	add("+", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ai, aok := args[0].(ArliInt); aok {
			if bi, bok := args[1].(ArliInt); bok {
				return ArliInt(int64(ai) + int64(bi)), nil
			}
		}
		a, b := toFloat(args[0]), toFloat(args[1])
		return ArliFloat(a + b), nil
	}, 2)
	add("-", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ai, aok := args[0].(ArliInt); aok {
			if bi, bok := args[1].(ArliInt); bok {
				return ArliInt(int64(ai) - int64(bi)), nil
			}
		}
		a, b := toFloat(args[0]), toFloat(args[1])
		return ArliFloat(a - b), nil
	}, 2)
	add("*", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ai, aok := args[0].(ArliInt); aok {
			if bi, bok := args[1].(ArliInt); bok {
				return ArliInt(int64(ai) * int64(bi)), nil
			}
		}
		a, b := toFloat(args[0]), toFloat(args[1])
		return ArliFloat(a * b), nil
	}, 2)
	add("/", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return ArliFloat(a / b), nil
	}, 2)
	add("//", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toInt(args[0]), toInt(args[1])
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return ArliInt(a / b), nil
	}, 2)
	add("%", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toInt(args[0]), toInt(args[1])
		if b == 0 {
			return nil, fmt.Errorf("modulo by zero")
		}
		return ArliInt(a % b), nil
	}, 2)
	add("neg", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ai, ok := args[0].(ArliInt); ok {
			return ArliInt(-int64(ai)), nil
		}
		return ArliFloat(-toFloat(args[0])), nil
	}, 1)

	// Comparison
	add("=", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a == b), nil
	}, 2)
	add("<", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a < b), nil
	}, 2)
	add(">", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a > b), nil
	}, 2)
	add("<=", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a <= b), nil
	}, 2)
	add(">=", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a >= b), nil
	}, 2)
	add("!=", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := toFloat(args[0]), toFloat(args[1])
		return boolResult(a != b), nil
	}, 2)

	// Logic
	add("and", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		// Short-circuit AND â€” variadic
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
	add("or", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		// Short-circuit OR â€” variadic
		for _, arg := range args {
			if IsTruthy(arg) {
				return arg, nil
			}
		}
		return Nil, nil
	}, -1)
	add("not", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return boolResult(!IsTruthy(args[0])), nil
	}, 1)

	// Stack operations (Forth-like)
	add("dup", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			ev.Stack = append(ev.Stack, args[0])
		}
		return args[0], nil
	}, 1)
	add("swap", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			// Remove both, push back in swapped order
			ev.Stack = ev.Stack[:len(ev.Stack)-2]
			ev.Stack = append(ev.Stack, args[1], args[0])
		}
		return args[1], nil
	}, 2)
	add("drop", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return Nil, nil
	}, 1)
	add("over", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			ev.Stack = append(ev.Stack, args[0])
		}
		return args[0], nil
	}, 2)
	add("rot", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			// (a b c) -> (b c a): args = [a, b, c]
			// Remove all three, push b, c, a
			ev.Stack = ev.Stack[:len(ev.Stack)-3]
			ev.Stack = append(ev.Stack, args[1], args[2], args[0])
		}
		return args[2], nil
	}, 3)
	add("nip", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			// (a b) -> (b): args = [a, b], keep b only
			ev.Stack = ev.Stack[:len(ev.Stack)-2]
			ev.Stack = append(ev.Stack, args[1])
		}
		return args[1], nil
	}, 2)
	add("tuck", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			// (a b) -> (b a b): args = [a, b]
			ev.Stack = ev.Stack[:len(ev.Stack)-2]
			ev.Stack = append(ev.Stack, args[1], args[0], args[1])
		}
		return args[1], nil
	}, 2)

	// Stack shorthand: pick and roll
	add("pick", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
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
	add("roll", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
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
	add("cons", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		a, b := args[0], args[1]
		if list, ok := b.(ArliList); ok {
			result := make(ArliList, len(list)+1)
			result[0] = a
			copy(result[1:], list)
			return result, nil
		}
		if _, ok := b.(ArliNil); ok {
			return ArliList{a}, nil
		}
		return ArliList{a, b}, nil
	}, 2)
	add("car", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		list, ok := args[0].(ArliList)
		if !ok || len(list) == 0 {
			return Nil, nil
		}
		return list[0], nil
	}, 1)
	add("cdr", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		list, ok := args[0].(ArliList)
		if !ok || len(list) <= 1 {
			return Nil, nil
		}
		result := make(ArliList, len(list)-1)
		copy(result, list[1:])
		return result, nil
	}, 1)
	add("list", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return ArliList(args), nil
	}, -1)

	// Type predicates
	add("nil?", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(ArliNil)
		return boolResult(ok), nil
	}, 1)
	add("list?", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(ArliList)
		return boolResult(ok), nil
	}, 1)
	add("number?", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok1 := args[0].(ArliInt)
		_, ok2 := args[0].(ArliFloat)
		return boolResult(ok1 || ok2), nil
	}, 1)
	add("string?", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(ArliString)
		return boolResult(ok), nil
	}, 1)
	add("symbol?", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok := args[0].(ArliSymbol)
		return boolResult(ok), nil
	}, 1)
	add("fn?", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		_, ok1 := args[0].(*ArliFn)
		_, ok2 := args[0].(*ArliBuiltin)
		return boolResult(ok1 || ok2), nil
	}, 1)

	// I/O
	add("print", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		for _, a := range args {
			fmt.Println(a.ArliRepr())
		}
		if len(args) > 0 {
			return args[0], nil
		}
		return Nil, nil
	}, 1)
	add(".", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		// Forth-style dot â€” print value (same as print with arity 1)
		if len(args) > 0 {
			fmt.Print(args[0].ArliRepr())
		}
		if len(args) > 0 {
			return args[0], nil
		}
		return Nil, nil
	}, 1)
	add("read", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		var s string
		_, err := fmt.Scanln(&s)
		if err != nil {
			return Nil, nil
		}
		return ArliString(s), nil
	}, 0)

	// Sequence operations
	add("map", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		fn := args[0]
		lst, ok := args[1].(ArliList)
		if !ok || ev == nil {
			return args[1], nil
		}
		result := make(ArliList, len(lst))
		for i, x := range lst {
			v, err := ev.apply(fn, []ArliValue{x})
			if err != nil {
				return nil, err
			}
			result[i] = v
		}
		return result, nil
	}, 2)
	add("filter", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		fn := args[0]
		lst, ok := args[1].(ArliList)
		if !ok || ev == nil {
			return args[1], nil
		}
		result := make(ArliList, 0)
		for _, x := range lst {
			v, err := ev.apply(fn, []ArliValue{x})
			if err != nil {
				return nil, err
			}
			if IsTruthy(v) {
				result = append(result, x)
			}
		}
		return result, nil
	}, 2)
	add("reduce", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		fn := args[0]
		init := args[1]
		lst, ok := args[2].(ArliList)
		if !ok || len(lst) == 0 || ev == nil {
			return init, nil
		}
		acc := init
		var err error
		for _, x := range lst {
			acc, err = ev.apply(fn, []ArliValue{acc, x})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}, 3)

	// Hash-map creation
	add("hash-map", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		result := make(map[string]ArliValue)
		for i := 0; i < len(args)-1; i += 2 {
			key := args[i]
			val := args[i+1]
			if sym, ok := key.(ArliSymbol); ok {
				result[string(sym)] = val
			} else {
				result[key.ArliRepr()] = val
			}
		}
		return &GoValue{Value: reflect.ValueOf(result)}, nil
	}, -1)

	// Result type constructors
	add("Ok", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return ArliList{ArliSymbol("Ok"), args[0]}, nil
	}, 1)
	add("Err", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		return ArliList{ArliSymbol("Err"), args[0]}, nil
	}, 1)

	// Result type combinators
	add("map-ok", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return args[0], nil
		}
		result := args[0]
		fn := args[1]
		if list, ok := result.(ArliList); ok && len(list) == 2 {
			if sym, ok := list[0].(ArliSymbol); ok && string(sym) == "Ok" {
				v, err := ev.apply(fn, []ArliValue{list[1]})
				if err != nil {
					return nil, err
				}
				return ArliList{ArliSymbol("Ok"), v}, nil
			}
		}
		return result, nil
	}, 2)
	add("and-then", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return args[0], nil
		}
		result := args[0]
		fn := args[1]
		if list, ok := result.(ArliList); ok && len(list) == 2 {
			if sym, ok := list[0].(ArliSymbol); ok && string(sym) == "Ok" {
				return ev.apply(fn, []ArliValue{list[1]})
			}
		}
		return result, nil
	}, 2)
	add("or-else", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return args[0], nil
		}
		result := args[0]
		fn := args[1]
		if list, ok := result.(ArliList); ok && len(list) == 2 {
			if sym, ok := list[0].(ArliSymbol); ok && string(sym) == "Ok" {
				return result, nil
			}
		}
		return ev.apply(fn, []ArliValue{})

	}, 2)

	// Stack reflection: capture current stack as a list
	add("stack", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return ArliList{}, nil
		}
		cp := make(ArliList, len(ev.Stack))
		copy(cp, ev.Stack)
		return cp, nil
	}, 0)

	// Stack reflection: replace stack from a list
	add("stack!", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return Nil, nil
		}
		switch v := args[0].(type) {
		case ArliList:
			ev.Stack = make([]ArliValue, len(v))
			copy(ev.Stack, v)
		case ArliNil:
			ev.Stack = ev.Stack[:0]
		default:
			ev.Stack = []ArliValue{v}
		}
		return nil, nil // nil return skips the stack push in Eval()
	}, 1)

	// -----------------------------------------------------------------------
	// Exec stack operations (Push-like self-modifying code support)
	// -----------------------------------------------------------------------

	// exec-stack: push a copy of the exec stack to the data stack
	add("exec-stack", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return ArliList{}, nil
		}
		cp := make(ArliList, len(ev.ExecStack))
		copy(cp, ev.ExecStack)
		return cp, nil
	}, 0)

	// exec!: replace the exec stack with a list
	add("exec!", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return Nil, nil
		}
		switch v := args[0].(type) {
		case ArliList:
			ev.ExecStack = make([]ArliValue, len(v))
			copy(ev.ExecStack, v)
		case ArliNil:
			ev.ExecStack = ev.ExecStack[:0]
		default:
			ev.ExecStack = []ArliValue{v}
		}
		return nil, nil
	}, 1)

	// exec-push: push a form onto the exec stack
	add("exec-push", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev != nil {
			ev.ExecStack = append(ev.ExecStack, args[0])
		}
		return nil, nil
	}, 1)

	// exec-pop: pop top of exec stack to data stack
	add("exec-pop", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil || len(ev.ExecStack) == 0 {
			return Nil, nil
		}
		top := ev.ExecStack[len(ev.ExecStack)-1]
		ev.ExecStack = ev.ExecStack[:len(ev.ExecStack)-1]
		return top, nil
	}, 0)

	// exec-depth: push exec stack depth to data stack
	add("exec-depth", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return ArliInt(0), nil
		}
		return ArliInt(len(ev.ExecStack)), nil
	}, 0)

	// exec-step: pop and evaluate one form from exec stack
	add("exec-step", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil || len(ev.ExecStack) == 0 {
			return Nil, nil
		}
		top := ev.ExecStack[len(ev.ExecStack)-1]
		ev.ExecStack = ev.ExecStack[:len(ev.ExecStack)-1]
		return ev.evalExpr(top)
	}, 0)

	// exec (variadic, parens required): Push interpreter â€” process exec stack until empty
	add("exec", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
		if ev == nil {
			return Nil, nil
		}
		for len(ev.ExecStack) > 0 {
			item := ev.ExecStack[len(ev.ExecStack)-1]
			ev.ExecStack = ev.ExecStack[:len(ev.ExecStack)-1]

			switch v := item.(type) {
			case ArliList:
				result, err := ev.evalExpr(v)
				if err != nil {
					return nil, err
				}
				if result != nil {
					ev.Stack = append(ev.Stack, result)
				}

			case ArliSymbol:
				name := string(v)
				if name == "__restore_env__" {
					if len(ev.EnvStack) > 0 {
						ev.Env = ev.EnvStack[len(ev.EnvStack)-1]
						ev.EnvStack = ev.EnvStack[:len(ev.EnvStack)-1]
					}
					continue
				}
				if name == "if" {
					var condVal ArliValue = Nil
					if len(ev.Stack) > 0 {
						condVal = ev.Stack[len(ev.Stack)-1]
						ev.Stack = ev.Stack[:len(ev.Stack)-1]
					}
					var elseBranch ArliValue = Nil
					if len(ev.ExecStack) > 0 {
						elseBranch = ev.ExecStack[len(ev.ExecStack)-1]
						ev.ExecStack = ev.ExecStack[:len(ev.ExecStack)-1]
					}
					var thenBranch ArliValue = Nil
					if len(ev.ExecStack) > 0 {
						thenBranch = ev.ExecStack[len(ev.ExecStack)-1]
						ev.ExecStack = ev.ExecStack[:len(ev.ExecStack)-1]
					}
					if IsTruthy(condVal) {
						ev.ExecStack = append(ev.ExecStack, thenBranch)
					} else {
						ev.ExecStack = append(ev.ExecStack, elseBranch)
					}
					continue
				}
				if name == "do" {
					continue
				}
				if name == "quote" {
					var quoted ArliValue = Nil
					if len(ev.ExecStack) > 0 {
						quoted = ev.ExecStack[len(ev.ExecStack)-1]
						ev.ExecStack = ev.ExecStack[:len(ev.ExecStack)-1]
					}
					ev.Stack = append(ev.Stack, quoted)
					continue
				}
				fn, err := ev.Env.Get(name)
				if err != nil {
					return nil, err
				}
				switch f := fn.(type) {
				case *ArliBuiltin:
					arity := f.Arity
					if arity < 0 {
						arity = 0
					}
					fnArgs := make([]ArliValue, 0, arity)
					for i := 0; i < arity; i++ {
						if len(ev.Stack) > 0 {
							idx := len(ev.Stack) - 1
							fnArgs = append([]ArliValue{ev.Stack[idx]}, fnArgs...)
							ev.Stack = ev.Stack[:idx]
						}
					}
					result, err := f.Call(fnArgs, ev)
					if err != nil {
						return nil, err
					}
					if result != nil {
						ev.Stack = append(ev.Stack, result)
					}
				case *ArliFn:
					fnArgs := make([]ArliValue, 0, len(f.Params))
					for i := 0; i < len(f.Params); i++ {
						if len(ev.Stack) > 0 {
							idx := len(ev.Stack) - 1
							fnArgs = append([]ArliValue{ev.Stack[idx]}, fnArgs...)
							ev.Stack = ev.Stack[:idx]
						}
					}
					ev.EnvStack = append(ev.EnvStack, ev.Env)
					callEnv := NewEnvironment(f.Env, "call")
					for i, param := range f.Params {
						if i < len(fnArgs) {
							callEnv.Define(string(param), fnArgs[i])
						}
					}
					ev.ExecStack = append(ev.ExecStack, ArliSymbol("__restore_env__"))
					for i := len(f.Body) - 1; i >= 0; i-- {
						ev.ExecStack = append(ev.ExecStack, f.Body[i])
					}
					ev.Env = callEnv
				default:
					ev.Stack = append(ev.Stack, fn)
				}
			default:
				ev.Stack = append(ev.Stack, v)
			}
		}
		return nil, nil
	}, -1)

	// Evaluation control: eval builtin (for f-expressions)
	add("eval", func(args []ArliValue, ev *Evaluator) (ArliValue, error) {
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

func toFloat(v ArliValue) float64 {
	switch n := v.(type) {
	case ArliInt:
		return float64(n)
	case ArliFloat:
		return float64(n)
	default:
		return math.NaN()
	}
}

func toInt(v ArliValue) int64 {
	switch n := v.(type) {
	case ArliInt:
		return int64(n)
	case ArliFloat:
		return int64(n)
	default:
		return 0
	}
}

func boolResult(b bool) ArliValue {
	if b {
		return True
	}
	return False
}
