package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// maxGraphEdges bounds every served view; the Mermaid client allows more (headroom).
const maxGraphEdges = 300

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
	View     string      `json:"view"`
	Program  string      `json:"program"`
	Node     string      `json:"node"`
	Depth    int         `json:"depth"`
	Programs []string    `json:"programs"`
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
	Partial  bool        `json:"partial"`
	Reasons  []Warning   `json:"reasons"`
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

// progOf returns the program that owns a paragraph/section/program node ("" for ext:/dyn:).
func progOf(id string) string {
	if strings.HasPrefix(id, "ext:") || strings.HasPrefix(id, "dyn:") {
		return ""
	}
	p, _, _ := strings.Cut(id, ":")
	return p
}

// sub keeps the nodes of g in keep (source order) and the given edges.
func sub(g Graph, keep map[string]bool, edges []GraphEdge) Graph {
	v := Graph{Nodes: []GraphNode{}, Edges: edges, Reasons: []Warning{}}
	if v.Edges == nil {
		v.Edges = []GraphEdge{}
	}
	for _, n := range g.Nodes {
		if keep[n.ID] {
			v.Nodes = append(v.Nodes, n)
		}
	}
	return v
}

func systemView(g Graph) Graph {
	keep := map[string]bool{}
	for _, n := range g.Nodes {
		keep[n.ID] = n.Kind == "program" || n.Kind == "external" || n.Kind == "dynamic"
	}
	type pair struct{ from, to string }
	seen := map[pair]bool{}
	var edges []GraphEdge
	for _, e := range g.Edges {
		if e.Kind != "call" {
			continue
		}
		e.From = progOf(e.From)
		if k := (pair{e.From, e.To}); !seen[k] && keep[e.From] && keep[e.To] {
			seen[k] = true
			edges = append(edges, e)
		}
	}
	return sub(g, keep, edges)
}

func programView(g Graph, prog string) Graph {
	keep := map[string]bool{}
	for _, n := range g.Nodes {
		keep[n.ID] = progOf(n.ID) == prog
	}
	var edges []GraphEdge
	for _, e := range g.Edges {
		if progOf(e.From) == prog {
			keep[e.To] = true
			edges = append(edges, e)
		}
	}
	return sub(g, keep, edges)
}

// focusView is an undirected BFS from node over all edges, up to depth hops.
func focusView(g Graph, node string, depth int) Graph {
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	keep := map[string]bool{node: true}
	frontier := []string{node}
	for d := 0; d < depth; d++ {
		var next []string
		for _, id := range frontier {
			for _, o := range adj[id] {
				if !keep[o] {
					keep[o] = true
					next = append(next, o)
				}
			}
		}
		frontier = next
	}
	var edges []GraphEdge
	for _, e := range g.Edges {
		if keep[e.From] && keep[e.To] {
			edges = append(edges, e)
		}
	}
	return sub(g, keep, edges)
}

// truncate keeps the first max edges (stable order), the nodes they touch and the center.
func truncate(full, v Graph, center string) Graph {
	keep := map[string]bool{}
	if center != "" {
		keep[center] = true
	}
	if len(v.Edges) > maxGraphEdges {
		v.Edges = v.Edges[:maxGraphEdges]
	}
	for _, e := range v.Edges {
		keep[e.From], keep[e.To] = true, true
	}
	return sub(full, keep, v.Edges)
}

// makeView derives a bounded view from the full graph. view/program/node/depth must already be valid.
func makeView(full Graph, view, program, node string, depth int) Graph {
	var v Graph
	total := 0
	switch view {
	case "system":
		v = systemView(full)
	case "program":
		v = programView(full, program)
		if len(v.Edges) > maxGraphEdges {
			for _, n := range full.Nodes {
				if n.Kind == "paragraph" && progOf(n.ID) == program {
					total, view, node, depth = len(v.Edges), "focus", n.ID, 2
					break
				}
			}
		}
	}
	if view == "focus" {
		v = focusView(full, node, depth)
		if total == 0 {
			total = len(v.Edges)
		}
		for len(v.Edges) > maxGraphEdges && depth > 1 {
			depth--
			v = focusView(full, node, depth)
		}
	}
	if total == 0 || total < len(v.Edges) {
		total = len(v.Edges)
	}
	if len(v.Edges) > maxGraphEdges {
		v = truncate(full, v, node)
	}
	v.View, v.Program, v.Node, v.Depth = view, program, node, 0
	if view == "focus" {
		v.Depth = depth
	}
	if total > len(v.Edges) {
		v.Reasons = append(v.Reasons, Warning{Kind: "too_large", Detail: fmt.Sprintf("%d/%d", len(v.Edges), total)})
	}
	return v
}

func (a *api) graph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !projectExists(a.db, id) {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	full, err := buildGraph(a.db, id)
	if err != nil {
		serverError(w, err)
		return
	}
	var programs []string
	progFile := map[string]string{} // program -> file
	have := map[string]bool{}
	for _, n := range full.Nodes {
		have[n.ID] = true
		if n.Kind == "program" {
			programs = append(programs, n.ID)
			progFile[n.ID] = n.File
		}
	}
	sort.Strings(programs)
	if programs == nil {
		programs = []string{}
	}

	q := r.URL.Query()
	view, program, node, depth := q.Get("view"), q.Get("program"), q.Get("node"), 1
	if d := q.Get("depth"); d != "" {
		var err error
		if depth, err = strconv.Atoi(d); err != nil || depth < 1 || depth > 3 {
			fail(w, http.StatusBadRequest, "depth must be 1..3")
			return
		}
	}
	if view == "" {
		view = "system"
		if len(programs) == 1 {
			view = "program"
		}
	}
	if view == "program" && program == "" && len(programs) == 1 {
		program = programs[0]
	}
	switch view {
	case "system":
		program = ""
	case "program":
		if _, ok := progFile[program]; !ok {
			fail(w, http.StatusBadRequest, "unknown program")
			return
		}
	case "focus":
		if !have[node] {
			fail(w, http.StatusBadRequest, "unknown node")
			return
		}
	default:
		fail(w, http.StatusBadRequest, "invalid view")
		return
	}

	g := makeView(full, view, program, node, depth)
	g.Programs = programs
	if g.Node != "" { // focus: the program the center belongs to (dyn: via its file; ext: none)
		g.Program = progOf(g.Node)
		if g.Program == "" {
			for _, n := range full.Nodes {
				if n.ID == g.Node && n.Kind == "dynamic" {
					for p, f := range progFile {
						if f == n.File {
							g.Program = p
						}
					}
				}
			}
		}
	}
	files := map[string]bool{}
	for _, n := range full.Nodes {
		if g.Program != "" && progOf(n.ID) == g.Program {
			files[n.File] = true
		}
	}
	rows, err := a.db.Query(`SELECT DISTINCT kind, detail, file, line FROM warnings WHERE project=? ORDER BY file, line, kind`, id)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	reasons := []Warning{}
	for rows.Next() {
		var x Warning
		if err := rows.Scan(&x.Kind, &x.Detail, &x.File, &x.Line); err != nil {
			serverError(w, err)
			return
		}
		if partialKinds[x.Kind] && (g.Program == "" || files[x.File]) {
			reasons = append(reasons, x)
		}
	}
	g.Reasons = append(reasons, g.Reasons...)
	g.Partial = len(g.Reasons) > 0
	writeJSON(w, http.StatusOK, g)
}
