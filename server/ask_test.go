package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacksonos/legacy-navigator/server/llm"
)

type fakeProvider struct {
	text string
	err  error
	user string
}

func (f *fakeProvider) ID() string    { return "openai" }
func (f *fakeProvider) Model() string { return "fake" }
func (f *fakeProvider) Complete(_ context.Context, _, user string) (string, error) {
	f.user = user
	return f.text, f.err
}

const canonical = "¿Dónde se calcula el saldo y qué archivos intervienen?"

func askHandler(t *testing.T, ps ...llm.Provider) (http.Handler, string) {
	t.Helper()
	db, err := openDB(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h := newHandler(fstest.MapFS{}, db, ps)
	return h, samplesProject(t, h).ID
}

func postAsk(t *testing.T, h http.Handler, id, q string, out any) int {
	t.Helper()
	body := `{"question":` + jsonStr(q) + `,"lang":"es"}`
	return do(t, h, "POST", "/api/projects/"+id+"/ask", "application/json", []byte(body), out)
}

func jsonStr(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

func covers(ev []Evidence, file string, line int) bool {
	for _, e := range ev {
		if e.File == file && e.StartLine <= line && line <= e.EndLine {
			return true
		}
	}
	return false
}

func TestQuestionTerms(t *testing.T) {
	got := strings.Join(questionTerms(canonical), ",")
	if got != "calcula,saldo" {
		t.Fatalf("terms = %q", got)
	}
}

func TestAskEvidenceModeWithoutProviders(t *testing.T) {
	h, id := askHandler(t)
	var res askResponse
	if code := postAsk(t, h, id, canonical, &res); code != 200 {
		t.Fatalf("code %d", code)
	}
	if res.Mode != "evidence" || res.Provider != "" || res.Answer != "" {
		t.Fatalf("%+v", res)
	}
	if !covers(res.Evidence, "CALCSALD.cbl", 86) || !covers(res.Evidence, "CALCSALD.cbl", 88) || !covers(res.Evidence, "CTAREG.cpy", 5) {
		t.Fatalf("evidence misses expected ranges: %+v", res.Evidence)
	}
	var par bool
	for _, e := range res.Evidence {
		par = par || (e.Paragraph == "4000-CALCULAR-SALDO" && e.File == "CALCSALD.cbl" && strings.Contains(e.Text, "COMPUTE CTA-SALDO"))
	}
	if !par {
		t.Fatalf("no 4000 paragraph: %+v", res.Evidence)
	}
	var pr struct{ Providers []any }
	do(t, h, "GET", "/api/providers", "", nil, &pr)
	if pr.Providers == nil || len(pr.Providers) != 0 {
		t.Fatalf("providers = %v", pr.Providers)
	}
}

func TestAskWithProviderValidatesCitations(t *testing.T) {
	fp := &fakeProvider{text: "El saldo se calcula [CALCSALD.cbl:86], no en [CALCSALD.cbl:5] ni en [NOPE.cbl:1]."}
	h, id := askHandler(t, fp)
	var res askResponse
	if code := postAsk(t, h, id, canonical, &res); code != 200 {
		t.Fatalf("code %d", code)
	}
	if res.Mode != "llm" || res.Provider != "openai" {
		t.Fatalf("%+v", res)
	}
	if res.Answer != "El saldo se calcula [CALCSALD.cbl:86], no en ni en." {
		t.Fatalf("answer %q", res.Answer)
	}
	if len(res.Citations) != 1 || res.Citations[0] != (Citation{"CALCSALD.cbl", 86, 86}) {
		t.Fatalf("citations %+v", res.Citations)
	}
	if len(res.Unverified) != 2 || res.Unverified[0] != "CALCSALD.cbl:5" || res.Unverified[1] != "NOPE.cbl:1" {
		t.Fatalf("unverified %v", res.Unverified)
	}
	if !strings.Contains(fp.user, "=== FILE CALCSALD.cbl lines ") || !strings.Contains(fp.user, "086|") {
		t.Fatalf("prompt: %s", fp.user)
	}
	var pr struct{ Providers []struct{ ID, Model string } }
	do(t, h, "GET", "/api/providers", "", nil, &pr)
	if len(pr.Providers) != 1 || pr.Providers[0].Model != "fake" {
		t.Fatalf("providers = %v", pr.Providers)
	}
}

func TestAskErrors(t *testing.T) {
	fp := &fakeProvider{err: errors.New("boom")}
	h, id := askHandler(t, fp)
	var res map[string]any
	if code := postAsk(t, h, id, canonical, &res); code != 502 || res["error"] == nil {
		t.Fatalf("provider error: %d %v", code, res)
	}
	fp.err = context.DeadlineExceeded
	if code := postAsk(t, h, id, canonical, &res); code != 504 {
		t.Fatalf("timeout: %d", code)
	}
	if code := postAsk(t, h, id, "  ", &res); code != 400 {
		t.Fatalf("empty: %d", code)
	}
	if code := postAsk(t, h, id, strings.Repeat("a", 501), &res); code != 400 {
		t.Fatalf("long: %d", code)
	}
	if code := postAsk(t, h, "nope", canonical, &res); code != 404 {
		t.Fatalf("404: %d", code)
	}
	req := httptest.NewRequest("POST", "/api/projects/"+id+"/ask", strings.NewReader(`{"question":"saldo","provider":"gemini"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("unconfigured provider: %d", w.Code)
	}
}

func TestAskNoEvidenceSkipsLLM(t *testing.T) {
	fp := &fakeProvider{text: "inventado [CALCSALD.cbl:86]"}
	h, id := askHandler(t, fp)
	var res askResponse
	if code := postAsk(t, h, id, "zzzqx blorptang", &res); code != 200 {
		t.Fatalf("code %d", code)
	}
	if res.Mode != "evidence" || res.Answer != "" || res.Evidence == nil || len(res.Evidence) != 0 || fp.user != "" {
		t.Fatalf("%+v user=%q", res, fp.user)
	}
}
