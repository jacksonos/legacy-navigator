package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// run serves one canned reply, checks path/header and returns the decoded request body.
func run(t *testing.T, p func(base string) Provider, path, hdr, hval, reply string) (string, map[string]any) {
	t.Helper()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Header.Get(hdr) != hval || r.Method != "POST" {
			t.Errorf("bad request %s %s %s=%q", r.Method, r.URL.Path, hdr, r.Header.Get(hdr))
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		io.WriteString(w, reply)
	}))
	defer srv.Close()
	got, err := p(srv.URL).Complete(context.Background(), "SYS", "USR")
	if err != nil {
		t.Fatal(err)
	}
	return got, body
}

func fails(t *testing.T, p func(base string) Provider) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, strings.Repeat("x", 500), 500)
	}))
	defer srv.Close()
	_, err := p(srv.URL).Complete(context.Background(), "s", "u")
	if err == nil || !strings.Contains(err.Error(), "500") || len(err.Error()) > 340 || strings.Contains(err.Error(), "KEY") {
		t.Fatalf("err = %v", err)
	}
}

func TestAnthropic(t *testing.T) {
	mk := func(b string) Provider { return NewAnthropic("KEY", "m1", b, nil) }
	got, body := run(t, mk, "/v1/messages", "x-api-key", "KEY", `{"content":[{"type":"text","text":"a"},{"type":"tool","text":"no"},{"type":"text","text":"b"}]}`)
	if got != "ab" || body["system"] != "SYS" || body["model"] != "m1" || body["max_tokens"] != float64(1500) {
		t.Fatalf("%q %v", got, body)
	}
	if m := body["messages"].([]any)[0].(map[string]any); m["role"] != "user" || m["content"] != "USR" {
		t.Fatal(m)
	}
	fails(t, mk)
}

func TestOpenAI(t *testing.T) {
	mk := func(b string) Provider { return NewOpenAI("KEY", "m2", b, nil) }
	got, body := run(t, mk, "/v1/chat/completions", "Authorization", "Bearer KEY", `{"choices":[{"message":{"content":"hola"}}]}`)
	ms := body["messages"].([]any)
	if got != "hola" || body["model"] != "m2" || len(ms) != 2 || ms[0].(map[string]any)["role"] != "system" || ms[1].(map[string]any)["content"] != "USR" {
		t.Fatalf("%q %v", got, body)
	}
	fails(t, mk)
	run(t, mk, "/v1/chat/completions", "X-Title", "", `{"choices":[{"message":{"content":"x"}}]}`) // no X-Title
}

func TestOpenRouter(t *testing.T) {
	mk := func(b string) Provider { return NewOpenRouter("KEY", "openai/gpt-4o", b, nil) }
	reply := `{"choices":[{"message":{"content":"hola"}}]}`
	run(t, mk, "/v1/chat/completions", "X-Title", "Legacy Code Navigator", reply)
	got, body := run(t, mk, "/v1/chat/completions", "Authorization", "Bearer KEY", reply)
	if got != "hola" || body["model"] != "openai/gpt-4o" {
		t.Fatalf("%q %v", got, body)
	}
	fails(t, mk)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer srv.Close()
	if _, err := mk(srv.URL).Complete(context.Background(), "s", "u"); err == nil || !strings.HasPrefix(err.Error(), "openrouter:") {
		t.Fatalf("err = %v", err)
	}
}

func TestGemini(t *testing.T) {
	mk := func(b string) Provider { return NewGemini("KEY", "m3", b, nil) }
	got, body := run(t, mk, "/v1beta/models/m3:generateContent", "x-goog-api-key", "KEY", `{"candidates":[{"content":{"parts":[{"text":"x"},{"text":"y"}]}}]}`)
	si := body["systemInstruction"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"]
	c := body["contents"].([]any)[0].(map[string]any)
	if got != "xy" || si != "SYS" || c["role"] != "user" || c["parts"].([]any)[0].(map[string]any)["text"] != "USR" {
		t.Fatalf("%q %v", got, body)
	}
	fails(t, mk)
}

func TestFromEnv(t *testing.T) {
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_MODEL", "OPENAI_API_KEY", "OPENAI_MODEL", "GEMINI_API_KEY", "GEMINI_MODEL", "OPENROUTER_API_KEY", "OPENROUTER_MODEL"} {
		t.Setenv(k, "")
	}
	if len(FromEnv()) != 0 {
		t.Fatal("expected none")
	}
	t.Setenv("GEMINI_API_KEY", "k")
	t.Setenv("GEMINI_MODEL", "g")
	t.Setenv("OPENAI_API_KEY", "k") // no model: skipped
	t.Setenv("ANTHROPIC_API_KEY", "k")
	t.Setenv("OPENROUTER_API_KEY", "k")
	t.Setenv("OPENROUTER_MODEL", "openai/gpt-4o")
	ps := FromEnv()
	if len(ps) != 3 || ps[0].ID() != "anthropic" || ps[0].Model() != "claude-sonnet-5-5" || ps[1].ID() != "gemini" || ps[2].ID() != "openrouter" {
		t.Fatalf("%v", ps)
	}
}
