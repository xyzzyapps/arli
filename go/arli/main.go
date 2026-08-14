package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "-e" && len(os.Args) > 2 {
			runString(os.Args[2])
			return
		}
		// Run file
		runFile(os.Args[1])
		return
	}

	// REPL
	repl()
}

func runString(source string) {
	ev := NewEvaluator()
	tokens := Tokenize(source)
	stream := NewTokenStream(tokens)
	for !stream.IsEOF() {
		expr := ev.parser.parseExpr(stream, true)
		if expr != nil {
			result, err := ev.Eval(expr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if result != nil {
				fmt.Println(result.ArliRepr())
			}
		}
	}
}

func runFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	ev := NewEvaluator()
	source := string(data)

	// Parse and evaluate one expression at a time (interleaved)
	tokens := Tokenize(source)
	stream := NewTokenStream(tokens)
	for !stream.IsEOF() {
		expr := ev.parser.parseExpr(stream, true)
		if expr != nil {
			result, err := ev.Eval(expr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if result != nil {
				fmt.Println(result.ArliRepr())
			}
		}
	}
}

func repl() {
	ev := NewEvaluator()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Arli Go v0.1.0")
	fmt.Println("Arity-driven Lisp with Forth-like stack operations")
	fmt.Println("Type 'exit' or Ctrl+C to quit")
	fmt.Println()

	for {
		fmt.Print("arli> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		if line == "/stack" {
			if len(ev.Stack) == 0 {
				fmt.Println("Stack is empty.")
			} else {
				fmt.Printf("Stack (%d items):\n", len(ev.Stack))
				for i, v := range ev.Stack {
					fmt.Printf("  %d: %s\n", i, v.ArliRepr())
				}
			}
			continue
		}
		if line == "/env" {
			printEnv(ev.Env, 0)
			continue
		}
		if line == "/arity" {
			fmt.Println("Registered arities:")
			for name, arity := range ev.Arities.table {
				label := fmt.Sprintf("%d", arity)
				if arity < 0 {
					label = "variadic"
				}
				fmt.Printf("  %s: %s\n", name, label)
			}
			continue
		}
		if line == "/debug" {
			fmt.Println("Debug mode not yet implemented in Go backend")
			continue
		}
		if line == "/clear" {
			ev.Stack = ev.Stack[:0]
			fmt.Println("Stack cleared.")
			continue
		}
		if line == "/reset" {
			ev = NewEvaluator()
			fmt.Println("Evaluator reset.")
			continue
		}

		// Evaluate
		// Support multi-line: keep reading if parens are unclosed
		text := line
		opens := strings.Count(text, "(")
		closes := strings.Count(text, ")")
		for opens > closes {
			fmt.Print(".. ")
			if !scanner.Scan() {
				break
			}
			more := scanner.Text()
			text += "\n" + more
			opens = strings.Count(text, "(")
			closes = strings.Count(text, ")")
		}

		// Parse and evaluate
		tokens := Tokenize(text)
		stream := NewTokenStream(tokens)
		anyError := false
		for !stream.IsEOF() {
			expr := ev.parser.parseExpr(stream, true)
			if expr != nil {
				result, err := ev.Eval(expr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					anyError = true
					break
				}
				if result != nil {
					fmt.Println(result.ArliRepr())
				}
			}
		}
		if anyError {
			continue
		}
	}
}

func printEnv(env *Environment, depth int) {
	prefix := strings.Repeat("  ", depth)
	for k, v := range env.bindings {
		fmt.Printf("%s  %s: %s\n", prefix, k, v.ArliRepr())
	}
	if env.parent != nil {
		fmt.Printf("%s(parent)\n", prefix)
		printEnv(env.parent, depth+1)
	}
}
