package main

import "fmt"

// ---------------------------------------------------------------------------
// Environment â€” lexical scoping with parent chain
// ---------------------------------------------------------------------------

type Environment struct {
	parent  *Environment
	name    string
	bindings map[string]ArliValue
}

func NewEnvironment(parent *Environment, name string) *Environment {
	return &Environment{
		parent:   parent,
		name:     name,
		bindings: make(map[string]ArliValue),
	}
}

func (env *Environment) Define(name string, value ArliValue) ArliValue {
	env.bindings[name] = value
	return value
}

func (env *Environment) Lookup(name string) (ArliValue, bool) {
	if v, ok := env.bindings[name]; ok {
		return v, true
	}
	if env.parent != nil {
		return env.parent.Lookup(name)
	}
	return nil, false
}

func (env *Environment) Get(name string) (ArliValue, error) {
	v, ok := env.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("undefined symbol: %s", name)
	}
	return v, nil
}

func (env *Environment) Set(name string, value ArliValue) error {
	if _, ok := env.bindings[name]; ok {
		env.bindings[name] = value
		return nil
	}
	if env.parent != nil {
		return env.parent.Set(name, value)
	}
	return fmt.Errorf("cannot set! undefined symbol: %s", name)
}
