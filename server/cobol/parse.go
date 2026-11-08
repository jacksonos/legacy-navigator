package cobol

import (
	"path"
	"sort"
	"strconv"
	"strings"
)

// Analyze parses every program in files (name -> content). Copybooks are
// the other files, matched by basename without extension.
func Analyze(files map[string]string) *Project {
	proj := &Project{}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	c := &copier{files: files, byKey: map[string]string{}, copied: map[string]bool{}, cache: map[string][]Token{}}
	for _, n := range names {
		if _, dup := c.byKey[baseKey(n)]; !dup {
			c.byKey[baseKey(n)] = n
		}
	}

	type expanded struct {
		name  string
		toks  []Token
		warns []Warning
	}
	var exp []expanded
	for _, n := range names {
		if isFreeFormat(files[n]) {
			proj.Warnings = append(proj.Warnings, Warning{WFreeFormat, ">>SOURCE FORMAT FREE is not supported", Loc{n, 1}})
			continue
		}
		var w []Warning
		toks := c.expand(c.tokens(n), []string{baseKey(n)}, &w)
		exp = append(exp, expanded{n, toks, w})
	}

	for _, e := range exp {
		id, hasDiv := programID(e.toks)
		switch {
		case id != "":
			proj.Programs = append(proj.Programs, *parseProgram(e.name, id, e.toks, e.warns))
		case c.copied[e.name] || strings.EqualFold(path.Ext(e.name), ".cpy"):
			proj.Warnings = append(proj.Warnings, e.warns...)
		default:
			d := "no DIVISION found"
			if hasDiv {
				d = "no PROGRAM-ID found"
			}
			proj.Warnings = append(proj.Warnings, e.warns...)
			proj.Warnings = append(proj.Warnings, Warning{WNotCobol, d, Loc{e.name, 1}})
		}
	}

	ids := map[string]bool{}
	for _, p := range proj.Programs {
		ids[p.ID] = true
	}
	for _, p := range proj.Programs {
		for _, e := range p.Edges {
			if e.Kind == "call" && !e.Dynamic && !ids[e.To] {
				proj.Warnings = append(proj.Warnings, Warning{WUnresolvedCall, e.To, e.Loc})
			}
		}
	}
	return proj
}

// programID returns the PROGRAM-ID (after an IDENTIFICATION/ID DIVISION) and
// whether any DIVISION exists.
func programID(toks []Token) (id string, hasDiv bool) {
	idDiv := false
	for i, t := range toks {
		if i+1 < len(toks) && toks[i+1].Text == "DIVISION" {
			hasDiv = true
			idDiv = idDiv || t.Text == "IDENTIFICATION" || t.Text == "ID"
		}
		if idDiv && t.Text == "PROGRAM-ID" {
			for j := i + 1; j < len(toks); j++ {
				if toks[j].Text != "." {
					return toks[j].Text, hasDiv
				}
			}
		}
	}
	return "", hasDiv
}

type parser struct {
	p       *Program
	known   map[string]bool   // data items and file names
	recFile map[string]string // record name -> owning file
}

func parseProgram(file, id string, toks []Token, warns []Warning) *Program {
	p := &Program{ID: id, File: file, Warnings: warns}
	ps := &parser{p: p, known: map[string]bool{}, recFile: map[string]string{}}
	var env, data, proc []Token
	div := ""
	for i, t := range toks {
		if i+1 < len(toks) && toks[i+1].Text == "DIVISION" {
			div = t.Text
		}
		switch div {
		case "ENVIRONMENT":
			env = append(env, t)
		case "DATA":
			data = append(data, t)
		case "PROCEDURE":
			proc = append(proc, t)
		}
	}
	ps.environment(env)
	ps.dataDivision(data)
	ps.procedure(proc)
	return p
}

func (ps *parser) def(name string, loc Loc) {
	ps.known[name] = true
	ps.p.Refs = append(ps.p.Refs, Ref{Name: name, Kind: "def", Loc: loc})
}

func (ps *parser) environment(toks []Token) {
	for i, t := range toks {
		if t.Text != "SELECT" || i+1 >= len(toks) {
			continue
		}
		j := i + 1
		if toks[j].Text == "OPTIONAL" && j+1 < len(toks) {
			j++
		}
		f := FileDecl{Name: toks[j].Text, Loc: t.Loc}
		for k := j + 1; k < len(toks) && toks[k].Text != "."; k++ {
			if toks[k].Text == "ASSIGN" {
				k++
				if k < len(toks) && toks[k].Text == "TO" {
					k++
				}
				if k < len(toks) {
					f.Assign = strings.Trim(toks[k].Text, `"'`)
				}
				break
			}
		}
		ps.p.Files = append(ps.p.Files, f)
		ps.def(f.Name, t.Loc)
	}
}

