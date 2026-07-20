package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// HyaValue interface — every value in Hya implements this
// ---------------------------------------------------------------------------

type HyaValue interface {
	HyaRepr() string
}

// ---------------------------------------------------------------------------
// Primitive types
// ---------------------------------------------------------------------------

type HyaInt int64

func (v HyaInt) HyaRepr() string { return strconv.FormatInt(int64(v), 10) }

type HyaFloat float64

func (v HyaFloat) HyaRepr() string {
	s := strconv.FormatFloat(float64(v), 'g', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

type HyaString string

func (v HyaString) HyaRepr() string { return fmt.Sprintf("%q", string(v)) }

type HyaSymbol string

func (v HyaSymbol) HyaRepr() string { return string(v) }

// ---------------------------------------------------------------------------
// Nil — singleton false value
// ---------------------------------------------------------------------------

type HyaNil struct{}

func (HyaNil) HyaRepr() string { return "nil" }

var Nil = HyaNil{}

// Helper: convert []HyaValue from a parameter list into HyaList of symbols
func symbolsFromList(list HyaList) HyaList {
	syms := make(HyaList, len(list))
	for i, p := range list {
		if sym, ok := p.(HyaSymbol); ok {
			syms[i] = sym
		} else {
			panic("params must be symbols")
		}
	}
	return syms
}

func IsTruthy(v HyaValue) bool {
	_, isnil := v.(HyaNil)
	_, isfalse := v.(HyaBool)
	return !isnil && !(isfalse && v.(HyaBool) == False)
}

// ---------------------------------------------------------------------------
// Bool
// ---------------------------------------------------------------------------

type HyaBool bool

const True = HyaBool(true)
const False = HyaBool(false)

func (v HyaBool) HyaRepr() string {
	if v {
		return "true"
	}
	return "false"
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

type HyaList []HyaValue

func (l HyaList) HyaRepr() string {
	if len(l) == 0 {
		return "()"
	}
	parts := make([]string, len(l))
	for i, v := range l {
		parts[i] = v.HyaRepr()
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// ---------------------------------------------------------------------------
// Builtin — Go function wrapped with arity
// ---------------------------------------------------------------------------

type HyaBuiltin struct {
	Name  string
	Fn    func(args []HyaValue, ev *Evaluator) (HyaValue, error)
	Arity int // -1 = variadic
}

func (b *HyaBuiltin) HyaRepr() string { return fmt.Sprintf("<builtin %s arity=%d>", b.Name, b.Arity) }

func (b *HyaBuiltin) Call(args []HyaValue, ev *Evaluator) (HyaValue, error) {
	return b.Fn(args, ev)
}

// ---------------------------------------------------------------------------
// Function — user-defined closure
// ---------------------------------------------------------------------------

type HyaFn struct {
	Name   string
	Params []HyaSymbol
	Body   []HyaValue
	Env    *Environment
}

func (f *HyaFn) HyaRepr() string {
	n := f.Name
	if n == "" {
		n = "anon"
	}
	return fmt.Sprintf("<fn %s>", n)
}

// ---------------------------------------------------------------------------
// GoValue — wraps a Go value for interop
// ---------------------------------------------------------------------------

type GoValue struct {
	Value reflect.Value
}

func (g *GoValue) HyaRepr() string {
	return fmt.Sprintf("<Go %s>", g.Value.Type().String())
}

func (g *GoValue) GetField(name string) (HyaValue, error) {
	v := g.Value
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName(name)
		if !f.IsValid() {
			return nil, fmt.Errorf("no field '%s' on %s", name, v.Type())
		}
		return reflectToHya(f), nil
	}
	// Method
	m := g.Value.MethodByName(name)
	if !m.IsValid() {
		return nil, fmt.Errorf("no method/field '%s' on %s", name, g.Value.Type())
	}
	return &GoValue{Value: m}, nil
}

func (g *GoValue) Call(args []HyaValue) (HyaValue, error) {
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
			goArgs = append(goArgs, hyaToReflect(args[i], t.In(i)))
		}

		// Pack remaining args into variadic slice
		varSlice := reflect.MakeSlice(t.In(numFixed), 0, len(args)-numFixed)
		for i := numFixed; i < len(args); i++ {
			varSlice = reflect.Append(varSlice, hyaToReflect(args[i], t.In(numFixed).Elem()))
		}
		goArgs = append(goArgs, varSlice)

		results := v.Call(goArgs)
		if len(results) == 0 {
			return Nil, nil
		}
		return reflectToHya(results[0]), nil
	}

	// Non-variadic
	goArgs := make([]reflect.Value, len(args))
	for i, a := range args {
		goArgs[i] = hyaToReflect(a, t.In(i))
	}
	results := v.Call(goArgs)
	if len(results) == 0 {
		return Nil, nil
	}
	return reflectToHya(results[0]), nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func hyaToReflect(v HyaValue, t reflect.Type) reflect.Value {
	switch val := v.(type) {
	case HyaInt:
		return reflect.ValueOf(int64(val)).Convert(t)
	case HyaFloat:
		return reflect.ValueOf(float64(val)).Convert(t)
	case HyaString:
		return reflect.ValueOf(string(val)).Convert(t)
	case HyaBool:
		return reflect.ValueOf(bool(val)).Convert(t)
	case HyaNil:
		return reflect.Zero(t)
	case *GoValue:
		return val.Value
	default:
		return reflect.ValueOf(v)
	}
}

func reflectToHya(v reflect.Value) HyaValue {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return HyaInt(v.Int())
	case reflect.Float32, reflect.Float64:
		return HyaFloat(v.Float())
	case reflect.String:
		return HyaString(v.String())
	case reflect.Bool:
		if v.Bool() {
			return True
		}
		return False
	case reflect.Slice, reflect.Array:
		l := make(HyaList, v.Len())
		for i := 0; i < v.Len(); i++ {
			l[i] = reflectToHya(v.Index(i))
		}
		return l
	case reflect.Map:
		// Return as GoValue for now
		return &GoValue{Value: v}
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return Nil
		}
		return reflectToHya(v.Elem())
	case reflect.Func:
		return &GoValue{Value: v}
	case reflect.Struct:
		return &GoValue{Value: v}
	default:
		return &GoValue{Value: v}
	}
}
