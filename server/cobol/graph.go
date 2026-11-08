package cobol

import (
	"strconv"
	"strings"
)

func isLiteral(s string) bool { return s != "" && (s[0] == '\'' || s[0] == '"') }

// flow handles PERFORM/GO/CALL/ALTER at body[i] and returns the index to
// resume from (tokens that were consumed are never data references).
func (ps *parser) flow(body []Token, i int, cur string, paras []string) int {
	t := body[i]
	edge := func(from, to, kind string, uncertain bool) {
		ps.p.Edges = append(ps.p.Edges, Edge{From: from, To: to, Kind: kind, Loc: t.Loc, Uncertain: uncertain})
	}
	warn := func(kind, detail string) {
		ps.p.Warnings = append(ps.p.Warnings, Warning{kind, detail, t.Loc})
	}
	at := func(k int) string {
		if k < len(body) {
			return body[k].Text
		}
		return ""
	}
	switch t.Text {
	case "PERFORM":
		x := at(i + 1)
		if _, num := strconv.Atoi(x); num == nil || x == "" || isLiteral(x) || ps.known[x] {
			return i // inline PERFORM (UNTIL/VARYING/TIMES/...): no edge
		}
		switch x {
		case "UNTIL", "VARYING", "WITH", "TEST", "FOREVER":
			return i
		}
		y := at(i + 2)
		if (y == "THRU" || y == "THROUGH") && at(i+3) != "" {
			y = at(i + 3)
			a, b := indexOf(paras, x), indexOf(paras, y)
			if a >= 0 && b >= a {
				for _, n := range paras[a : b+1] {
					edge(cur, n, "perform", false)
				}
				return i
			}
			edge(cur, x, "perform", false)
			edge(cur, y, "perform", false)
			return i
		}
		edge(cur, x, "perform", false)
	case "GO":
		j := i + 1
		if at(j) == "TO" {
			j++
		}
		var names []Token
		for j < len(body) && !endsStatement(body[j].Text) && body[j].Text != "DEPENDING" {
			names = append(names, body[j])
			j++
		}
		if len(names) == 0 {
			return i
		}
		if at(j) == "DEPENDING" {
			var ns []string
			for _, n := range names {
				edge(cur, n.Text, "goto", true)
				ns = append(ns, n.Text)
			}
			warn(WGotoDepending, strings.Join(ns, " "))
			return j - 1
		}
		edge(cur, names[0].Text, "goto", false)
		return j - 1
	case "CALL":
		x := at(i + 1)
		switch {
		case x == "":
		case isLiteral(x):
			edge(cur, strings.ToUpper(strings.Trim(x, `"'`)), "call", false)
		default:
			ps.p.Edges = append(ps.p.Edges, Edge{From: cur, To: "?", Kind: "call", Loc: t.Loc, Dynamic: true})
			warn(WDynamicCall, x)
		}
	case "ALTER":
		p, q := at(i+1), ""
		for j := i + 2; j < len(body) && !endsStatement(body[j].Text); j++ {
			if w := body[j].Text; w != "TO" && w != "PROCEED" {
				q = w
			}
		}
		warn(WAlter, "ALTER "+p+" TO "+q)
		if p != "" && q != "" {
			edge(p, q, "goto", true)
		}
	}
	return i
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