func sentences(toks []Token) [][]Token {
	var out [][]Token
	start := 0
	for i, t := range toks {
		if t.Text == "." {
			out = append(out, toks[start:i])
			start = i + 1
		}
	}
	if start < len(toks) {
		out = append(out, toks[start:])
	}
	return out
}

func (ps *parser) dataDivision(toks []Token) {
	var stack []DataItem
	pendingFD := ""
	for _, s := range sentences(toks) {
		if len(s) == 0 {
			continue
		}
		if s[0].Text == "FD" || s[0].Text == "SD" {
			if len(s) > 1 {
				pendingFD = s[1].Text
			}
			continue
		}
		lvl, err := strconv.Atoi(s[0].Text)
		if err != nil {
			continue
		}
		name := ""
		if len(s) > 1 {
			switch s[1].Text {
			case "FILLER", "PIC", "PICTURE", "VALUE", "OCCURS", "USAGE", "REDEFINES":
			default:
				name = s[1].Text
			}
		}
		it := DataItem{Name: name, Level: lvl, Loc: s[0].Loc}
		for k, t := range s {
			if t.Text == "PIC" || t.Text == "PICTURE" {
				if k+1 < len(s) && s[k+1].Text == "IS" {
					k++
				}
				if k+1 < len(s) {
					it.Pic = s[k+1].Text
				}
				break
			}
		}
		switch {
		case lvl == 88:
			if len(stack) > 0 {
				it.Parent = stack[len(stack)-1].Name
			}
		case lvl >= 1 && lvl <= 49:
			for len(stack) > 0 && stack[len(stack)-1].Level >= lvl {
				stack = stack[:len(stack)-1]
			}
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].Name != "" {
					it.Parent = stack[k].Name
					break
				}
			}
			stack = append(stack, it)
			if lvl == 1 && pendingFD != "" {
				for k := range ps.p.Files {
					if ps.p.Files[k].Name == pendingFD && ps.p.Files[k].Record == "" {
						ps.p.Files[k].Record = name
					}
				}
				if name != "" {
					ps.recFile[name] = pendingFD
				}
				pendingFD = ""
			}
		}
		if name != "" {
			ps.p.Data = append(ps.p.Data, it)
			ps.def(name, it.Loc)
		}
	}
}

// Statement verbs and the mode their operands start in.
var verbMode = map[string]string{
	"MOVE": "read", "COMPUTE": "write", "ADD": "read", "SUBTRACT": "read",
	"MULTIPLY": "read", "DIVIDE": "read", "READ": "io", "WRITE": "io",
	"REWRITE": "io", "DELETE": "io", "START": "io", "OPEN": "io", "CLOSE": "io",
	"ACCEPT": "write", "INITIALIZE": "write", "SET": "write", "STRING": "read",
	"UNSTRING": "read", "PERFORM": "read", "GO": "read", "CALL": "read",
	"ALTER": "read",
}

// Words that end the operand list of a statement (back to read mode).
var resetWord = map[string]bool{
	"IF": true, "ELSE": true, "WHEN": true, "EVALUATE": true, "DISPLAY": true,
	"STOP": true, "GOBACK": true, "EXIT": true, "CONTINUE": true, "INSPECT": true,
	"SEARCH": true, "ON": true, "AT": true, "INVALID": true, "NOT": true,
	"UNTIL": true, "VARYING": true, "THEN": true, "RETURN": true, "SORT": true,
	"MERGE": true, "CANCEL": true, "RELEASE": true,
}

func endsStatement(w string) bool {
	_, v := verbMode[w]
	return w == "." || v || resetWord[w] || strings.HasPrefix(w, "END-")
}

// givingAhead reports whether the current statement has a GIVING clause.
func givingAhead(body []Token, i int) bool {
	for j := i + 1; j < len(body) && !endsStatement(body[j].Text); j++ {
		if body[j].Text == "GIVING" {
			return true
		}
	}
	return false
}

// trigger returns the new operand mode when keyword w appears in a statement
// with the given verb ("" = unchanged).
func trigger(verb, w string, body []Token, i int) string {
	tgt := func() string { // TO/FROM/BY/INTO target: written unless GIVING follows
		if givingAhead(body, i) {
			return "read"
		}
		return "write"
	}
	switch verb {
	case "MOVE":
		if w == "TO" {
			return "write"
		}
	case "COMPUTE":
		if w == "=" {
			return "read"
		}
	case "ADD", "SUBTRACT", "MULTIPLY", "DIVIDE":
		switch {
		case w == "GIVING":
			return "write"
		case verb == "ADD" && w == "TO", verb == "SUBTRACT" && w == "FROM",
			verb == "MULTIPLY" && w == "BY", verb == "DIVIDE" && w == "INTO":
			return tgt()
		case verb == "DIVIDE" && w == "BY":
			return "read"
		}
	case "READ":
		switch w {
		case "INTO":
			return "write"
		case "KEY":
			return "read"
		}
	case "WRITE", "REWRITE":
		switch w {
		case "FROM", "ADVANCING", "BEFORE", "AFTER":
			return "read"
		}
	case "INITIALIZE":
		if w == "REPLACING" {
			return "read"
		}
	case "SET":
		switch w {
		case "TO", "UP", "DOWN", "BY":
			return "read"
		}
	case "STRING", "UNSTRING":
		switch w {
		case "INTO":
			return "write"
		case "WITH", "POINTER", "DELIMITER", "COUNT", "TALLYING":
			return "read"
		}
	}
	return ""
}

