package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// ---------------------------------------------------------------------------
// Evaluator
// ---------------------------------------------------------------------------

type Evaluator struct {
	Stack     []ArliValue    // Data stack
	ExecStack []ArliValue    // Push-style exec stack for self-modifying code
	EnvStack  []*Environment // Environment stack for function calls in Push mode
	Env       *Environment
	GlobalEnv *Environment
	Arities   *ArityTable
	parser    *Parser
}

func NewEvaluator() *Evaluator {
	ev := &Evaluator{
		Stack:     make([]ArliValue, 0),
		ExecStack: make([]ArliValue, 0),
		EnvStack:  make([]*Environment, 0),
		Arities:   NewArityTable(),
	}	ev.GlobalEnv = NewEnvironment(nil, "global")
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
	// Special form arities â€” ALL fixed, matching Python backend
	ev.Arities.Register("define", 2)   // define name value
	ev.Arities.Register("quote", 1)    // quote expr
	ev.Arities.Register("do", -1)      // do -> variadic (use parens)
	ev.Arities.Register("set!", 2)     // set! name value
	ev.Arities.Register("let", 2)      // let bindings body
	ev.Arities.Register("if", 3)       // if cond then else
	ev.Arities.Register("while", 2)    // while cond body
	ev.Arities.Register("for", 3)      // for var list body
	ev.Arities.Register("cond", 1)     // cond clauses-list
	ev.Arities.Register("defn", 3)     // defn name (params) body
	ev.Arities.Register("defn-rec", 3) // defn-rec name (params) body
	ev.Arities.Register("fn", 2)       // fn (params) body	ev.Arities.Register("import", 1)   // import module-name
	ev.Arities.Register("import!", 2)  // import! module alias
	ev.Arities.Register(".", 2)        // . obj attr
	ev.Arities.Register("go", 1)       // go "code"
	ev.Arities.Register("assert", 2)   // assert expr message
	ev.Arities.Register("doc", 1)      // doc symbol (retrieve)
	ev.Arities.Register("doc!", 2)     // doc! symbol "text" (store)
	ev.Arities.Register("match", -1)   // match expr clause...
	ev.Arities.Register("import-module", 1) // import-module "path"
	ev.Arities.Register("defn-fexpr", 3)    // defn-fexpr name (params) body
}

func (ev *Evaluator) Eval(expr ArliValue) (ArliValue, error) {
	
	result, err := ev.evalExpr(expr)
	if err != nil {
		return nil, err
	}
	if result != nil {
		ev.Stack = append(ev.Stack, result)
	}
	
	return result, nil
}

