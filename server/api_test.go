package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := openDB(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return newHandler(fstest.MapFS{}, db)
}

func do(t *testing.T, h http.Handler, method, path, ctype string, body []byte, out any) int {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: %v: %q", method, path, err, w.Body.String())
		}
	}
	return w.Code
}

func upload(t *testing.T, h http.Handler, files map[string]string) (int, Project) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, content := range files {
		fw, _ := mw.CreateFormFile("files", name)
		fw.Write([]byte(content))
	}
	mw.Close()
	var p Project
	code := do(t, h, "POST", "/api/projects", mw.FormDataContentType(), buf.Bytes(), &p)
	return code, p
}

func samplesProject(t *testing.T, h http.Handler) Project {
	t.Helper()
	var p Project
	if code := do(t, h, "POST", "/api/projects?samples=1", "", nil, &p); code != 201 {
		t.Fatalf("samples: %d", code)
	}
	return p
}

func TestSamplesProject(t *testing.T) {
	h := testHandler(t)
	p := samplesProject(t, h)
	if len(p.ID) != 16 {
		t.Fatalf("id %q", p.ID)
	}
	var got Project
	if code := do(t, h, "GET", "/api/projects/"+p.ID, "", nil, &got); code != 200 {
		t.Fatalf("get: %d", code)
	}
	var calc, cpy *FileSummary
	for i, f := range got.Files {
		switch f.Name {
		case "CALCSALD.cbl":
			calc = &got.Files[i]
		case "CTAREG.cpy":
			cpy = &got.Files[i]
		}
	}
	if calc == nil || calc.ProgramID != "CALCSALD" || calc.Paragraphs == 0 {
		t.Fatalf("CALCSALD summary: %+v", calc)
	}
	if cpy == nil || cpy.ProgramID != "" || cpy.Paragraphs != 0 || cpy.DataItems == 0 {
		t.Fatalf("CTAREG summary: %+v", cpy)
	}
	found := false
	for _, w := range calc.Warnings {
		found = found || (w.Kind == "missing_copy" && strings.Contains(w.Detail, "AUDITREG"))
	}
	if !found {
		t.Fatalf("missing_copy AUDITREG not in %+v", calc.Warnings)
	}
}

func TestSearch(t *testing.T) {
	h := testHandler(t)
	p := samplesProject(t, h)
	var res struct {
		Results []SymbolHit `json:"results"`
		Lines   []LineHit   `json:"lines"`
	}
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q=saldo", "", nil, &res); code != 200 {
		t.Fatalf("search: %d", code)
	}
	var hit *SymbolHit
	for i, s := range res.Results {
		if s.Name == "CTA-SALDO" {
			hit = &res.Results[i]
		}
	}
	if hit == nil || hit.Kind != "data" || hit.Def != (Def{"CTAREG.cpy", 5}) {
		t.Fatalf("CTA-SALDO hit: %+v", hit)
	}
	var writes []int
	for _, r := range hit.Refs {
		if r.File == "CALCSALD.cbl" && r.Kind == "write" {
			writes = append(writes, r.Line)
		}
	}
	if len(writes) != 2 || writes[0] != 86 || writes[1] != 88 {
		t.Fatalf("write refs: %+v", hit.Refs)
	}
	comment := false
	for _, l := range res.Lines {
		comment = comment || strings.Contains(l.Text, "CALCULA EL SALDO")
	}
	if !comment {
		t.Fatalf("comment line missing from %+v", res.Lines)
	}

	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q=saldo&kind=paragraph", "", nil, &res); code != 200 {
		t.Fatalf("kind filter: %d", code)
	}
	for _, s := range res.Results {
		if s.Kind != "paragraph" {
			t.Fatalf("kind filter leaked %+v", s)
		}
	}
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/search?q=%20-%22", "", nil, nil); code != 400 {
		t.Fatalf("empty query: %d", code)
	}
}

func TestUploadValidation(t *testing.T) {
	h := testHandler(t)
	if code, _ := upload(t, h, map[string]string{"evil.exe": "x"}); code != 400 {
		t.Fatalf("bad ext: %d", code)
	}
	if code, _ := upload(t, h, map[string]string{"big.cbl": strings.Repeat("x", maxFile+1)}); code != 413 {
		t.Fatalf("oversize: %d", code)
	}
	if code, _ := upload(t, h, map[string]string{"../../etc/A.CBL": "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. A.\n"}); code != 201 {
		t.Fatalf("path strip: %d", code)
	}
	if code := do(t, h, "POST", "/api/projects", "", nil, nil); code != 400 {
		t.Fatalf("no body: %d", code)
	}
}

func TestNotFoundAndFile(t *testing.T) {
	h := testHandler(t)
	var e map[string]string
	if code := do(t, h, "GET", "/api/projects/nope", "", nil, &e); code != 404 || e["error"] == "" {
		t.Fatalf("unknown project: %d %v", code, e)
	}
	if code := do(t, h, "GET", "/api/projects/nope/search?q=a", "", nil, nil); code != 404 {
		t.Fatalf("unknown search: %d", code)
	}
	p := samplesProject(t, h)
	var f struct {
		Name  string   `json:"name"`
		Lines []string `json:"lines"`
	}
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/files/CTAREG.cpy", "", nil, &f); code != 200 {
		t.Fatalf("file: %d", code)
	}
	var want int
	for _, s := range p.Files {
		if s.Name == "CTAREG.cpy" {
			want = s.Lines
		}
	}
	if f.Name != "CTAREG.cpy" || len(f.Lines) != want || want == 0 {
		t.Fatalf("lines %d want %d", len(f.Lines), want)
	}
	if code := do(t, h, "GET", "/api/projects/"+p.ID+"/files/NOPE.cbl", "", nil, nil); code != 404 {
		t.Fatalf("unknown file: %d", code)
	}
}
