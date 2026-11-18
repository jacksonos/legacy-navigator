package main

import "testing"

func TestLargeDemoCL(t *testing.T) {
	h := testHandler(t)
	p := largeProject(t, h)
	var f FileSummary
	for _, x := range p.Files {
		if x.Name == "CIERREDIA.clp" {
			f = x
		}
	}
	if f.Language != "cl" || f.ProgramID != "CIERREDIA" || f.Paragraphs != 2 || len(f.Warnings) != 0 {
		t.Fatalf("CIERREDIA.clp: %+v", f)
	}
	sys := getGraph(t, h, p.ID, "?view=system")
	if len(sys.Edges) > maxGraphEdges {
		t.Fatalf("system edges: %d", len(sys.Edges))
	}
	if n := sys.node("CIERREDIA"); n == nil || n.Language != "cl" {
		t.Fatalf("CIERREDIA node %+v", n)
	}
	if n := sys.node("BATCHDIA"); n == nil || n.Language != "cobol" {
		t.Fatalf("BATCHDIA node %+v", n)
	}
	async := map[string]bool{}
	for _, e := range sys.Edges {
		if e.From == "CIERREDIA" && e.Kind == "call" {
			async[e.To] = e.Async
		}
	}
	if _, ok := async["BATCHDIA"]; len(async) != 2 || !ok || async["BATCHDIA"] || !async["EXTRACTO"] {
		t.Fatalf("CIERREDIA calls (async by target): %v", async)
	}
	pv := getGraph(t, h, p.ID, "?view=program&program=CIERREDIA")
	if !pv.edge("CIERREDIA:ERROR", "CIERREDIA:ERROR", "goto") && !pv.edge("CIERREDIA", "CIERREDIA:ERROR", "goto") {
		t.Fatalf("program view edges %+v", pv.Edges)
	}

	var ans askResponse
	if code := postAsk(t, h, p.ID, "Â¿QuÃ© hace el cierre diario?", &ans); code != 200 || ans.Mode != "evidence" {
		t.Fatalf("ask: %d %+v", code, ans)
	}
	cl := false
	for _, e := range ans.Evidence {
		cl = cl || e.File == "CIERREDIA.clp"
	}
	if !cl {
		t.Fatalf("no CIERREDIA.clp in evidence: %+v", ans.Evidence)
	}
}
