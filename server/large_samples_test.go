package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
)

func largeProject(t *testing.T, h http.Handler) Project {
	t.Helper()
	var p Project
	if code := do(t, h, "POST", "/api/projects?samples=large", "", nil, &p); code != 201 {
		t.Fatalf("large samples: %d", code)
	}
	return p
}

func TestSamplesUnknownSet(t *testing.T) {
	h := testHandler(t)
	if code := do(t, h, "POST", "/api/projects?samples=nope", "", nil, nil); code != 400 {
		t.Fatalf("unknown set: %d", code)
	}
}

func TestLargeSamples(t *testing.T) {
	h := testHandler(t)
	p := largeProject(t, h)

	wantProgs := []string{"AUDITLOG", "BATCHDIA", "CARGAMOV", "CIERRE", "COMISION", "ERRORES",
		"EXTRACTO", "FECHAS", "INTERESES", "MORAS", "SALDOS", "VALIDCTA"}
	var progs []string
	big, bigParas := "", 0
	for _, f := range p.Files {
		if strings.HasSuffix(f.Name, ".cbl") {
			if f.ProgramID == "" || f.ProgramID+".cbl" != f.Name {
				t.Fatalf("%s: programId %q", f.Name, f.ProgramID)
			}
			progs = append(progs, f.ProgramID)
			if f.Paragraphs > bigParas {
				big, bigParas = f.ProgramID, f.Paragraphs
			}
		}
	}
	sort.Strings(progs)
	if strings.Join(progs, ",") != strings.Join(wantProgs, ",") {
		t.Fatalf("programs %v", progs)
	}
	if bigParas <= 40 {
		t.Fatalf("biggest program %s has %d paragraphs", big, bigParas)
	}

	seen := map[Warning]bool{}
	var got []string
	for _, ws := range append([][]Warning{p.Warnings}, func() (r [][]Warning) {
		for _, f := range p.Files {
			r = append(r, f.Warnings)
		}
		return
	}()...) {
		for _, w := range ws {
			if seen[w] {
				continue
			}
			seen[w] = true
			d := w.Detail
			if w.Kind == "goto_depending" {
				d = strings.Fields(d)[0]
			}
			got = append(got, fmt.Sprintf("%s:%s:%s", w.Kind, d, w.File))
		}
	}
	sort.Strings(got)
	want := []string{
		"dynamic_call:WS-PROGRAMA:BATCHDIA.cbl",
		"goto_depending:3310-REGLA-DEPOSITO:VALIDCTA.cbl",
		"missing_copy:NOTIFREG:CIERRE.cbl",
		"unresolved_call:NOTIFICA:CIERRE.cbl",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("warnings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	sys := getGraph(t, h, p.ID, "?view=system")
	if len(sys.Edges) == 0 || len(sys.Edges) > maxGraphEdges {
		t.Fatalf("system edges: %d", len(sys.Edges))
	}
	for _, to := range []string{"CARGAMOV", "SALDOS", "INTERESES", "COMISION", "MORAS", "EXTRACTO", "CIERRE"} {
		if !sys.edge("BATCHDIA", to, "call") {
			t.Fatalf("missing call edge BATCHDIA -> %s", to)
		}
	}
	for _, e := range [][2]string{{"CARGAMOV", "VALIDCTA"}, {"VALIDCTA", "FECHAS"}, {"CIERRE", "ext:NOTIFICA"}, {"SALDOS", "AUDITLOG"}} {
		if !sys.edge(e[0], e[1], "call") {
			t.Fatalf("missing call edge %s -> %s", e[0], e[1])
		}
	}
	t.Logf("system view: %d nodes, %d edges", len(sys.Nodes), len(sys.Edges))

	pv := getGraph(t, h, p.ID, "?view=program&program="+big)
	if len(pv.Edges) == 0 || len(pv.Edges) > maxGraphEdges || (pv.View != "program" && pv.View != "focus") {
		t.Fatalf("program view of %s: %s, %d edges", big, pv.View, len(pv.Edges))
	}

	var res struct {
		Results []SymbolHit `json:"results"`
	}
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q=interes", "", nil, &res); code != 200 {
		t.Fatalf("search: %d", code)
	}
	found := false
	for _, r := range res.Results {
		found = found || (r.Def.File == "INTERESES.cbl" && strings.Contains(r.Name, "INTERES"))
	}
	if !found {
		t.Fatalf("search interes: %+v", res.Results)
	}

	var ans askResponse
	if code := postAsk(t, h, p.ID, "¿Cómo se calculan los intereses mensuales?", &ans); code != 200 {
		t.Fatalf("ask: %d", code)
	}
	if ans.Mode != "evidence" || len(ans.Evidence) == 0 {
		t.Fatalf("ask: %+v", ans)
	}
}
