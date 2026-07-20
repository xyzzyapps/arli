package main

import "fmt"

// ---------------------------------------------------------------------------
// Environment — lexical scoping with parent chain
// ---------------------------------------------------------------------------

type Environment struct {
	parent  *Environment
	name    string
	bindings map[string]HyaValue
}

func NewEnvironment(parent *Environment, name string) *Environment {
	return &Environment{
		parent:   parent,
		name:     name,
		bindings: make(map[string]HyaValue),
	}
}

func (env *Environment) Define(name string, value HyaValue) HyaValue {
	env.bindings[name] = value
	return value
}

func (env *Environment) Lookup(name string) (HyaValue, bool) {
	if v, ok := env.bindings[name]; ok {
		return v, true
	}
	if env.parent != nil {
		return env.parent.Lookup(name)
	}
	return nil, false
}

func (env *Environment) Get(name string) (HyaValue, error) {
	v, ok := env.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("undefined symbol: %s", name)
	}
	return v, nil
}

func (env *Environment) Set(name string, value HyaValue) error {
	if _, ok := env.bindings[name]; ok {
		env.bindings[name] = value
		return nil
	}
	if env.parent != nil {
		return env.parent.Set(name, value)
	}
	return fmt.Errorf("cannot set! undefined symbol: %s", name)
}