func (ps *parser) procedure(proc []Token) {
	h := 0
	for h < len(proc) && proc[h].Text != "." {
		h++
	}
	if len(proc) == 0 {
		return
	}
	header, body := proc[:min(h+1, len(proc))], proc[min(h+1, len(proc)):]
	ps.refs(header, ps.p.ID, "read")

	// Structure: sections and paragraphs.
	type start struct {
		name    string
		section bool
		width   int
	}
	starts := map[int]start{}
	for i, t := range body {
		if (i != 0 && body[i-1].Text != ".") || t.Col < 8 || t.Col > 11 || i+1 >= len(body) {
			continue
		}
		if body[i+1].Text == "." {
			starts[i] = start{t.Text, false, 2}
		} else if body[i+1].Text == "SECTION" && i+2 < len(body) && body[i+2].Text == "." {
			starts[i] = start{t.Text, true, 3}
		}
	}
	var idxs []int
	for i := range starts {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	endBefore := func(k int) int { // line of the last token before body[k]
		if k >= len(body) {
			k = len(body)
		}
		if k == 0 {
			return 0
		}
		return body[k-1].Loc.Line
	}
	sect := ""
	var paraNames []string
	for n, i := range idxs {
		s := starts[i]
		blk := Block{Name: s.name, Loc: body[i].Loc}
		if s.section {
			sect = s.name
			end := len(body)
			for _, k := range idxs[n+1:] {
				if starts[k].section {
					end = k
					break
				}
			}
			blk.EndLine = endBefore(end)
			ps.p.Sections = append(ps.p.Sections, blk)
			continue
		}
		next := len(body)
		if n+1 < len(idxs) {
			next = idxs[n+1]
		}
		blk.Section, blk.EndLine = sect, endBefore(next)
		ps.p.Paragraphs = append(ps.p.Paragraphs, blk)
		paraNames = append(paraNames, s.name)
	}

	// Statements.
	cur, verb, mode := ps.p.ID, "", "read"
	for i := 0; i < len(body); i++ {
		t, w := body[i], body[i].Text
		if s, ok := starts[i]; ok {
			cur, verb, mode = s.name, "", "read"
			i += s.width - 1
			continue
		}
		switch {
		case w == "END" && i+1 < len(body) && body[i+1].Text == "PROGRAM":
			return
		case w == ".":
			verb, mode = "", "read"
			continue
		case w == "EXEC":
			for i++; i < len(body) && body[i].Text != "END-EXEC"; i++ {
				if strings.HasPrefix(body[i].Text, ":") {
					ps.refs([]Token{{strings.TrimPrefix(body[i].Text, ":"), body[i].Loc, body[i].Col}}, cur, "read")
				}
			}
			verb, mode = "", "read"
			continue
		}
		if m, ok := verbMode[w]; ok {
			verb, mode = w, m
			i = ps.flow(body, i, cur, paraNames)
			continue
		}
		if resetWord[w] || strings.HasPrefix(w, "END-") {
			verb, mode = "", "read"
			continue
		}
		if m := trigger(verb, w, body, i); m != "" {
			mode = m
			continue
		}
		ps.refs([]Token{t}, cur, mode)
	}
}

// refs emits a Ref for every known name inside the tokens.
func (ps *parser) refs(toks []Token, para, mode string) {
	for _, t := range toks {
		if t.Text == "" || t.Text[0] == '\'' || t.Text[0] == '"' {
			continue
		}
		parts := strings.FieldsFunc(t.Text, func(r rune) bool { return r == '(' || r == ')' || r == ':' })
		for k, c := range parts {
			if !ps.known[c] {
				continue
			}
			m := mode
			if k > 0 {
				m = "read" // subscripts and ref-mod operands are only read
			}
			ps.p.Refs = append(ps.p.Refs, Ref{c, m, t.Loc, para})
			if f, ok := ps.recFile[c]; ok && m == "io" {
				ps.p.Refs = append(ps.p.Refs, Ref{f, "io", t.Loc, para})
			}
		}
	}
}
