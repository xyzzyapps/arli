package main

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// ArliValue interface â€” every value in Arli implements this
// ---------------------------------------------------------------------------

type ArliValue interface {
	ArliRepr() string
}

// ---------------------------------------------------------------------------
// Primitive types
// ---------------------------------------------------------------------------

type ArliInt int64

func (v ArliInt) ArliRepr() string { return strconv.FormatInt(int64(v), 10) }

type ArliFloat float64

func (v ArliFloat) ArliRepr() string {
	s := strconv.FormatFloat(float64(v), 'g', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

type ArliString string

func (v ArliString) ArliRepr() string { return fmt.Sprintf("%q", string(v)) }

type ArliSymbol string

func (v ArliSymbol) ArliRepr() string { return string(v) }

// ---------------------------------------------------------------------------
// Nil â€” singleton false value
// ---------------------------------------------------------------------------

type ArliNil struct{}

func (ArliNil) ArliRepr() string { return "nil" }

var Nil = ArliNil{}

// Helper: convert []ArliValue from a parameter list into ArliList of symbols
func symbolsFromList(list ArliList) ArliList {
	syms := make(ArliList, len(list))
	for i, p := range list {
		if sym, ok := p.(ArliSymbol); ok {
			syms[i] = sym
		} else {
			panic("params must be symbols")
		}
	}
	return syms
}

func IsTruthy(v ArliValue) bool {
	_, isnil := v.(ArliNil)
	_, isfalse := v.(ArliBool)
	return !isnil && !(isfalse && v.(ArliBool) == False)
}

// ---------------------------------------------------------------------------
// Bool
// ---------------------------------------------------------------------------

type ArliBool bool

const True = ArliBool(true)
const False = ArliBool(false)

func (v ArliBool) ArliRepr() string {
	if v {
		return "true"
	}
	return "false"
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

type ArliList []ArliValue

func (l ArliList) ArliRepr() string {
	if len(l) == 0 {
		return "()"
	}
	parts := make([]string, len(l))
	for i, v := range l {
		parts[i] = v.ArliRepr()
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// ---------------------------------------------------------------------------
// Builtin â€” Go function wrapped with arity
// ---------------------------------------------------------------------------

type ArliBuiltin struct {
	Name  string
	Fn    func(args []ArliValue, ev *Evaluator) (ArliValue, error)
	Arity int // -1 = variadic
}

func (b *ArliBuiltin) ArliRepr() string { return fmt.Sprintf("<builtin %s arity=%d>", b.Name, b.Arity) }

func (b *ArliBuiltin) Call(args []ArliValue, ev *Evaluator) (ArliValue, error) {
	return b.Fn(args, ev)
}

// ---------------------------------------------------------------------------
// Function â€” user-defined closure
// ---------------------------------------------------------------------------

type ArliFn struct {
	Name     string
	Params   []ArliSymbol
	Body     []ArliValue
	Env      *Environment
	IsFexpr  bool // f-expressions don't evaluate arguments
}

func (f *ArliFn) ArliRepr() string {
	kind := "fn"
	if f.IsFexpr {
		kind = "fexpr"
	}
	n := f.Name
	if n == "" {
		n = "anon"
	}
	return fmt.Sprintf("<%s %s>", kind, n)
}

// ---------------------------------------------------------------------------
// GoValue â€” wraps a Go value for interop
// ---------------------------------------------------------------------------

type GoValue struct {
	Value reflect.Value
}

func (g *GoValue) ArliRepr() string {
	v := g.Value
	// Display maps in Python-style dict format
	if v.Kind() == reflect.Map {
		keys := v.MapKeys()
		// Sort keys for deterministic output matching Python's insertion order
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprintf("%v", keys[i].Interface()) < fmt.Sprintf("%v", keys[j].Interface())
		})
		var parts []string
		for _, k := range keys {
			kv := k.Interface()
			keyStr := ""
			switch s := kv.(type) {
			case string:
				keyStr = fmt.Sprintf("'%s'", s) // single quotes like Python dict repr
			default:
				keyStr = fmt.Sprintf("%v", kv)
			}
			valStr := reflectToArli(v.MapIndex(k)).ArliRepr()
			parts = append(parts, fmt.Sprintf("%s: %s", keyStr, valStr))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprintf("<Go %s>", v.Type().String())
}

func (g *GoValue) GetField(name string) (ArliValue, error) {
	v := g.Value
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName(name)
		if !f.IsValid() {
			return nil, fmt.Errorf("no field '%s' on %s", name, v.Type())
		}
		return reflectToArli(f), nil
	}
	// Method
	m := g.Value.MethodByName(name)
	if !m.IsValid() {
		return nil, fmt.Errorf("no method/field '%s' on %s", name, g.Value.Type())
	}
	return &GoValue{Value: m}, nil
}

func (g *GoValue) Call(args []ArliValue) (ArliValue, error) {
	v := g.Value
	if v.Kind() != reflect.Func {
		return nil, fmt.Errorf("cannot call non-function Go value: %s", v.Type())
	}
	t := v.Type()

	// Handle variadic functions
	if t.IsVariadic() {
		// Fixed args + variadic slice
		numFixed := t.NumIn() - 1
		goArgs := make([]reflect.Value, 0, numFixed+1)

		for i := 0; i < numFixed && i < len(args); i++ {
			goArgs = append(goArgs, arliToReflect(args[i], t.In(i)))
		}

		// Pack remaining args into variadic slice
		varSlice := reflect.MakeSlice(t.In(numFixed), 0, len(args)-numFixed)
		for i := numFixed; i < len(args); i++ {
			varSlice = reflect.Append(varSlice, arliToReflect(args[i], t.In(numFixed).Elem()))
		}
		goArgs = append(goArgs, varSlice)

		results := v.Call(goArgs)
		if len(results) == 0 {
			return Nil, nil
		}
		return reflectToArli(results[0]), nil
	}

	// Non-variadic
	goArgs := make([]reflect.Value, len(args))
	for i, a := range args {
		goArgs[i] = arliToReflect(a, t.In(i))
	}
	results := v.Call(goArgs)
	if len(results) == 0 {
		return Nil, nil
	}
	return reflectToArli(results[0]), nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func arliToReflect(v ArliValue, t reflect.Type) reflect.Value {
	switch val := v.(type) {
	case ArliInt:
		return reflect.ValueOf(int64(val)).Convert(t)
	case ArliFloat:
		return reflect.ValueOf(float64(val)).Convert(t)
	case ArliString:
		return reflect.ValueOf(string(val)).Convert(t)
	case ArliBool:
		return reflect.ValueOf(bool(val)).Convert(t)
	case ArliNil:
		return reflect.Zero(t)
	case *GoValue:
		return val.Value
	default:
		return reflect.ValueOf(v)
	}
}

func reflectToArli(v reflect.Value) ArliValue {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ArliInt(v.Int())
	case reflect.Float32, reflect.Float64:
		return ArliFloat(v.Float())
	case reflect.String:
		return ArliString(v.String())
	case reflect.Bool:
		if v.Bool() {
			return True
		}
		return False
	case reflect.Slice, reflect.Array:
		l := make(ArliList, v.Len())
		for i := 0; i < v.Len(); i++ {
			l[i] = reflectToArli(v.Index(i))
		}
		return l
	case reflect.Map:
		// Return as GoValue for now
		return &GoValue{Value: v}
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return Nil
		}
		return reflectToArli(v.Elem())
	case reflect.Func:
		return &GoValue{Value: v}
	case reflect.Struct:
		return &GoValue{Value: v}
	default:
		return &GoValue{Value: v}
	}
}
