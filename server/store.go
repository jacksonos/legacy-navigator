package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jacksonos/legacy-navigator/server/cobol"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS files(project TEXT NOT NULL, name TEXT NOT NULL, content TEXT NOT NULL, PRIMARY KEY(project, name));
CREATE TABLE IF NOT EXISTS symbols(project TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL, file TEXT NOT NULL, line INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS symbols_project ON symbols(project, name);
CREATE TABLE IF NOT EXISTS refs(project TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL, file TEXT NOT NULL, line INTEGER NOT NULL, paragraph TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS refs_project ON refs(project, name);
CREATE TABLE IF NOT EXISTS edges(project TEXT NOT NULL, from_name TEXT NOT NULL, to_name TEXT NOT NULL, kind TEXT NOT NULL, file TEXT NOT NULL, line INTEGER NOT NULL, uncertain INTEGER NOT NULL, dynamic INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS edges_project ON edges(project);
CREATE TABLE IF NOT EXISTS warnings(project TEXT NOT NULL, kind TEXT NOT NULL, detail TEXT NOT NULL, file TEXT NOT NULL, line INTEGER NOT NULL, program_file TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS warnings_project ON warnings(project);
CREATE VIRTUAL TABLE IF NOT EXISTS lines_fts USING fts5(project UNINDEXED, file UNINDEXED, line UNINDEXED, text);
`

func openDB(path string) (*sql.DB, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// ponytail: single connection = single writer, no SQLITE_BUSY; revisit if concurrency matters.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

var programIDLine = regexp.MustCompile(`(?i)\bPROGRAM-ID\b`)

// saveProject stores the sources and everything the parser found, in one transaction.
func saveProject(db *sql.DB, id string, files map[string]string, proj *cobol.Project) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	exec := func(q string, args ...any) {
		if err == nil {
			_, err = tx.Exec(q, args...)
		}
	}
	exec(`INSERT INTO projects(id, created_at) VALUES(?, datetime('now'))`, id)
	for name, content := range files {
		exec(`INSERT INTO files VALUES(?,?,?)`, id, name, content)
		for i, l := range splitLines(content) {
			exec(`INSERT INTO lines_fts(project, file, line, text) VALUES(?,?,?,?)`, id, name, i+1, l)
		}
	}
	seen := map[[5]any]bool{}
	sym := func(name, kind, file string, line int) {
		k := [5]any{"s", name, kind, file, line}
		if !seen[k] { // a copybook expanded in several programs repeats its items
			seen[k] = true
			exec(`INSERT INTO symbols VALUES(?,?,?,?,?)`, id, name, kind, file, line)
		}
	}
	warn := func(w cobol.Warning, programFile string) {
		exec(`INSERT INTO warnings VALUES(?,?,?,?,?,?)`, id, w.Kind, w.Detail, w.Loc.File, w.Loc.Line, programFile)
	}
	for _, w := range proj.Warnings {
		warn(w, "")
	}
	for _, p := range proj.Programs {
		for i, l := range splitLines(files[p.File]) {
			if programIDLine.MatchString(l) {
				sym(p.ID, "program", p.File, i+1)
				break
			}
		}
		for _, d := range p.Data {
			sym(d.Name, "data", d.Loc.File, d.Loc.Line)
		}
		for _, b := range p.Sections {
			sym(b.Name, "section", b.Loc.File, b.Loc.Line)
		}
		for _, b := range p.Paragraphs {
			sym(b.Name, "paragraph", b.Loc.File, b.Loc.Line)
		}
		for _, f := range p.Files {
			sym(f.Name, "file", f.Loc.File, f.Loc.Line)
		}
		for _, r := range p.Refs {
			k := [5]any{"r", r.Name, r.Kind, r.Loc.File, r.Loc.Line}
			if r.Kind == "def" || seen[k] {
				continue
			}
			seen[k] = true
			exec(`INSERT INTO refs VALUES(?,?,?,?,?,?)`, id, r.Name, r.Kind, r.Loc.File, r.Loc.Line, r.Paragraph)
		}
		for _, e := range p.Edges {
			exec(`INSERT INTO edges VALUES(?,?,?,?,?,?,?,?)`, id, e.From, e.To, e.Kind, e.Loc.File, e.Loc.Line, e.Uncertain, e.Dynamic)
		}
		for _, w := range p.Warnings {
			warn(w, p.File)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func projectExists(db *sql.DB, id string) bool {
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM projects WHERE id=?`, id).Scan(&n)
	return n > 0
}

type Warning struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}
type FileSummary struct {
	Name       string    `json:"name"`
	Lines      int       `json:"lines"`
	ProgramID  string    `json:"programId"`
	Paragraphs int       `json:"paragraphs"`
	DataItems  int       `json:"dataItems"`
	Warnings   []Warning `json:"warnings"`
}
type Project struct {
	ID       string        `json:"id"`
	Files    []FileSummary `json:"files"`
	Warnings []Warning     `json:"warnings"`
}

func loadProject(db *sql.DB, id string) (Project, error) {
	out := Project{ID: id, Files: []FileSummary{}, Warnings: []Warning{}}
	byFile := map[string]*FileSummary{}
	rows, err := db.Query(`SELECT name, content FROM files WHERE project=? ORDER BY name`, id)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var n, c string
		if err := rows.Scan(&n, &c); err != nil {
			rows.Close()
			return out, err
		}
		out.Files = append(out.Files, FileSummary{Name: n, Lines: len(splitLines(c)), Warnings: []Warning{}})
	}
	rows.Close()
	for i := range out.Files {
		byFile[out.Files[i].Name] = &out.Files[i]
	}
	rows, err = db.Query(`SELECT file, kind, COUNT(*) FROM symbols WHERE project=? AND kind IN ('program','paragraph','data') GROUP BY file, kind`, id)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var file, kind string
		var n int
		if err := rows.Scan(&file, &kind, &n); err != nil {
			rows.Close()
			return out, err
		}
		if f := byFile[file]; f != nil {
			switch kind {
			case "paragraph":
				f.Paragraphs = n
			case "data":
				f.DataItems = n
			}
		}
	}
	rows.Close()
	rows, err = db.Query(`SELECT file, name FROM symbols WHERE project=? AND kind='program'`, id)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var file, name string
		if err := rows.Scan(&file, &name); err != nil {
			rows.Close()
			return out, err
		}
		if f := byFile[file]; f != nil {
			f.ProgramID = name
		}
	}
	rows.Close()
	rows, err = db.Query(`SELECT DISTINCT kind, detail, file, line FROM warnings WHERE project=? ORDER BY file, line, kind`, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var w Warning
		if err := rows.Scan(&w.Kind, &w.Detail, &w.File, &w.Line); err != nil {
			return out, err
		}
		if f := byFile[w.File]; f != nil {
			f.Warnings = append(f.Warnings, w)
		} else {
			out.Warnings = append(out.Warnings, w)
		}
	}
	return out, rows.Err()
}
