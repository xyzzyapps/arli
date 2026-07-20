package main

import (
	"fmt"
	"reflect"
	"strings"
)

// ---------------------------------------------------------------------------
// Evaluator
// ---------------------------------------------------------------------------

type Evaluator struct {
	Stack     []HyaValue
	Env       *Environment
	GlobalEnv *Environment
	Arities   *ArityTable
	parser    *Parser
}

func NewEvaluator() *Evaluator {
	ev := &Evaluator{
		Stack:     make([]HyaValue, 0),
		Arities:   NewArityTable(),
	}
	ev.GlobalEnv = NewEnvironment(nil, "global")
	ev.Env = ev.GlobalEnv
	ev.parser = NewParser(ev.Arities)
	ev.loadBuiltins()
	return ev
}

func (ev *Evaluator) loadBuiltins() {
	for name, b := range getBuiltins() {
		ev.GlobalEnv.Define(name, b)
		if b.Arity >= 0 {
			ev.Arities.Register(name, b.Arity)
		}
	}
	// Special form arities
	ev.Arities.Register("define", 2)
	ev.Arities.Register("quote", 1)
	ev.Arities.Register("do", -1)   // variadic — use parens
	ev.Arities.Register("set!", 2)
	ev.Arities.Register("let", 2)
	ev.Arities.Register("if", 3)
	ev.Arities.Register("while", 2)
	ev.Arities.Register("for", 3)
	ev.Arities.Register("cond", 1)
	ev.Arities.Register("defn", 3)  // defn name (params) body
	ev.Arities.Register("fn", 2)    // fn (params) body
	ev.Arities.Register("import", 1)
	ev.Arities.Register("import!", 2)
	ev.Arities.Register(".", 2)
	ev.Arities.Register("go", 1)
}

func (ev *Evaluator) Eval(expr HyaValue) (HyaValue, error) {
	result, err := ev.evalExpr(expr)
	if err != nil {
		return nil, err
	}
	if result != nil {
		ev.Stack = append(ev.Stack, result)
	}
	return result, nil
}

