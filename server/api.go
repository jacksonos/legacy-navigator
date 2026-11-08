package main

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/jacksonos/legacy-navigator/server/cobol"
	"github.com/jacksonos/legacy-navigator/server/llm"
)

//go:embed samples
var samplesFS embed.FS

const (
	maxFile  = 1 << 20
	maxTotal = 10 << 20
	maxHits  = 50
)

type api struct {
	db        *sql.DB
	providers []llm.Provider
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	fail(w, http.StatusInternalServerError, err.Error())
}

func (a *api) register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/projects", a.create)
	mux.HandleFunc("GET /api/projects/{id}", a.project)
	mux.HandleFunc("GET /api/projects/{id}/files/{name}", a.file)
	mux.HandleFunc("GET /api/projects/{id}/search", a.search)
	mux.HandleFunc("GET /api/projects/{id}/graph", a.graph)
	mux.HandleFunc("POST /api/projects/{id}/ask", a.ask)
	mux.HandleFunc("GET /api/providers", a.listProviders)
}

func validExt(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".cbl", ".cob", ".cpy", ".txt":
		return true
	}
	return false
}

func (a *api) create(w http.ResponseWriter, r *http.Request) {
	files := map[string]string{}
	if r.URL.Query().Get("samples") == "1" {
		ents, err := samplesFS.ReadDir("samples")
		if err != nil {
			serverError(w, err)
			return
		}
		for _, e := range ents {
			b, err := samplesFS.ReadFile("samples/" + e.Name())
			if err != nil {
				serverError(w, err)
				return
			}
			files[e.Name()] = string(b)
		}
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, maxTotal)
		if err := r.ParseMultipartForm(maxTotal); err != nil {
			var mb *http.MaxBytesError
			if errors.As(err, &mb) {
				fail(w, http.StatusRequestEntityTooLarge, "upload exceeds 10 MB")
			} else {
				fail(w, http.StatusBadRequest, "expected multipart form with field \"files\"")
			}
			return
		}
		defer r.MultipartForm.RemoveAll()
		headers := r.MultipartForm.File["files"]
		if len(headers) == 0 {
			fail(w, http.StatusBadRequest, "no files uploaded")
			return
		}
		total := int64(0)
		for _, fh := range headers {
			name := path.Base(strings.ReplaceAll(fh.Filename, "\\", "/"))
			switch {
			case name == "." || name == "/" || name == "":
				fail(w, http.StatusBadRequest, "empty file name")
				return
			case !validExt(name):
				fail(w, http.StatusBadRequest, "unsupported extension: "+name)
				return
			case fh.Size > maxFile:
				fail(w, http.StatusRequestEntityTooLarge, name+" exceeds 1 MB")
				return
			}
			if _, dup := files[name]; dup {
				fail(w, http.StatusBadRequest, "duplicate file name: "+name)
				return
			}
			if total += fh.Size; total > maxTotal {
				fail(w, http.StatusRequestEntityTooLarge, "upload exceeds 10 MB")
				return
			}
			f, err := fh.Open()
			if err != nil {
				serverError(w, err)
				return
			}
			b, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				serverError(w, err)
				return
			}
			files[name] = string(b)
		}
	}
	b := make([]byte, 8)
	rand.Read(b)
	id := hex.EncodeToString(b)
	if err := saveProject(a.db, id, files, cobol.Analyze(files)); err != nil {
		serverError(w, err)
		return
	}
	p, err := loadProject(a.db, id)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *api) project(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !projectExists(a.db, id) {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	p, err := loadProject(a.db, id)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *api) file(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var content string
	err := a.db.QueryRow(`SELECT content FROM files WHERE project=? AND name=?`, r.PathValue("id"), name).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound, "file not found")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "lines": splitLines(content)})
}

type Ref struct {
	Kind      string `json:"kind"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Paragraph string `json:"paragraph"`
}
type Def struct {
	File string `json:"file"`
	Line int    `json:"line"`
}
type SymbolHit struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Def  Def    `json:"def"`
	Refs []Ref  `json:"refs"`
}
type LineHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

var ftsWord = regexp.MustCompile(`[A-Za-z0-9]+`)

const ftsSQL = `SELECT file, line, text FROM lines_fts
	WHERE lines_fts MATCH ? AND project=? ORDER BY file, CAST(line AS INTEGER) LIMIT ?`

// ftsQuery turns words into an FTS5 prefix query matching any of them.
func ftsQuery(words []string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = `"` + w + `"*`
	}
	return strings.Join(q, " OR ")
}

func (a *api) search(w http.ResponseWriter, r *http.Request) {
	id, q, kind := r.PathValue("id"), r.URL.Query().Get("q"), r.URL.Query().Get("kind")
	if !projectExists(a.db, id) {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	words := ftsWord.FindAllString(q, -1)
	if strings.TrimSpace(q) == "" || len(words) == 0 {
		fail(w, http.StatusBadRequest, "q is required")
		return
	}
	res := struct {
		Results []SymbolHit `json:"results"`
		Lines   []LineHit   `json:"lines"`
	}{[]SymbolHit{}, []LineHit{}}

	rows, err := a.db.Query(`SELECT name, kind, file, line FROM symbols
		WHERE project=? AND instr(upper(name), upper(?))>0 AND (?='' OR kind=?)
		ORDER BY name, file, line LIMIT ?`, id, q, kind, kind, maxHits)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		var s SymbolHit
		if err := rows.Scan(&s.Name, &s.Kind, &s.Def.File, &s.Def.Line); err != nil {
			rows.Close()
			serverError(w, err)
			return
		}
		s.Refs = []Ref{}
		res.Results = append(res.Results, s)
	}
	rows.Close()
	// ponytail: refs are matched by name only (not by qualifier); one query per hit, at most 50.
	for i := range res.Results {
		s := &res.Results[i]
		rr, err := a.db.Query(`SELECT kind, file, line, paragraph FROM refs
			WHERE project=? AND name=? AND kind IN ('read','write','io') ORDER BY file, line, kind`, id, s.Name)
		if err != nil {
			serverError(w, err)
			return
		}
		for rr.Next() {
			var x Ref
			if err := rr.Scan(&x.Kind, &x.File, &x.Line, &x.Paragraph); err != nil {
				rr.Close()
				serverError(w, err)
				return
			}
			s.Refs = append(s.Refs, x)
		}
		rr.Close()
	}

	lr, err := a.db.Query(ftsSQL, ftsQuery(words), id, maxHits)
	if err != nil {
		serverError(w, err)
		return
	}
	defer lr.Close()
	for lr.Next() {
		var l LineHit
		if err := lr.Scan(&l.File, &l.Line, &l.Text); err != nil {
			serverError(w, err)
			return
		}
		res.Lines = append(res.Lines, l)
	}
	writeJSON(w, http.StatusOK, res)
}
