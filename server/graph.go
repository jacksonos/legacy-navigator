package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

type GraphNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Resolved bool   `json:"resolved"`
}
type GraphEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Kind      string `json:"kind"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Uncertain bool   `json:"uncertain"`
}
type Graph struct {
	Nodes   []GraphNode `json:"nodes"`
	Edges   []GraphEdge `json:"edges"`
	Partial bool        `json:"partial"`
	Reasons []Warning   `json:"reasons"`
}

var partialKinds = map[string]bool{"missing_copy": true, "dynamic_call": true, "unresolved_call": true, "alter": true, "goto_depending": true}

func buildGraph(db *sql.DB, id string) (Graph, error) {
	g := Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}, Reasons: []Warning{}}
	prog := map[string]string{} // file -> program ID
	have := map[string]bool{}
	add := func(n GraphNode) {
		if !have[n.ID] {
			have[n.ID] = true
			g.Nodes = append(g.Nodes, n)
		}
	}
	rows, err := db.Query(`SELECT name, kind, file, line FROM symbols WHERE project=? AND kind IN ('program','section','paragraph') ORDER BY file, line`, id)
	if err != nil {
		return g, err
	}
	type sym struct {
		name, kind, file string
		line             int
	}
	var syms []sym
	for rows.Next() {
		var s sym
		if err := rows.Scan(&s.name, &s.kind, &s.file, &s.line); err != nil {
			rows.Close()
			return g, err
		}
		syms = append(syms, s)
		if s.kind == "program" {
			prog[s.file] = s.name
		}
	}
	rows.Close()
	for _, s := range syms {
		if s.kind == "program" {
			add(GraphNode{s.name, s.name, "program", s.file, s.line, true})
		} else if p := prog[s.file]; p != "" {
			add(GraphNode{p + ":" + s.name, s.name, s.kind, s.file, s.line, true})
		}
	}

	rows, err = db.Query(`SELECT from_name, to_name, kind, file, line, uncertain, dynamic FROM edges WHERE project=? ORDER BY file, line, rowid`, id)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	seen := map[GraphEdge]bool{}
	for rows.Next() {
		var from, to string
		var e GraphEdge
		var dyn bool
		if err := rows.Scan(&from, &to, &e.Kind, &e.File, &e.Line, &e.Uncertain, &dyn); err != nil {
			return g, err
		}
		p := prog[e.File]
		if p == "" {
			continue
		}
		e.From = p + ":" + from
		if from == p {
			e.From = p
		}
		switch {
		case dyn:
			e.To = fmt.Sprintf("dyn:%s:%d", e.File, e.Line)
			add(GraphNode{e.To, "?", "dynamic", e.File, e.Line, false})
		case e.Kind == "call":
			e.To = to
			if !have[to] {
				e.To = "ext:" + to
				add(GraphNode{e.To, to, "external", "", 0, false})
			}
		default:
			e.To = p + ":" + to
			if to == p {
				e.To = p
			}
		}
		if !have[e.From] || !have[e.To] || seen[e] {
			continue
		}
		seen[e] = true
		g.Edges = append(g.Edges, e)
	}
	if err := rows.Err(); err != nil {
		return g, err
	}
	return g, nil
}

func (a *api) graph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !projectExists(a.db, id) {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	g, err := buildGraph(a.db, id)
	if err != nil {
		serverError(w, err)
		return
	}
	rows, err := a.db.Query(`SELECT DISTINCT kind, detail, file, line FROM warnings WHERE project=? ORDER BY file, line, kind`, id)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var x Warning
		if err := rows.Scan(&x.Kind, &x.Detail, &x.File, &x.Line); err != nil {
			serverError(w, err)
			return
		}
		if partialKinds[x.Kind] {
			g.Reasons = append(g.Reasons, x)
		}
	}
	g.Partial = len(g.Reasons) > 0
	writeJSON(w, http.StatusOK, g)
}