func (ev *Evaluator) evalExpr(expr HyaValue) (HyaValue, error) {
	// Literals
	switch v := expr.(type) {
	case HyaInt, HyaFloat, HyaString:
		return v, nil
	case HyaNil:
		return v, nil
	case HyaBool:
		return v, nil
	case HyaSymbol:
		name := string(v)
		// Constants
		if name == "nil" {
			return Nil, nil
		}
		if name == "true" {
			return True, nil
		}
		if name == "false" {
			return False, nil
		}
		val, err := ev.Env.Get(name)
		if err != nil {
			return nil, err
		}
		return val, nil

	case HyaList:
		if len(v) == 0 {
			return Nil, nil
		}

		head := v[0]
		sym, isSym := head.(HyaSymbol)
		if !isSym {
			// Generic call
			fn, err := ev.evalExpr(head)
			if err != nil {
				return nil, err
			}
			args := make([]HyaValue, len(v)-1)
			for i, a := range v[1:] {
				args[i], err = ev.evalExpr(a)
				if err != nil {
					return nil, err
				}
			}
			return ev.apply(fn, args)
		}

		name := string(sym)

		// ---- Special forms ----

		// QUOTE
		if name == "quote" {
			if len(v) < 2 {
				return nil, fmt.Errorf("quote expects 1 argument")
			}
			return v[1], nil
		}

		// DEFINE
		if name == "define" {
			if len(v) < 3 {
				return nil, fmt.Errorf("define expects (define name value)")
			}
			nameExpr := v[1]
			valueExpr := v[2]
			if sym, ok := nameExpr.(HyaSymbol); ok {
				val, err := ev.evalExpr(valueExpr)
				if err != nil {
					return nil, err
				}
				ev.Env.Define(string(sym), val)
				// Register arity for functions
				if fn, ok := val.(*HyaFn); ok {
					ev.Arities.Register(string(sym), len(fn.Params))
				} else if gv, ok := val.(*GoValue); ok && gv.Value.Kind() == reflect.Func {
					t := gv.Value.Type()
					numIn := t.NumIn()
					ev.Arities.Register(string(sym), numIn)
				}
				return val, nil
			}
			return nil, fmt.Errorf("define expects a symbol name")
		}

		// DEFN
		if name == "defn" {
			if len(v) < 4 {
				return nil, fmt.Errorf("defn expects (defn name (params) body...)")
			}
			nameSym, ok := v[1].(HyaSymbol)
			if !ok {
				return nil, fmt.Errorf("defn expects a symbol name")
			}
			params, ok := v[2].(HyaList)
			if !ok {
				return nil, fmt.Errorf("defn expects a parameter list")
			}
			paramSyms := make([]HyaSymbol, len(params))
			for i, p := range params {
				if s, ok := p.(HyaSymbol); ok {
					paramSyms[i] = s
				} else {
					return nil, fmt.Errorf("defn params must be symbols")
				}
			}
			body := v[3:]
			fn := &HyaFn{
				Name:   string(nameSym),
				Params: paramSyms,
				Body:   body,
				Env:    ev.Env,
			}
			ev.Env.Define(string(nameSym), fn)
			ev.Arities.Register(string(nameSym), len(paramSyms))
			return fn, nil
		}

		// IF
		if name == "if" {
			if len(v) < 4 {
				return nil, fmt.Errorf("if expects (if cond then else)")
			}
			cond, err := ev.evalExpr(v[1])
			if err != nil {
				return nil, err
			}
			if IsTruthy(cond) {
				return ev.evalExpr(v[2])
			}
			return ev.evalExpr(v[3])
		}

		// DO
		if name == "do" {
			result := HyaValue(Nil)
			var err error
			for _, sub := range v[1:] {
				result, err = ev.evalExpr(sub)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		}

		// FN
		if name == "fn" {
			if len(v) < 3 {
				return nil, fmt.Errorf("fn expects (fn (params) body...)")
			}
			params, ok := v[1].(HyaList)
			if !ok {
				return nil, fmt.Errorf("fn expects a parameter list")
			}
			paramSyms := make([]HyaSymbol, len(params))
			for i, p := range params {
				if s, ok := p.(HyaSymbol); ok {
					paramSyms[i] = s
				} else {
					return nil, fmt.Errorf("fn params must be symbols")
				}
			}
			body := v[2:]
			return &HyaFn{
				Params: paramSyms,
				Body:   body,
				Env:    ev.Env,
			}, nil
		}

		// WHILE
		if name == "while" {
			if len(v) < 3 {
				return nil, fmt.Errorf("while expects (while cond body)")
			}
			result := HyaValue(Nil)
			for {
				cond, err := ev.evalExpr(v[1])
				if err != nil {
					return nil, err
				}
				if !IsTruthy(cond) {
					break
				}
				for _, sub := range v[2:] {
					result, err = ev.evalExpr(sub)
					if err != nil {
						return nil, err
					}
				}
			}
			return result, nil
		}

		// FOR
		if name == "for" {
			if len(v) < 4 {
				return nil, fmt.Errorf("for expects (for var list body)")
			}
			varSym, ok := v[1].(HyaSymbol)
			if !ok {
				return nil, fmt.Errorf("for expects a symbol as variable")
			}
			listVal, err := ev.evalExpr(v[2])
			if err != nil {
				return nil, err
			}
			list, ok := listVal.(HyaList)
			if !ok {
				return nil, fmt.Errorf("for expects a list")
			}
			result := HyaValue(Nil)
			for _, item := range list {
				ev.Env.Define(string(varSym), item)
				for _, sub := range v[3:] {
					result, err = ev.evalExpr(sub)
					if err != nil {
						return nil, err
					}
				}
			}
			return result, nil
		}

		// COND
		if name == "cond" {
			if len(v) < 2 {
				return nil, fmt.Errorf("cond expects (cond clause...)")
			}
			clauses, ok := v[1].(HyaList)
			if !ok {
				return nil, fmt.Errorf("cond expects a clause list")
			}
			i := 0
			for i < len(clauses)-1 {
				test, err := ev.evalExpr(clauses[i])
				if err != nil {
					return nil, err
				}
				if IsTruthy(test) {
					return ev.evalExpr(clauses[i+1])
				}
				i += 2
			}
			if i < len(clauses) {
				if IsTruthy(clauses[i]) {
					// Trailing single truthy clause (no result)
				}
			}
			return Nil, nil
		}

		// SET!
		if name == "set!" {
			if len(v) < 3 {
				return nil, fmt.Errorf("set! expects (set! name value)")
			}
			nameExpr := v[1]
			val, err := ev.evalExpr(v[2])
			if err != nil {
				return nil, err
			}
			if sym, ok := nameExpr.(HyaSymbol); ok {
				err := ev.Env.Set(string(sym), val)
				if err != nil {
					return nil, err
				}
				return val, nil
			}
			return nil, fmt.Errorf("set! expects a symbol name")
		}

		// LET
		if name == "let" {
			if len(v) < 3 {
				return nil, fmt.Errorf("let expects (let bindings body)")
			}
			bindings, ok := v[1].(HyaList)
			if !ok {
				return nil, fmt.Errorf("let expects a bindings list")
			}
			body := v[2:]

			letEnv := NewEnvironment(ev.Env, "let")
			oldEnv := ev.Env
			ev.Env = letEnv
			var err error
			result := HyaValue(Nil)

			for _, binding := range bindings {
				if b, ok := binding.(HyaList); ok && len(b) >= 2 {
					if bname, ok := b[0].(HyaSymbol); ok {
						bval, e := ev.evalExpr(b[1])
						if e != nil {
							err = e
							break
						}
						ev.Env.Define(string(bname), bval)
					}
				}
			}
			if err == nil {
				for _, sub := range body {
					result, err = ev.evalExpr(sub)
					if err != nil {
						break
					}
				}
			}
			ev.Env = oldEnv
			return result, err
		}

		// IMPORT — Go module import via reflection
		if name == "import" {
			if len(v) < 2 {
				return nil, fmt.Errorf("import expects (import \"package\")")
			}
			pkgName := ""
			switch p := v[1].(type) {
			case HyaString:
				pkgName = string(p)
			case HyaSymbol:
				pkgName = string(p)
			default:
				return nil, fmt.Errorf("import expects a symbol or string")
			}
			// Use Go's reflection to find the package
			// For now, we use a simple approach: register known packages
			mod, err := importGoPackage(pkgName)
			if err != nil {
				return nil, err
			}
			ev.Env.Define(pkgName, mod)
			return mod, nil
		}

		// DOT — chained attribute access
		if name == "." {
			if len(v) < 3 {
				return nil, fmt.Errorf(". expects (. obj attr...)")
			}
			obj, err := ev.evalExpr(v[1])
			if err != nil {
				return nil, err
			}
			i := 2
			for i < len(v) {
				item := v[i]
				if sym, ok := item.(HyaSymbol); ok {
					// Attribute/method access
					if gv, ok := obj.(*GoValue); ok {
						obj, err = gv.GetField(string(sym))
						if err != nil {
							return nil, err
						}
					} else {
						// Try to find the method via reflection on the Go side
						return nil, fmt.Errorf("dot access requires Go value")
					}
					i++
				} else {
					break
				}
			}
			// Call if remaining args
			if i < len(v) {
				callArgs := make([]HyaValue, len(v)-i)
				for j := i; j < len(v); j++ {
					callArgs[j-i], err = ev.evalExpr(v[j])
					if err != nil {
						return nil, err
					}
				}
				if gv, ok := obj.(*GoValue); ok {
					return gv.Call(callArgs)
				}
				return nil, fmt.Errorf("cannot call non-Go value")
			}
			return obj, nil
		}

		// GO — evaluate arbitrary Go expression string
		if name == "go" {
			if len(v) < 2 {
				return nil, fmt.Errorf("go expects (go \"code\")")
			}
			code, ok := v[1].(HyaString)
			if !ok {
				return nil, fmt.Errorf("go expects a string")
			}
			return ev.evalGo(string(code))
		}

		// DEFAULTS
		// Generic function call
		fn, err := ev.evalExpr(head)
		if err != nil {
			return nil, err
		}
		args := make([]HyaValue, len(v)-1)
		for i, a := range v[1:] {
			args[i], err = ev.evalExpr(a)
			if err != nil {
				return nil, err
			}
		}
		return ev.apply(fn, args)
	}

	return Nil, nil
}

// ---------------------------------------------------------------------------
// Function application
// ---------------------------------------------------------------------------

func (ev *Evaluator) apply(fn HyaValue, args []HyaValue) (HyaValue, error) {
	switch f := fn.(type) {
	case *HyaBuiltin:
		return f.Call(args, ev)

	case *HyaFn:
		if len(args) != len(f.Params) {
			return nil, fmt.Errorf("function %s expected %d args, got %d",
				f.Name, len(f.Params), len(args))
		}
		callEnv := NewEnvironment(f.Env, "call")
		oldEnv := ev.Env
		ev.Env = callEnv
		defer func() { ev.Env = oldEnv }()

		for i, param := range f.Params {
			ev.Env.Define(string(param), args[i])
		}

		result := HyaValue(Nil)
		var err error
		for _, sub := range f.Body {
			result, err = ev.evalExpr(sub)
			if err != nil {
				return nil, err
			}
		}
		return result, nil

	case *GoValue:
		return f.Call(args)

	default:
		return nil, fmt.Errorf("cannot call non-function: %s", fn.HyaRepr())
	}
}

// ---------------------------------------------------------------------------
// Go interop helpers
// ---------------------------------------------------------------------------

func importGoPackage(pkgPath string) (HyaValue, error) {
	// Use a registry of known Go standard library packages.
	// In a full implementation, this would use go/packages or plugin.
	// For now, we provide a mapping of commonly useful packages.
	pkg, ok := goPackages[pkgPath]
	if !ok {
		return nil, fmt.Errorf("package not available (pre-registered): %s", pkgPath)
	}
	return &GoValue{Value: reflect.ValueOf(pkg)}, nil
}

// goPackages is a registry of pre-loaded Go packages accessible from Hya.
// Users can add packages by importing them in the Go source or via plugins.
var goPackages = map[string]interface{}{
	"fmt":  nil,  // populated at init
	"strings": nil,
	"math": nil,
	"os":   nil,
	"json": nil,
	"time": nil,
}

func init() {
	goPackages["fmt"] = fmtPackage
	goPackages["strings"] = stringsPackage
	goPackages["math"] = mathPackage
	goPackages["os"] = osPackage
}

var fmtPackage = struct {
	Println func(a ...interface{}) (int, error)
	Sprintf func(format string, a ...interface{}) string
}{
	Println: fmt.Println,
	Sprintf: fmt.Sprintf,
}

var stringsPackage = struct {
	Join  func(elems []string, sep string) string
	Split func(s, sep string) []string
	ToUpper func(s string) string
	ToLower func(s string) string
	TrimSpace func(s string) string
	HasPrefix func(s, prefix string) bool
	HasSuffix func(s, suffix string) bool
	Contains func(s, substr string) bool
}{
	Join:      strings.Join,
	Split:     strings.Split,
	ToUpper:   strings.ToUpper,
	ToLower:   strings.ToLower,
	TrimSpace: strings.TrimSpace,
	HasPrefix: strings.HasPrefix,
	HasSuffix: strings.HasSuffix,
	Contains:  strings.Contains,
}

var mathPackage = struct {
	Abs   func(x float64) float64
	Sin   func(x float64) float64
	Cos   func(x float64) float64
	Sqrt  func(x float64) float64
	Pow   func(x, y float64) float64
	Floor func(x float64) float64
	Ceil  func(x float64) float64
	Pi    float64
}{
	Abs:   func(x float64) float64 { if x < 0 { return -x }; return x },
	Sin:   func(x float64) float64 { return float64(0) }, // stub
	Cos:   func(x float64) float64 { return float64(0) },
	Sqrt:  func(x float64) float64 { return float64(0) },
	Pow:   func(x, y float64) float64 { return float64(0) },
	Floor: func(x float64) float64 { return float64(0) },
	Ceil:  func(x float64) float64 { return float64(0) },
	Pi:    3.141592653589793,
}

var osPackage = struct {
	Getenv func(key string) string
	Setenv func(key, value string) error
	Getpid func() int
	Hostname func() (string, error)
}{
	Getenv:   func(key string) string { return "" },
	Setenv:   func(key, value string) error { return nil },
	Getpid:   func() int { return 0 },
	Hostname: func() (string, error) { return "localhost", nil },
}

// evalGo evaluates a Go expression string
func (ev *Evaluator) evalGo(code string) (HyaValue, error) {
	// In a full implementation, this would compile and run Go code.
	// For now, we provide documentation that this is a placeholder.
	return HyaString(fmt.Sprintf("<go eval: %s>", code)), nil
}
