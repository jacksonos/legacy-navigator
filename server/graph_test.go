package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// scaleGraph: 50 programs x 500 paragraphs; each paragraph performs the next 2,
// every 25th calls another program, and a few CALLs are unresolved externals.
func scaleGraph() Graph {
	g := Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}, Reasons: []Warning{}}
	const progs, paras = 50, 500
	for p := 0; p < progs; p++ {
		pn := fmt.Sprintf("P%02d", p)
		g.Nodes = append(g.Nodes, GraphNode{pn, pn, "program", pn + ".cbl", 1, true})
		for i := 0; i < paras; i++ {
			g.Nodes = append(g.Nodes, GraphNode{fmt.Sprintf("%s:PAR-%03d", pn, i), fmt.Sprintf("PAR-%03d", i), "paragraph", pn + ".cbl", 10 + i, true})
		}
	}
	for _, x := range []string{"EXT1", "EXT2", "EXT3"} {
		g.Nodes = append(g.Nodes, GraphNode{"ext:" + x, x, "external", "", 0, false})
	}
	for p := 0; p < progs; p++ {
		pn := fmt.Sprintf("P%02d", p)
		for i := 0; i < paras; i++ {
			from := fmt.Sprintf("%s:PAR-%03d", pn, i)
			for d := 1; d <= 2 && i+d < paras; d++ {
				g.Edges = append(g.Edges, GraphEdge{from, fmt.Sprintf("%s:PAR-%03d", pn, i+d), "perform", pn + ".cbl", 10 + i, false})
			}
			if i%25 == 0 {
				to := fmt.Sprintf("P%02d", (p+1+i/25)%progs)
				if i%250 == 0 {
					to = "ext:EXT1"
				}
				g.Edges = append(g.Edges, GraphEdge{from, to, "call", pn + ".cbl", 10 + i, false})
			}
		}
	}
	return g
}

func TestGraphViewsAtScale(t *testing.T) {
	g := scaleGraph()
	type req struct {
		view, program, node string
		depth               int
	}
	reqs := []req{{"system", "", "", 0}}
	for _, p := range []string{"P00", "P17", "P49"} {
		reqs = append(reqs, req{"program", p, "", 0})
	}
	for _, n := range []string{"P00:PAR-000", "P10:PAR-250", "P49:PAR-499", "P05", "ext:EXT1"} {
		for d := 1; d <= 3; d++ {
			reqs = append(reqs, req{"focus", "", n, d})
		}
	}
	start := time.Now()
	for _, r := range reqs {
		v := makeView(g, r.view, r.program, r.node, r.depth)
		tooLarge := false
		for _, w := range v.Reasons {
			if w.Kind == "too_large" {
				tooLarge = true
				var shown, total int
				fmt.Sscanf(w.Detail, "%d/%d", &shown, &total)
				if shown != len(v.Edges) || total <= shown {
					t.Fatalf("%+v: bad detail %q (edges %d)", r, w.Detail, len(v.Edges))
				}
			}
		}
		if len(v.Edges) > maxGraphEdges {
			t.Fatalf("%+v: %d edges", r, len(v.Edges))
		}
		if (r.view == "system" || strings.HasPrefix(r.program, "P")) && !tooLarge && len(v.Edges) == maxGraphEdges {
			t.Fatalf("%+v: truncated without too_large", r)
		}
		if v.View == "focus" && v.node(v.Node) == nil {
			t.Fatalf("%+v: center %s missing", r, v.Node)
		}
		if r.view == "program" && (v.View != "focus" || v.Depth != 2 || !tooLarge || v.Node != r.program+":PAR-000") {
			t.Fatalf("%+v: program not downgraded: %s depth %d %+v", r, v.View, v.Depth, v.Reasons)
		}
		if r.view == "system" && !tooLarge {
			t.Fatalf("system view of 50 programs should be truncated (%d edges)", len(v.Edges))
		}
		ids := map[string]bool{}
		for _, n := range v.Nodes {
			ids[n.ID] = true
		}
		for _, e := range v.Edges {
			if !ids[e.From] || !ids[e.To] {
				t.Fatalf("%+v: dangling edge %+v", r, e)
			}
		}
	}
	el := time.Since(start)
	t.Logf("scale: %d views over %d nodes / %d edges in %v", len(reqs), len(g.Nodes), len(g.Edges), el)
	if el > 2*time.Second {
		t.Fatalf("too slow: %v", el)
	}
}

func TestFocusDepthShrinksAndTruncates(t *testing.T) {
	g := Graph{Nodes: []GraphNode{{"HUB", "HUB", "program", "H.cbl", 1, true}}}
	for i := 0; i < 400; i++ { // hub with 400 leaves: depth 1 itself exceeds the cap
		id := fmt.Sprintf("L%03d", i)
		g.Nodes = append(g.Nodes, GraphNode{id, id, "program", "L.cbl", i, true})
		g.Edges = append(g.Edges, GraphEdge{"HUB", id, "call", "H.cbl", i, false})
	}
	v := makeView(g, "focus", "", "HUB", 3)
	if v.Depth != 1 || len(v.Edges) != maxGraphEdges || v.node("HUB") == nil || len(v.Nodes) != maxGraphEdges+1 {
		t.Fatalf("depth %d edges %d nodes %d", v.Depth, len(v.Edges), len(v.Nodes))
	}
	if len(v.Reasons) != 1 || v.Reasons[0].Detail != "300/400" {
		t.Fatalf("reasons %+v", v.Reasons)
	}
}
