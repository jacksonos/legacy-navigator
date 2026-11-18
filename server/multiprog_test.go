package main

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func fixed(lines ...string) string {
	return "       " + strings.Join(lines, "\n       ") + "\n"
}

// Two consecutive programs; B copies C.cpy. Line numbers are asserted below.
var twoProgs = fixed("IDENTIFICATION DIVISION.", "PROGRAM-ID. A.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 1-4
	"01  XAA PIC X.", "PROCEDURE DIVISION.", "MAIN.", // 5-7
	"    MOVE 'A' TO XAA.", "    CALL 'B'.", "    STOP RUN.", "END PROGRAM A.", // 8-11
	"IDENTIFICATION DIVISION.", "PROGRAM-ID. B.", "DATA DIVISION.", "WORKING-STORAGE SECTION.", // 12-15
	"COPY C.", "01  YBB PIC X.", "PROCEDURE DIVISION.", "MAIN.", // 16-19
	"    MOVE 'B' TO YBB.", "    MOVE YBB TO CPYVAR.", "    GOBACK.", "END PROGRAM B.") // 20-23

// OUTER contains INNER.
var nestedProgs = fixed("IDENTIFICATION DIVISION.", "PROGRAM-ID. OUTER.", "PROCEDURE DIVISION.", "MAIN.", // 1-4
	"    CALL 'INNER'.", "    STOP RUN.", // 5-6
	"IDENTIFICATION DIVISION.", "PROGRAM-ID. INNER.", "PROCEDURE DIVISION.", "MAIN.", // 7-10
	"    GOBACK.", "END PROGRAM INNER.", "END PROGRAM OUTER.") // 11-13

func TestSeveralProgramsPerFile(t *testing.T) {
	h := testHandler(t)
	code, p := upload(t, h, map[string]string{"M.cbl": twoProgs, "N.cbl": nestedProgs, "C.cpy": fixed("01  CPYVAR PIC X.")})
	if code != 201 {
		t.Fatalf("upload %d", code)
	}
	want := map[string][]string{"M.cbl": {"A", "B"}, "N.cbl": {"OUTER", "INNER"}, "C.cpy": {}}
	for _, f := range p.Files {
		if !reflect.DeepEqual(f.ProgramIDs, want[f.Name]) || (len(want[f.Name]) > 0 && f.ProgramID != want[f.Name][0]) || (len(want[f.Name]) == 0 && f.ProgramID != "") {
			t.Errorf("%s: programId %q programIds %v", f.Name, f.ProgramID, f.ProgramIDs)
		}
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings %+v", p.Warnings)
	}

	type hit struct {
		name, program, defFile string
		defLine                int
	}
	search := func(q, kind string) (hits []hit, res struct {
		Results []SymbolHit `json:"results"`
	}) {
		if c := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q="+url.QueryEscape(q)+"&kind="+kind, "", nil, &res); c != 200 {
			t.Fatalf("search %s: %d", q, c)
		}
		for _, r := range res.Results {
			hits = append(hits, hit{r.Name, r.Program, r.Def.File, r.Def.Line})
		}
		return
	}
	for _, c := range []struct {
		q, kind string
		want    []hit
	}{
		{"MAIN", "paragraph", []hit{{"MAIN", "A", "M.cbl", 7}, {"MAIN", "B", "M.cbl", 19}, {"MAIN", "OUTER", "N.cbl", 4}, {"MAIN", "INNER", "N.cbl", 10}}},
		{"XAA", "data", []hit{{"XAA", "A", "M.cbl", 5}}},
		{"YBB", "data", []hit{{"YBB", "B", "M.cbl", 17}}},
		{"CPYVAR", "data", []hit{{"CPYVAR", "", "C.cpy", 1}}},
		{"INNER", "program", []hit{{"INNER", "INNER", "N.cbl", 8}}},
	} {
		if got, _ := search(c.q, c.kind); !reflect.DeepEqual(got, c.want) {
			t.Errorf("search %s: %v, want %v", c.q, got, c.want)
		}
	}
	_, res := search("YBB", "data")
	var refs []string
	for _, r := range res.Results[0].Refs {
		refs = append(refs, r.Kind+"@"+r.Program)
	}
	if !reflect.DeepEqual(refs, []string{"write@B", "read@B"}) {
		t.Errorf("YBB refs = %v", refs)
	}
	_, res = search("CPYVAR", "data")
	if r := res.Results[0].Refs; len(r) != 1 || r[0].Program != "B" || r[0].Line != 21 {
		t.Errorf("CPYVAR refs = %+v", r)
	}

	// Evidence carries the program of its range.
	var ans askResponse
	if c := do(t, h, "POST", "/api/projects/"+p.ID+"/ask", "application/json", []byte(`{"question":"YBB","lang":"en"}`), &ans); c != 200 {
		t.Fatalf("ask %d", c)
	}
	var found bool
	for _, e := range ans.Evidence {
		if e.File == "M.cbl" && e.StartLine <= 20 && 20 <= e.EndLine {
			found = true
			if e.Program != "B" || e.Paragraph != "MAIN" {
				t.Errorf("evidence %+v", e)
			}
		}
	}
	if !found {
		t.Errorf("no evidence for M.cbl:20: %+v", ans.Evidence)
	}

	// Graph: the program view of the second program holds only its paragraphs.
	var g Graph
	if c := do(t, h, "GET", "/api/projects/"+p.ID+"/graph?view=program&program=B", "", nil, &g); c != 200 {
		t.Fatalf("graph %d", c)
	}
	ids := map[string]bool{}
	for _, n := range g.Nodes {
		ids[n.ID] = true
	}
	if !ids["B"] || !ids["B:MAIN"] || ids["A:MAIN"] || ids["INNER:MAIN"] {
		t.Errorf("program B view nodes = %v", ids)
	}
	var sys Graph
	do(t, h, "GET", "/api/projects/"+p.ID+"/graph?view=system", "", nil, &sys)
	var calls []string
	for _, e := range sys.Edges {
		calls = append(calls, e.From+">"+e.To)
	}
	if !reflect.DeepEqual(sys.Programs, []string{"A", "B", "INNER", "OUTER"}) || !reflect.DeepEqual(calls, []string{"A>B", "OUTER>INNER"}) {
		t.Errorf("system view: programs %v calls %v", sys.Programs, calls)
	}
	var foc Graph
	do(t, h, "GET", "/api/projects/"+p.ID+"/graph?view=focus&node=INNER:MAIN", "", nil, &foc)
	if foc.Program != "INNER" {
		t.Errorf("focus program = %q", foc.Program)
	}
}

// Projects saved before the programs table existed still resolve programs from the symbols.
func TestProgramIndexFallback(t *testing.T) {
	db, err := openDB(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h := newHandler(fstest.MapFS{}, db, nil)
	_, p := upload(t, h, map[string]string{"M.cbl": twoProgs, "C.cpy": fixed("01  CPYVAR PIC X.")})
	if _, err := db.Exec(`DELETE FROM programs`); err != nil {
		t.Fatal(err)
	}
	ix, err := loadProgIndex(db, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{ix.at("M.cbl", 8), ix.at("M.cbl", 20), ix.at("C.cpy", 1)}; !reflect.DeepEqual(got, []string{"A", "B", ""}) {
		t.Errorf("fallback programs = %v", got)
	}
}
