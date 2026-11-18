package main

import (
	"database/sql"
	"strings"
	"testing"
)

const clJob = `/* JOB DE PRUEBA */
PGM
  OVRDBF FILE(MOVIMIENTOS) TOFILE(BANCOLIB/MOVDIA)
  CALL PGM(BANCOLIB/CALCSALD)
  SBMJOB CMD(CALL PGM(LISTADO)) JOB(LST)
  CALL PGM(NOEXISTE)
ENDPGM
`

var cobCalcsald = fixed("IDENTIFICATION DIVISION.", "PROGRAM-ID. CALCSALD.", "ENVIRONMENT DIVISION.", "INPUT-OUTPUT SECTION.",
	"FILE-CONTROL.", "    SELECT MOVIMIENTOS ASSIGN TO 'MOV.DAT'.", "DATA DIVISION.", "FILE SECTION.", "FD  MOVIMIENTOS.",
	"01  MOV-REG PIC X(10).", "PROCEDURE DIVISION.", "MAIN.", "    OPEN INPUT MOVIMIENTOS.", "    CALL 'JOB'.", "    STOP RUN.")

var cobListado = fixed("IDENTIFICATION DIVISION.", "PROGRAM-ID. LISTADO.", "PROCEDURE DIVISION.", "MAIN.", "    GOBACK.")

func TestCLWithCobol(t *testing.T) {
	h := testHandler(t)
	code, p := upload(t, h, map[string]string{"JOB.clp": clJob, "CALCSALD.cbl": cobCalcsald, "LISTADO.cbl": cobListado, "R.cpy": fixed("01  X PIC X.")})
	if code != 201 {
		t.Fatalf("upload: %d", code)
	}
	lang := map[string]string{}
	for _, f := range p.Files {
		lang[f.Name] = f.Language
	}
	if lang["JOB.clp"] != "cl" || lang["CALCSALD.cbl"] != "cobol" || lang["R.cpy"] != "copybook" {
		t.Fatalf("languages %v", lang)
	}
	var job FileSummary
	for _, f := range p.Files {
		if f.Name == "JOB.clp" {
			job = f
		}
	}
	if job.ProgramID != "JOB" || len(job.ProgramIDs) != 1 {
		t.Fatalf("job summary %+v", job)
	}
	// CL -> COBOL, COBOL -> CL resolved; only the call to NOEXISTE is unresolved
	var un []string
	for _, w := range append(append([]Warning{}, p.Warnings...), job.Warnings...) {
		if w.Kind == "unresolved_call" {
			un = append(un, w.Detail)
		}
	}
	if strings.Join(un, ",") != "NOEXISTE" {
		t.Fatalf("unresolved %v (project %+v)", un, p.Warnings)
	}

	g := getGraph(t, h, p.ID, "?view=system")
	if !g.edge("JOB", "CALCSALD", "call") || !g.edge("CALCSALD", "JOB", "call") || !g.edge("JOB", "LISTADO", "call") || !g.edge("JOB", "ext:NOEXISTE", "call") {
		t.Fatalf("edges %+v", g.Edges)
	}
	for _, e := range g.Edges {
		if want := e.From == "JOB" && e.To == "LISTADO"; e.Async != want {
			t.Errorf("async of %s>%s = %v", e.From, e.To, e.Async)
		}
	}
	if n := g.node("JOB"); n == nil || n.Language != "cl" || n.Kind != "program" {
		t.Fatalf("JOB node %+v", n)
	}
	if n := g.node("CALCSALD"); n == nil || n.Language != "cobol" {
		t.Fatalf("CALCSALD node %+v", n)
	}
	if n := g.node("ext:NOEXISTE"); n == nil || n.Language != "" {
		t.Fatalf("external node %+v", n)
	}
	if !g.Partial {
		t.Error("unresolved call must make the map partial")
	}

	// the CL file override shows up as an io ref with its target on the COBOL file name
	var res struct{ Results []SymbolHit }
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q=MOVIMIENTOS&kind=file", "", nil, &res); code != 200 || len(res.Results) != 1 {
		t.Fatalf("search: %d %+v", code, res.Results)
	}
	var found Ref
	for _, r := range res.Results[0].Refs {
		if r.File == "JOB.clp" {
			found = r
		}
	}
	if found.Kind != "io" || found.Detail != "BANCOLIB/MOVDIA" || found.Line != 3 || found.Program != "JOB" {
		t.Fatalf("OVRDBF ref %+v in %+v", found, res.Results[0].Refs)
	}
	// CL variables and labels are searchable symbols
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q=JOB&kind=program", "", nil, &res); code != 200 || len(res.Results) != 1 || res.Results[0].Name != "JOB" {
		t.Fatalf("search program: %d %+v", code, res.Results)
	}
}

func TestCLTxtAndExtensions(t *testing.T) {
	h := testHandler(t)
	for _, name := range []string{"A.clle", "B.CL", "C.CLP"} {
		if code, _ := upload(t, h, map[string]string{name: "PGM\nENDPGM\n"}); code != 201 {
			t.Errorf("%s: %d", name, code)
		}
	}
	code, p := upload(t, h, map[string]string{"cierre.txt": "PGM\n CALL X\nENDPGM\n", "otro.txt": "hola\n"})
	if code != 201 {
		t.Fatalf("txt: %d", code)
	}
	for _, f := range p.Files {
		if f.Name == "cierre.txt" && (f.Language != "cl" || f.ProgramID != "CIERRE") {
			t.Errorf("cierre.txt: %+v", f)
		}
		if f.Name == "otro.txt" && f.Language != "cobol" {
			t.Errorf("otro.txt: %+v", f)
		}
	}
}

func TestOldDatabaseGetsNewColumns(t *testing.T) {
	path := t.TempDir() + "/old.db"
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.Exec(`CREATE TABLE programs(project TEXT NOT NULL, name TEXT NOT NULL, file TEXT NOT NULL, start_line INTEGER NOT NULL, end_line INTEGER NOT NULL);
		CREATE TABLE refs(project TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL, file TEXT NOT NULL, line INTEGER NOT NULL, paragraph TEXT NOT NULL);
		CREATE TABLE edges(project TEXT NOT NULL, from_name TEXT NOT NULL, to_name TEXT NOT NULL, kind TEXT NOT NULL, file TEXT NOT NULL, line INTEGER NOT NULL, uncertain INTEGER NOT NULL, dynamic INTEGER NOT NULL);
		INSERT INTO programs VALUES('p','OLD','OLD.cbl',1,9)`)
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := openDB(path); err != nil { // idempotent
		t.Fatal(err)
	}
	var lang string
	if err := db.QueryRow(`SELECT language FROM programs`).Scan(&lang); err != nil || lang != "cobol" {
		t.Fatalf("language %q: %v", lang, err)
	}
	if _, err := db.Exec(`SELECT detail FROM refs; SELECT async FROM edges`); err != nil {
		t.Fatal(err)
	}
}
