package main

// ---------------------------------------------------------------------------
// ArityTable — maps symbol names to their arity
// ---------------------------------------------------------------------------

type ArityTable struct {
	table map[string]int
}

func NewArityTable() *ArityTable {
	return &ArityTable{table: make(map[string]int)}
}

func (at *ArityTable) Register(name string, arity int) {
	at.table[name] = arity
}

func (at *ArityTable) Get(name string) (int, bool) {
	v, ok := at.table[name]
	return v, ok
}

// ---------------------------------------------------------------------------
// Parser — arity-driven S-expression parser
// ---------------------------------------------------------------------------

type Parser struct {
	arities *ArityTable
}

func NewParser(at *ArityTable) *Parser {
	return &Parser{arities: at}
}

func (p *Parser) Parse(source string) []ArliValue {
	tokens := Tokenize(source)
	stream := NewTokenStream(tokens)
	var exprs []ArliValue
	for !stream.IsEOF() {
		expr := p.parseExpr(stream, true)
		if expr != nil {
			exprs = append(exprs, expr)
		}
	}
	return exprs
}

func (p *Parser) parseExpr(stream *TokenStream, allowArity bool) ArliValue {
	tok := stream.Peek()

	switch tok.Type {
	case TK_EOF:
		return nil

	case TK_OPEN:
		return p.parseParenList(stream)

	case TK_VECTOR_OPEN:
		return p.parseBracketList(stream)

	case TK_MAP_OPEN:
		return p.parseMapLiteral(stream)

	case TK_QUOTE:
		stream.Next()
		expr := p.parseExpr(stream, true)
		return ArliList{ArliSymbol("quote"), expr}

	case TK_QUASIQUOTE:
		stream.Next()
		expr := p.parseExpr(stream, true)
		return ArliList{ArliSymbol("quasiquote"), expr}

	case TK_UNQUOTE:
		stream.Next()
		expr := p.parseExpr(stream, true)
		return ArliList{ArliSymbol("unquote"), expr}

	case TK_UNQUOTE_SPLICE:
		stream.Next()
		expr := p.parseExpr(stream, true)
		return ArliList{ArliSymbol("unquote-splicing"), expr}

	case TK_NUMBER:
		stream.Next()
		f := tok.Num
		if f == float64(int64(f)) {
			return ArliInt(int64(f))
		}
		return ArliFloat(f)

	case TK_STRING:
		stream.Next()
		return ArliString(tok.Str)

	case TK_KEYWORD:
		stream.Next()
		return ArliSymbol(tok.Value)

	case TK_SYMBOL:
		stream.Next()
		name := tok.Value

		// defn-rec always uses special handler (for recursion)
		if name == "defn-rec" {
			return p.parseDefnRec(stream)
		}

		if !allowArity {
			return ArliSymbol(name)
		}

		// Arity-driven consumption
		if arity, ok := p.arities.Get(name); ok && arity >= 0 {
			args := make([]ArliValue, arity)
			for i := 0; i < arity; i++ {
				arg := p.parseExpr(stream, true)
				if arg == nil {
					panic("unexpected EOF while parsing " + name)
				}
				args[i] = arg
			}
			result := make(ArliList, arity+1)
			result[0] = ArliSymbol(name)
			copy(result[1:], args)
			return result
		}

		return ArliSymbol(name)
	}

	return nil
}

func (p *Parser) parseParenList(stream *TokenStream) ArliValue {
	stream.Expect(TK_OPEN) // consume (
	var items []ArliValue
	isFirst := true

	for {
		tok := stream.Peek()
		if tok.Type == TK_CLOSE {
			stream.Next() // consume )
			break
		}
		if tok.Type == TK_EOF {
			panic("unclosed parenthesis")
		}

		// First element: no arity (operator position).
		// Subsequent: arity-driven.
		expr := p.parseExpr(stream, !isFirst)
		if expr != nil {
			items = append(items, expr)
		}
		isFirst = false
	}

	// Unwrap single-element (defn ...) forms produced by
	// parseDefnRec and (via arity-driven) top-level defn
	if len(items) == 1 {
		if list, ok := items[0].(ArliList); ok && len(list) > 0 {
			if sym, ok := list[0].(ArliSymbol); ok && string(sym) == "defn" {
				return list
			}
		}
	}

	return ArliList(items)
}

