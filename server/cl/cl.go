// Package cl parses IBM i Control Language (CL/CLP/CLLE) sources held in memory.
// It reuses the cobol package types (Program, Block, Ref, Edge, Warning) so the
// store, graph and retrieval treat CL and COBOL programs alike. It never does I/O.
package cl

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/jacksonos/legacy-navigator/server/cobol"
)

var (
	pgmRe    = regexp.MustCompile(`(?im)^\s*(?:[A-Za-z#$@][\w#$@]*:\s*)?PGM\b`)
	endPgmRe = regexp.MustCompile(`(?im)^\s*ENDPGM\b`)
	labelRe  = regexp.MustCompile(`^([A-Za-z#$@][A-Za-z0-9#$@_]*):\s*(.*)$`)
	varRe    = regexp.MustCompile(`&[A-Za-z#$@][A-Za-z0-9#$@_]*`)
)

// Detect reports whether a file is CL: by extension (.clp .clle .cl) or, for .txt, by a PGM and an ENDPGM command.
func Detect(name, content string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".clp", ".clle", ".cl":
		return true
	case ".txt":
		return pgmRe.MatchString(content) && endPgmRe.MatchString(content)
	}
	return false
}

// Analyze parses every CL file (name -> content). Unresolved-call warnings are
// left to the caller, which knows the programs of every language.
func Analyze(files map[string]string) *cobol.Project {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	proj := &cobol.Project{}
	for _, n := range names {
		proj.Programs = append(proj.Programs, *parseFile(n, files[n]))
	}
	return proj
}

type stmt struct {
	text string
	line int
}

// logical joins physical lines into commands: comments are removed (also multi-line),
// a trailing + continues the command skipping the next line's leading blanks, a trailing - keeps them.
func logical(content string) (out []stmt, nlines int) {
	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(content, "\r\n", "\n"), "\n"), "\n")
	var cur strings.Builder
	start, trimNext, joining, inCom := 0, false, false, false
	for i, raw := range lines {
		var sb strings.Builder
		inStr := false
		for j := 0; j < len(raw); j++ {
			c := raw[j]
			next := byte(0)
			if j+1 < len(raw) {
				next = raw[j+1]
			}
			switch {
			case inCom:
				if c == '*' && next == '/' {
					inCom = false
					j++
					sb.WriteByte(' ')
				}
			case inStr:
				sb.WriteByte(c)
				if c == '\'' {
					if next == '\'' {
						sb.WriteByte(next)
						j++
					} else {
						inStr = false
					}
				}
			case c == '\'':
				inStr = true
				sb.WriteByte(c)
			case c == '/' && next == '*':
				inCom = true
				j++
			default:
				sb.WriteByte(c)
			}
		}
		t := strings.TrimRight(sb.String(), " \t")
		if joining && trimNext {
			t = strings.TrimLeft(t, " \t")
		}
		if !joining {
			if strings.TrimSpace(t) == "" {
				continue
			}
			start = i + 1
		}
		if !inStr && (strings.HasSuffix(t, "+") || strings.HasSuffix(t, "-")) {
			trimNext = strings.HasSuffix(t, "+")
			cur.WriteString(t[:len(t)-1])
			joining = true
			continue
		}
		cur.WriteString(t)
		out = append(out, stmt{strings.TrimSpace(cur.String()), start})
		cur.Reset()
		joining = false
	}
	if joining {
		out = append(out, stmt{strings.TrimSpace(cur.String()), start})
	}
	return out, len(lines)
}

type param struct {
	kw, val string
	group   bool // a bare (...) group
	end     int  // index in the source just after the parameter
}

