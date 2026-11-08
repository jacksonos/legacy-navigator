package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var askTimeout = 60 * time.Second

type askRequest struct {
	Question string `json:"question"`
	Lang     string `json:"lang"`
	Provider string `json:"provider"`
}

type askResponse struct {
	Mode       string     `json:"mode"`
	Provider   string     `json:"provider"`
	Answer     string     `json:"answer"`
	Citations  []Citation `json:"citations"`
	Unverified []string   `json:"unverified"`
	Evidence   []Evidence `json:"evidence"`
}

func (a *api) listProviders(w http.ResponseWriter, _ *http.Request) {
	type item struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	out := []item{}
	for _, p := range a.providers {
		out = append(out, item{p.ID(), p.Model()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

func (a *api) ask(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if n := utf8.RuneCountInString(req.Question); n == 0 || n > 500 {
		fail(w, http.StatusBadRequest, "question must be 1 to 500 characters")
		return
	}
	id := r.PathValue("id")
	if !projectExists(a.db, id) {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	prov := -1
	if req.Provider != "" {
		for i, p := range a.providers {
			if p.ID() == req.Provider {
				prov = i
			}
		}
		if prov < 0 {
			fail(w, http.StatusBadRequest, "unknown or unconfigured provider: "+req.Provider)
			return
		}
	} else if len(a.providers) > 0 {
		prov = 0
	}
	ev, err := retrieve(a.db, id, req.Question, maxChars())
	if err != nil {
		serverError(w, err)
		return
	}
	res := askResponse{Mode: "evidence", Citations: []Citation{}, Unverified: []string{}, Evidence: ev}
	if prov >= 0 && len(ev) > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), askTimeout)
		defer cancel()
		system, user := buildPrompt(req.Question, req.Lang, ev)
		text, err := a.providers[prov].Complete(ctx, system, user)
		if errors.Is(err, context.DeadlineExceeded) {
			fail(w, http.StatusGatewayTimeout, "provider timed out")
			return
		} else if err != nil {
			fail(w, http.StatusBadGateway, err.Error())
			return
		}
		res.Mode, res.Provider = "llm", a.providers[prov].ID()
		res.Answer, res.Citations, res.Unverified = validateCitations(text, ev)
	}
	writeJSON(w, http.StatusOK, res)
}