func (p *Parser) parseBracketList(stream *TokenStream) ArliValue {
	stream.Expect(TK_VECTOR_OPEN) // consume [
	var items []ArliValue
	for {
		tok := stream.Peek()
		if tok.Type == TK_VECTOR_CLOSE {
			stream.Next() // consume ]
			break
		}
		if tok.Type == TK_EOF {
			panic("unclosed bracket")
		}
		expr := p.parseExpr(stream, true)
		if expr != nil {
			items = append(items, expr)
		}
	}
	// Return (list item1 item2 ...)
	result := make(ArliList, len(items)+1)
	result[0] = ArliSymbol("list")
	copy(result[1:], items)
	return result
}

func (p *Parser) parseMapLiteral(stream *TokenStream) ArliValue {
	stream.Expect(TK_MAP_OPEN) // consume {
	var items []ArliValue
	for {
		tok := stream.Peek()
		if tok.Type == TK_MAP_CLOSE {
			stream.Next() // consume }
			break
		}
		if tok.Type == TK_EOF {
			panic("unclosed map brace")
		}
		expr := p.parseExpr(stream, true)
		if expr != nil {
			items = append(items, expr)
		}
	}
	// Return (hash-map :key1 val1 :key2 val2 ...)
	result := make(ArliList, len(items)+1)
	result[0] = ArliSymbol("hash-map")
	copy(result[1:], items)
	return result
}

// parseDefn parses (defn name (params) body...)
// Used when defn is inside parens as the first element (not arity-driven)
func (p *Parser) parseDefn(stream *TokenStream) ArliValue {
	nameTok := stream.Peek()
	if nameTok.Type != TK_SYMBOL {
		panic("defn expects a name")
	}
	name := ArliSymbol(stream.Next().Value)

	params := p.parseExpr(stream, false) // parse (x y z) with no arity on first
	paramList, ok := params.(ArliList)
	if !ok {
		panic("defn expects a parameter list")
	}
	paramSyms := symbolsFromList(paramList)

	// Register arity at parse time
	p.arities.Register(string(name), len(paramSyms))

	// Parse body (multiple expressions until close paren)
	var body []ArliValue
	for {
		tok := stream.Peek()
		if tok.Type == TK_CLOSE || tok.Type == TK_EOF {
			break
		}
		expr := p.parseExpr(stream, true)
		if expr != nil {
			body = append(body, expr)
		}
	}
	if len(body) == 0 {
		body = append(body, Nil)
	}

	result := make(ArliList, 3+len(body))
	result[0] = ArliSymbol("defn")
	result[1] = name
	result[2] = paramSyms
	copy(result[3:], body)
	return result
}

// parseFn parses (fn (params) body...)
func (p *Parser) parseFn(stream *TokenStream) ArliValue {
	params := p.parseExpr(stream, false)
	paramList, ok := params.(ArliList)
	if !ok {
		panic("fn expects a parameter list")
	}
	paramSyms := symbolsFromList(paramList)

	var body []ArliValue
	for {
		tok := stream.Peek()
		if tok.Type == TK_CLOSE || tok.Type == TK_EOF {
			break
		}
		expr := p.parseExpr(stream, true)
		if expr != nil {
			body = append(body, expr)
		}
	}
	if len(body) == 0 {
		body = append(body, Nil)
	}

	result := make(ArliList, 2+len(body))
	result[0] = ArliSymbol("fn")
	result[1] = paramSyms
	copy(result[2:], body)
	return result
}

func (p *Parser) parseDefnRec(stream *TokenStream) ArliValue {
	nameTok := stream.Peek()
	if nameTok.Type != TK_SYMBOL {
		panic("defn-rec expects a name")
	}
	name := ArliSymbol(stream.Next().Value)

	params := p.parseExpr(stream, false)
	paramList, ok := params.(ArliList)
	if !ok {
		panic("defn-rec expects a parameter list")
	}
	paramSyms := symbolsFromList(paramList)

	// Register arity BEFORE body (for recursion)
	p.arities.Register(string(name), len(paramSyms))

	// Parse ONE body expression (like defn arity 3).
	// Multiple expressions use (do ...) — consistent with defn.
	var body []ArliValue
	tok := stream.Peek()
	if tok.Type != TK_CLOSE && tok.Type != TK_EOF {
		expr := p.parseExpr(stream, true)
		if expr != nil {
			body = append(body, expr)
		}
	}
	if len(body) == 0 {
		body = append(body, Nil)
	}

	result := make(ArliList, 3+len(body))
	result[0] = ArliSymbol("defn")
	result[1] = name
	result[2] = paramSyms
	copy(result[3:], body)
	return result
}
