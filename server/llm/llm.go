// Package llm talks to OpenAI, Anthropic, Gemini and OpenRouter over plain HTTP (no SDKs).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Provider interface {
	ID() string
	Model() string
	Complete(ctx context.Context, system, user string) (string, error)
}

type client struct {
	id, key, model, base string
	hc                   *http.Client
}

func newClient(id, key, model, base, defBase string, hc *http.Client) client {
	if base == "" {
		base = defBase
	}
	if hc == nil {
		hc = &http.Client{Timeout: 90 * time.Second}
	}
	return client{id, key, model, strings.TrimRight(base, "/"), hc}
}

func (c client) ID() string    { return c.id }
func (c client) Model() string { return c.model }

// post sends body as JSON and decodes the reply into out. Errors never include the key.
func (c client) post(ctx context.Context, path string, headers map[string]string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s: request failed", c.id) // the url.Error text is dropped on purpose
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		ex := strings.TrimSpace(string(raw))
		if len(ex) > 300 {
			ex = ex[:300]
		}
		return fmt.Errorf("%s: status %d: %s", c.id, resp.StatusCode, ex)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: bad response: %v", c.id, err)
	}
	return nil
}

type Anthropic struct{ client }

func NewAnthropic(key, model, base string, hc *http.Client) *Anthropic {
	return &Anthropic{newClient("anthropic", key, model, base, "https://api.anthropic.com", hc)}
}

func (p *Anthropic) Complete(ctx context.Context, system, user string) (string, error) {
	var out struct {
		Content []struct{ Type, Text string } `json:"content"`
	}
	err := p.post(ctx, "/v1/messages", map[string]string{"x-api-key": p.key, "anthropic-version": "2023-06-01"}, map[string]any{
		"model": p.model, "max_tokens": 1500, "system": system,
		"messages": []map[string]string{{"role": "user", "content": user}},
	}, &out)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}

// OpenAI also serves OpenRouter (same chat-completions API) with another id, base and extra headers.
type OpenAI struct {
	client
	extra map[string]string
}

func NewOpenAI(key, model, base string, hc *http.Client) *OpenAI {
	return &OpenAI{client: newClient("openai", key, model, base, "https://api.openai.com", hc)}
}

func NewOpenRouter(key, model, base string, hc *http.Client) *OpenAI {
	return &OpenAI{newClient("openrouter", key, model, base, "https://openrouter.ai/api", hc),
		map[string]string{"X-Title": "Legacy Code Navigator"}}
}

func (p *OpenAI) Complete(ctx context.Context, system, user string) (string, error) {
	var out struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	hdr := map[string]string{"Authorization": "Bearer " + p.key}
	for k, v := range p.extra {
		hdr[k] = v
	}
	err := p.post(ctx, "/v1/chat/completions", hdr, map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": system}, {"role": "user", "content": user}},
	}, &out)
	if err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: empty response", p.id)
	}
	return out.Choices[0].Message.Content, nil
}

type Gemini struct{ client }

func NewGemini(key, model, base string, hc *http.Client) *Gemini {
	return &Gemini{newClient("gemini", key, model, base, "https://generativelanguage.googleapis.com", hc)}
}

func (p *Gemini) Complete(ctx context.Context, system, user string) (string, error) {
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	text := func(s string) []map[string]string { return []map[string]string{{"text": s}} }
	err := p.post(ctx, "/v1beta/models/"+p.model+":generateContent", map[string]string{"x-goog-api-key": p.key}, map[string]any{
		"systemInstruction": map[string]any{"parts": text(system)},
		"contents":          []map[string]any{{"role": "user", "parts": text(user)}},
	}, &out)
	if err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	var sb strings.Builder
	for _, x := range out.Candidates[0].Content.Parts {
		sb.WriteString(x.Text)
	}
	return sb.String(), nil
}

// FromEnv returns the providers with a key and a model, in order anthropic, openai, gemini, openrouter.
func FromEnv() []Provider {
	var ps []Provider
	add := func(env, def string, mk func(k, m string) Provider) {
		key := os.Getenv(env + "_API_KEY")
		if key == "" {
			return
		}
		model := os.Getenv(env + "_MODEL")
		if model == "" {
			model = def
		}
		if model == "" {
			log.Printf("warning: %s_API_KEY is set but %s_MODEL is missing; provider disabled", env, env)
			return
		}
		ps = append(ps, mk(key, model))
	}
	add("ANTHROPIC", "claude-sonnet-5-5", func(k, m string) Provider { return NewAnthropic(k, m, "", nil) })
	add("OPENAI", "", func(k, m string) Provider { return NewOpenAI(k, m, "", nil) })
	add("GEMINI", "", func(k, m string) Provider { return NewGemini(k, m, "", nil) })
	add("OPENROUTER", "", func(k, m string) Provider { return NewOpenRouter(k, m, "", nil) })
	return ps
}
