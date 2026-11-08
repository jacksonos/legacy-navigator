package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jacksonos/legacy-navigator/server/llm"
)

//go:embed all:static
var staticFS embed.FS

func newHandler(static fs.FS, db *sql.DB, providers []llm.Provider) http.Handler {
	mux := http.NewServeMux()
	(&api{db, providers}).register(mux)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	files := http.FileServerFS(static)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusServiceUnavailable)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if st, err := fs.Stat(static, name); name != "" && err == nil && !st.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
	return mux
}

func main() {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/navigator.db"
	}
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ttl := 168 * time.Hour
	if v := os.Getenv("PROJECT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			ttl = d
		} else {
			log.Printf("invalid PROJECT_TTL %q, using %s", v, ttl)
		}
	}
	if ttl > 0 {
		go func() {
			tick := time.NewTicker(time.Hour)
			for {
				if n, err := deleteExpired(db, ttl); err != nil {
					log.Printf("expiry: %v", err)
				} else if n > 0 {
					log.Printf("expiry: deleted %d projects", n)
				}
				<-tick.C
			}
		}()
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, newHandler(sub, db, llm.FromEnv())))
}