// closeParen returns the index of the ) matching the ( at s[i], or len(s) if unbalanced.
func closeParen(s string, i int) int {
	depth, inStr := 0, false
	for ; i < len(s); i++ {
		switch c := s[i]; {
		case inStr:
			inStr = c != '\''
		case c == '\'':
			inStr = true
		case c == '(':
			depth++
		case c == ')':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return len(s)
}

// parseParams splits the text after a command name into KWD(value), bare (group), 'string' and word parameters.
func parseParams(s string) []param {
	var out []param
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == ' ' || c == '\t':
			i++
		case c == '(':
			e := closeParen(s, i)
			out = append(out, param{val: strings.TrimSpace(s[i+1 : e]), group: true, end: min(e+1, len(s))})
			i = min(e+1, len(s))
		case c == '\'':
			j := i + 1
			for j < len(s) {
				if s[j] == '\'' {
					if j+1 < len(s) && s[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			j = min(j+1, len(s))
			out = append(out, param{val: s[i:j], end: j})
			i = j
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '(' {
				j++
			}
			w := s[i:j]
			if j < len(s) && s[j] == '(' && w != "" {
				e := closeParen(s, j)
				out = append(out, param{kw: strings.ToUpper(w), val: strings.TrimSpace(s[j+1 : e]), end: min(e+1, len(s))})
				i = min(e+1, len(s))
			} else {
				out = append(out, param{val: w, end: j})
				i = j
			}
		}
	}
	return out
}

// arg returns the value of keyword kw, or else of the pos-th positional parameter (-1: keyword only).
func arg(ps []param, kw string, pos int) string {
	n := 0
	for _, p := range ps {
		if p.kw == kw {
			return p.val
		}
	}
	for _, p := range ps {
		if p.kw == "" {
			if n == pos {
				return p.val
			}
			n++
		}
	}
	return ""
}

// base strips quotes and the library qualifier and uppercases: BANCOLIB/BATCHDIA -> BATCHDIA.
func base(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "'\"")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToUpper(s)
}

type parser struct {
	p      *cobol.Program
	file   string
	cur    string // current label, or the program id before the first label
	label  string // current label ("" before the first)
	dclf   string // last DCLF file
	blocks []cobol.Block
}

func parseFile(name, content string) *cobol.Program {
	stmts, n := logical(content)
	id := strings.ToUpper(strings.TrimSuffix(path.Base(name), path.Ext(name)))
	ps := &parser{p: &cobol.Program{ID: id, Language: "cl", File: name, IDLine: 1, StartLine: 1, EndLine: n}, file: name, cur: id}
	for _, s := range stmts {
		text := s.text
		if m := labelRe.FindStringSubmatch(text); m != nil {
			l := strings.ToUpper(m[1])
			if k := len(ps.blocks); k > 0 {
				ps.blocks[k-1].EndLine = s.line - 1
			}
			ps.blocks = append(ps.blocks, cobol.Block{Name: l, Loc: cobol.Loc{File: name, Line: s.line}, EndLine: n})
			ps.label, ps.cur = l, l
			text = m[2]
		}
		ps.command(text, s.line, false)
	}
	ps.p.Paragraphs = ps.blocks
	return ps.p
}

func (ps *parser) loc(line int) cobol.Loc { return cobol.Loc{File: ps.file, Line: line} }

func (ps *parser) ref(name, kind string, line int, detail string) {
	ps.p.Refs = append(ps.p.Refs, cobol.Ref{Name: name, Kind: kind, Loc: ps.loc(line), Paragraph: ps.label, Detail: detail})
}

// vars records a ref of the given kind for every &VAR in text (quoted strings are literals).
func (ps *parser) vars(text, kind string, line int) {
	var sb strings.Builder
	inStr := false
	for i := 0; i < len(text); i++ {
		if text[i] == '\'' {
			inStr = !inStr
		}
		if inStr || text[i] == '\'' {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(text[i])
		}
	}
	for _, v := range varRe.FindAllString(sb.String(), -1) {
		ps.ref(strings.ToUpper(v), kind, line, "")
	}
}

func (ps *parser) edge(to, kind string, line int, async bool) {
	ps.p.Edges = append(ps.p.Edges, cobol.Edge{From: ps.cur, To: to, Kind: kind, Loc: ps.loc(line), Async: async})
}

func (ps *parser) call(target string, line int, async bool) {
	if strings.HasPrefix(strings.TrimSpace(target), "&") {
		ps.p.Edges = append(ps.p.Edges, cobol.Edge{From: ps.cur, To: "?", Kind: "call", Loc: ps.loc(line), Dynamic: true, Async: async})
		ps.p.Warnings = append(ps.p.Warnings, cobol.Warning{Kind: cobol.WDynamicCall, Detail: strings.ToUpper(strings.TrimSpace(target)), Loc: ps.loc(line)})
		ps.vars(target, "read", line)
	} else if t := base(target); t != "" {
		ps.edge(t, "call", line, async)
	}
}

// command interprets one command (or a nested one: THEN, ELSE, EXEC, SBMJOB CMD).
func (ps *parser) command(text string, line int, async bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	end := strings.IndexAny(text, " \t(")
	if end < 0 {
		end = len(text)
	}
	name, rest := strings.ToUpper(text[:end]), text[end:]
	pr := parseParams(rest)
	switch name {
	case "PGM":
		ps.p.IDLine = line
	case "ENDPGM", "DO", "ENDDO", "SELECT", "ENDSELECT":
	case "DCL":
		if v := strings.ToUpper(arg(pr, "VAR", 0)); v != "" {
			typ := arg(pr, "TYPE", 1)
			if l := arg(pr, "LEN", 2); l != "" {
				typ += "(" + l + ")"
			}
			ps.p.Data = append(ps.p.Data, cobol.DataItem{Name: v, Level: 1, Pic: typ, Loc: ps.loc(line)})
		}
	case "DCLF":
		for _, f := range strings.Fields(arg(pr, "FILE", 0)) {
			ps.dclf = base(f)
			ps.p.Files = append(ps.p.Files, cobol.FileDecl{Name: ps.dclf, Assign: strings.ToUpper(f), Loc: ps.loc(line)})
			ps.ref(ps.dclf, "io", line, "DCLF")
		}
	case "RCVF", "SNDRCVF":
		if ps.dclf != "" {
			ps.ref(ps.dclf, "io", line, name)
		}
	case "CALL", "CALLPRC", "TFRCTL":
		kw := "PGM"
		if name == "CALLPRC" {
			kw = "PRC"
		}
		ps.call(arg(pr, kw, 0), line, async)
		ps.vars(arg(pr, "PARM", 1), "read", line)
	case "SBMJOB":
		for _, p := range pr {
			if p.kw == "CMD" {
				ps.command(p.val, line, true)
			} else {
				ps.vars(p.val, "read", line)
			}
		}
	case "OVRDBF":
		to := strings.ToUpper(arg(pr, "TOFILE", 1))
		for _, f := range strings.Fields(arg(pr, "FILE", 0)) {
			d := "OVRDBF"
			if to != "" {
				d = to
				if m := arg(pr, "MBR", -1); m != "" {
					d += " MBR(" + strings.ToUpper(m) + ")"
				}
			}
			ps.ref(base(f), "io", line, d)
		}
	case "DLTOVR":
		for _, f := range strings.Fields(arg(pr, "FILE", 0)) {
			if f != "*ALL" {
				ps.ref(base(f), "io", line, "DLTOVR")
			}
		}
	case "CHGVAR":
		ps.vars(arg(pr, "VAR", 0), "write", line)
		ps.vars(arg(pr, "VALUE", 1), "read", line)
	case "MONMSG":
		for _, p := range pr {
			if p.kw == "EXEC" {
				ps.command(p.val, line, false)
			}
		}
	case "GOTO":
		l := strings.Trim(arg(pr, "CMDLBL", 0), "() ")
		if l != "" {
			ps.edge(strings.ToUpper(l), "goto", line, false)
		}
	case "IF", "WHEN":
		cond, then := -1, ""
		for i, p := range pr {
			if p.kw == "COND" || (cond < 0 && p.group && p.kw == "") {
				cond = i
				break
			}
		}
		if cond >= 0 {
			ps.vars(pr[cond].val, "read", line)
		}
		if t := arg(pr, "THEN", -1); t != "" {
			then = t
		} else if cond >= 0 {
			then = rest[pr[cond].end:]
		}
		ps.command(then, line, false)
	case "ELSE", "OTHERWISE":
		if c := arg(pr, "CMD", -1); c != "" {
			ps.command(c, line, false)
		} else {
			ps.command(rest, line, false)
		}
	default:
		kind := "read"
		if strings.HasPrefix(name, "RTV") {
			kind = "write"
		}
		for _, p := range pr {
			k := kind
			switch p.kw {
			case "DTAARA", "SYSVAL", "MSGID", "MSGF", "FILE", "MBR":
				k = "read"
			}
			ps.vars(p.val, k, line)
		}
	}
}