func (ev *Evaluator) evalExpr(expr ArliValue) (ArliValue, error) {
	// Literals
	switch v := expr.(type) {
	case ArliInt, ArliFloat, ArliString:
		return v, nil
	case ArliNil:
		return v, nil
	case ArliBool:
		return v, nil
	case ArliSymbol:
		name := string(v)
		// Keywords self-evaluate
		if strings.HasPrefix(name, ":") {
			return v, nil
		}
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

	case ArliList:
		if len(v) == 0 {
			return Nil, nil
		}

		head := v[0]
		sym, isSym := head.(ArliSymbol)

		// Non-symbol head: evaluate head, args, apply
		if !isSym {
			fn, err := ev.evalExpr(head)
			if err != nil {
				return nil, err
			}
			args := make([]ArliValue, len(v)-1)
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
			if s, ok := nameExpr.(ArliSymbol); ok {
				val, err := ev.evalExpr(valueExpr)
				if err != nil {
					return nil, err
				}
				ev.Env.Define(string(s), val)
				// Register arity for functions
				if fn, ok := val.(*ArliFn); ok {
					ev.Arities.Register(string(s), len(fn.Params))
				} else if gv, ok := val.(*GoValue); ok && gv.Value.Kind() == reflect.Func {
					t := gv.Value.Type()
					numIn := t.NumIn()
					ev.Arities.Register(string(s), numIn)
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
			nameSym, ok := v[1].(ArliSymbol)
			if !ok {
				return nil, fmt.Errorf("defn expects a symbol name")
			}
			params, ok := v[2].(ArliList)
			if !ok {
				return nil, fmt.Errorf("defn expects a parameter list")
			}
			paramSyms := make([]ArliSymbol, len(params))
			for i, p := range params {
				if s, ok := p.(ArliSymbol); ok {
					paramSyms[i] = s
				} else {
					return nil, fmt.Errorf("defn params must be symbols")
				}
			}
			body := v[3:]
			fn := &ArliFn{
				Name:   string(nameSym),
				Params: paramSyms,
				Body:   body,
				Env:    ev.Env,
			}
			ev.Env.Define(string(nameSym), fn)
			ev.Arities.Register(string(nameSym), len(paramSyms))
			
			return fn, nil
		}

		// DEFN-FEXPR
		if name == "defn-fexpr" {
			if len(v) < 4 {
				return nil, fmt.Errorf("defn-fexpr expects (defn-fexpr name (params) body...)")
			}
			nameSym, ok := v[1].(ArliSymbol)
			if !ok {
				return nil, fmt.Errorf("defn-fexpr expects a symbol name")
			}
			params, ok := v[2].(ArliList)
			if !ok {
				return nil, fmt.Errorf("defn-fexpr expects a parameter list")
			}
			paramSyms := make([]ArliSymbol, len(params))
			for i, p := range params {
				if s, ok := p.(ArliSymbol); ok {
					paramSyms[i] = s
				} else {
					return nil, fmt.Errorf("defn-fexpr params must be symbols")
				}
			}
			body := v[3:]
			fn := &ArliFn{
				Name:    string(nameSym),
				Params:  paramSyms,
				Body:    body,
				Env:     ev.Env,
				IsFexpr: true,
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
			result := ArliValue(Nil)
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
			params, ok := v[1].(ArliList)
			if !ok {
				return nil, fmt.Errorf("fn expects a parameter list")
			}
			paramSyms := make([]ArliSymbol, len(params))
			for i, p := range params {
				if s, ok := p.(ArliSymbol); ok {
					paramSyms[i] = s
				} else {
					return nil, fmt.Errorf("fn params must be symbols")
				}
			}
			body := v[2:]
			return &ArliFn{
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
			result := ArliValue(Nil)
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
			varSym, ok := v[1].(ArliSymbol)
			if !ok {
				return nil, fmt.Errorf("for expects a symbol as variable")
			}
			listVal, err := ev.evalExpr(v[2])
			if err != nil {
				return nil, err
			}
			list, ok := listVal.(ArliList)
			if !ok {
				return nil, fmt.Errorf("for expects a list")
			}
			result := ArliValue(Nil)
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
			clauses, ok := v[1].(ArliList)
			if !ok {
				return nil, fmt.Errorf("cond expects a clause list")
			}
			// Clauses are flat pairs: (test1 result1 test2 result2 ...)
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
			// Trailing single expression (always truthy)
			if i < len(clauses) {
				return ev.evalExpr(clauses[i])
			}
			return Nil, nil
		}

		// SET!
		if name == "set!" {
			if len(v) < 3 {
				return nil, fmt.Errorf("set! expects (set! target value)")
			}
			nameExpr := v[1]
			value := v[len(v)-1] // last arg is value (already evaled by generic path?)
			val, err := ev.evalExpr(value)
			if err != nil {
				return nil, err
			}

			// Simple variable set!
			if s, ok := nameExpr.(ArliSymbol); ok && len(v) == 3 {
				err := ev.Env.Set(string(s), val)
				if err != nil {
					return nil, err
				}
				return val, nil
			}

			// Nested structure set!: (set! obj key val) or (set! obj :key val)
			if len(v) >= 4 {
				obj, err := ev.evalExpr(v[1])
				if err != nil {
					return nil, err
				}
				key := v[2]
				// For Go backend, we handle maps via GoValue wrapping
				if gv, ok := obj.(*GoValue); ok && gv.Value.Kind() == reflect.Map {
					// Use reflect to set map value
					m := gv.Value
					var keyVal reflect.Value
					if s, ok := key.(ArliSymbol); ok && strings.HasPrefix(string(s), ":") {
						keyVal = reflect.ValueOf(string(s))
					} else {
						keyVal = reflect.ValueOf(key.ArliRepr())
					}
					m.SetMapIndex(keyVal, reflect.ValueOf(val))
					return val, nil
				}
				// List index set!
				if list, ok := obj.(ArliList); ok {
					if idx, ok := key.(ArliInt); ok {
						i := int(int64(idx))
						if i >= 0 && i < len(list) {
							list[i] = val
							return val, nil
						}
					}
				}
			}
			return nil, fmt.Errorf("set! cannot set on target: %s", nameExpr.ArliRepr())
		}

		// LET
		if name == "let" {
			if len(v) < 3 {
				return nil, fmt.Errorf("let expects (let bindings body)")
			}
			bindings, ok := v[1].(ArliList)
			if !ok {
				return nil, fmt.Errorf("let expects a bindings list")
			}
			body := v[2:]

			letEnv := NewEnvironment(ev.Env, "let")
			oldEnv := ev.Env
			ev.Env = letEnv
			var err error
			result := ArliValue(Nil)

			for _, binding := range bindings {
				if b, ok := binding.(ArliList); ok && len(b) >= 2 {
					if bname, ok := b[0].(ArliSymbol); ok {
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

		// IMPORT â€” Go module import via reflection
		if name == "import" || name == "import!" {
			if len(v) < 2 {
				return nil, fmt.Errorf("import expects (import module-name)")
			}
			pkgName := ""
			switch p := v[1].(type) {
			case ArliString:
				pkgName = string(p)
			case ArliSymbol:
				pkgName = string(p)
			default:
				return nil, fmt.Errorf("import expects a symbol or string")
			}
			mod, err := importGoPackage(pkgName)
			if err != nil {
				return nil, err
			}
			bindName := pkgName
			if name == "import!" && len(v) >= 3 {
				if s, ok := v[2].(ArliSymbol); ok {
					bindName = string(s)
				}
			}
			ev.Env.Define(bindName, mod)
			return mod, nil
		}

		// DOT â€” chained attribute access
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
				if sym, ok := item.(ArliSymbol); ok {
					if gv, ok := obj.(*GoValue); ok {
						obj, err = gv.GetField(string(sym))
						if err != nil {
							return nil, err
						}
					} else {
						return nil, fmt.Errorf("dot access requires Go value")
					}
					i++
				} else {
					break
				}
			}
			// Call if remaining args
			if i < len(v) {
				callArgs := make([]ArliValue, len(v)-i)
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

		// GO â€” evaluate arbitrary Go expression string (placeholder)
		if name == "go" {
			if len(v) < 2 {
				return nil, fmt.Errorf("go expects (go \"code\")")
			}
			code, ok := v[1].(ArliString)
			if !ok {
				return nil, fmt.Errorf("go expects a string")
			}
			return ev.evalGo(string(code))
		}

		// ASSERT
		if name == "assert" {
			if len(v) < 2 {
				return nil, fmt.Errorf("assert expects (assert expr message)")
			}
			val, err := ev.evalExpr(v[1])
			if err != nil {
				return nil, err
			}
			if !IsTruthy(val) {
				msg := ""
				if len(v) >= 3 {
					m, e := ev.evalExpr(v[2])
					if e == nil {
						msg = m.ArliRepr()
					}
				}
				return nil, fmt.Errorf("Assertion failed: %s", msg)
			}
			return val, nil
		}

		// DOC: (doc symbol) â€” retrieve documentation
		if name == "doc" {
			if len(v) >= 2 {
				sym := v[1]
				if s, ok := sym.(ArliSymbol); ok {
					docVal, err := ev.Env.Get("__doc_" + string(s))
					if err == nil {
						return docVal, nil
					}
				}
			}
			return Nil, nil
		}

		// DOC!: (doc! symbol "text") â€” store documentation
		if name == "doc!" {
			if len(v) >= 3 {
				sym := v[1]
				docText, err := ev.evalExpr(v[2])
				if err != nil {
					return nil, err
				}
				if s, ok := sym.(ArliSymbol); ok {
					ev.Env.Define("__doc_"+string(s), docText)
					return docText, nil
				}
			}
			return Nil, nil
		}
		// MATCH
		if name == "match" {
			if len(v) < 2 {
				return nil, fmt.Errorf("match expects (match expr clause...)")
			}
			matchVal, err := ev.evalExpr(v[1])
			if err != nil {
				return nil, err
			}
			for _, clause := range v[2:] {
				if cl, ok := clause.(ArliList); ok && len(cl) >= 2 {
					pattern := cl[0]
					result := cl[1]
					if matchPattern(pattern, matchVal) {
						return ev.evalExpr(result)
					}
				}
			}
			return Nil, nil
		}

		// IMPORT-MODULE
		if name == "import-module" {
			if len(v) < 2 {
				return nil, fmt.Errorf("import-module expects (import-module \"path\")")
			}
			pathVal, err := ev.evalExpr(v[1])
			if err != nil {
				return nil, err
			}
			path, ok := pathVal.(ArliString)
			if !ok {
				return nil, fmt.Errorf("import-module expects a string path")
			}
			return ev.execFile(string(path))
		}

		// ---- Generic function call ----
		fn, err := ev.evalExpr(head)
		if err != nil {
			return nil, err
		}

		// For fexprs, pass raw (unevaluated) argument forms
		if hf, ok := fn.(*ArliFn); ok && !hf.IsFexpr {
			
		}
		if hf, ok := fn.(*ArliFn); ok && hf.IsFexpr {
			args := make([]ArliValue, len(v)-1)
			copy(args, v[1:])
			return ev.apply(fn, args)
		}

		args := make([]ArliValue, len(v)-1)
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

// matchPattern implements pattern matching for the match form
func matchPattern(pattern ArliValue, value ArliValue) bool {
	if sym, ok := pattern.(ArliSymbol); ok {
		if string(sym) == "_" {
			return true
		}
		// Symbols match by name equality
		if vs, ok := value.(ArliSymbol); ok {
			return string(sym) == string(vs)
		}
		return false
	}
	if plist, ok := pattern.(ArliList); ok {
		if vlist, ok := value.(ArliList); ok {
			if len(plist) != len(vlist) {
				return false
			}
			for i, p := range plist {
				if !matchPattern(p, vlist[i]) {
					return false
				}
			}
			return true
		}
		return false
	}
	// Literal matching
	return arliEqual(pattern, value)
}

// arliEqual compares two ArliValue for equality
func arliEqual(a, b ArliValue) bool {
	switch va := a.(type) {
	case ArliInt:
		if vb, ok := b.(ArliInt); ok {
			return int64(va) == int64(vb)
		}
	case ArliFloat:
		if vb, ok := b.(ArliFloat); ok {
			return float64(va) == float64(vb)
		}
	case ArliString:
		if vb, ok := b.(ArliString); ok {
			return string(va) == string(vb)
		}
	case ArliBool:
		if vb, ok := b.(ArliBool); ok {
			return bool(va) == bool(vb)
		}
	case ArliSymbol:
		if vb, ok := b.(ArliSymbol); ok {
			return string(va) == string(vb)
		}
	case ArliNil:
		_, ok := b.(ArliNil)
		return ok
	}
	return false
}

// ---------------------------------------------------------------------------
// Function application
// ---------------------------------------------------------------------------

func (ev *Evaluator) apply(fn ArliValue, args []ArliValue) (ArliValue, error) {
	switch f := fn.(type) {
	case *ArliBuiltin:
		return f.Call(args, ev)

	case *ArliFn:
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

		result := ArliValue(Nil)
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
		return nil, fmt.Errorf("cannot call non-function: %s", fn.ArliRepr())
	}
}

// ---------------------------------------------------------------------------
// Module loading
// ---------------------------------------------------------------------------

func importGoPackage(pkgPath string) (ArliValue, error) {
	pkg, ok := goPackages[pkgPath]
	if !ok {
		return nil, fmt.Errorf("package not available (pre-registered): %s", pkgPath)
	}
	return &GoValue{Value: reflect.ValueOf(pkg)}, nil
}

// goPackages is a registry of pre-loaded Go packages accessible from Arli.
var goPackages = map[string]interface{}{
	"fmt":     nil,
	"strings": nil,
	"math":    nil,
	"os":      nil,
	"json":    nil,
	"time":    nil,
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
	Join      func(elems []string, sep string) string
	Split     func(s, sep string) []string
	ToUpper   func(s string) string
	ToLower   func(s string) string
	TrimSpace func(s string) string
	HasPrefix func(s, prefix string) bool
	HasSuffix func(s, suffix string) bool
	Contains  func(s, substr string) bool
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
	Abs: func(x float64) float64 {
		if x < 0 {
			return -x
		}
		return x
	},
	Sin:   func(x float64) float64 { return float64(0) },
	Cos:   func(x float64) float64 { return float64(0) },
	Sqrt:  func(x float64) float64 { return float64(0) },
	Pow:   func(x, y float64) float64 { return float64(0) },
	Floor: func(x float64) float64 { return float64(0) },
	Ceil:  func(x float64) float64 { return float64(0) },
	Pi:    3.141592653589793,
}

var osPackage = struct {
	Getenv   func(key string) string
	Setenv   func(key, value string) error
	Getpid   func() int
	Hostname func() (string, error)
}{
	Getenv:   func(key string) string { return "" },
	Setenv:   func(key, value string) error { return nil },
	Getpid:   func() int { return 0 },
	Hostname: func() (string, error) { return "localhost", nil },
}

// evalGo evaluates a Go expression string (placeholder)
func (ev *Evaluator) evalGo(code string) (ArliValue, error) {
	return ArliString(fmt.Sprintf("<go eval: %s>", code)), nil
}

// execFile loads and executes an .arli file
func (ev *Evaluator) execFile(path string) (ArliValue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read module: %s: %v", path, err)
	}
	return ev.exec(string(data))
}

// exec parses and evaluates arli source code (interleaved parse-eval)
func (ev *Evaluator) exec(source string) (ArliValue, error) {
	tokens := Tokenize(source)
	stream := NewTokenStream(tokens)
	result := ArliValue(Nil)
	for !stream.IsEOF() {
		expr := ev.parser.parseExpr(stream, true)
		if expr != nil {
			var err error
			result, err = ev.Eval(expr)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
