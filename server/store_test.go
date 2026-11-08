package main

import (
	"testing"
	"testing/fstest"
	"time"
)

func TestDeleteExpired(t *testing.T) {
	db, err := openDB(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h := newHandler(fstest.MapFS{}, db, nil)
	old, fresh := samplesProject(t, h), samplesProject(t, h)
	if _, err := db.Exec(`UPDATE projects SET created_at = datetime('now','-10 days') WHERE id=?`, old.ID); err != nil {
		t.Fatal(err)
	}
	count := func(table, id string) int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+map[bool]string{true: "id", false: "project"}[table == "projects"]+`=?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	tables := []string{"projects", "files", "symbols", "refs", "edges", "warnings", "lines_fts"}

	if n, err := deleteExpired(db, 0); err != nil || n != 0 || count("files", old.ID) == 0 {
		t.Fatalf("ttl 0 must delete nothing: %d %v", n, err)
	}
	if n, err := deleteExpired(db, 168*time.Hour); err != nil || n != 1 {
		t.Fatalf("deleted %d, %v", n, err)
	}
	for _, tb := range tables {
		if c := count(tb, old.ID); c != 0 {
			t.Errorf("%s still has %d rows of expired project", tb, c)
		}
		if c := count(tb, fresh.ID); tb != "refs" && tb != "warnings" && c == 0 {
			t.Errorf("%s lost rows of fresh project", tb)
		}
	}
	if code := do(t, h, "GET", "/api/projects/"+old.ID, "", nil, nil); code != 404 {
		t.Fatalf("expired project: %d", code)
	}
	if code := do(t, h, "GET", "/api/projects/"+fresh.ID, "", nil, nil); code != 200 {
		t.Fatalf("fresh project: %d", code)
	}
	if code := do(t, h, "GET", "/api/projects/"+fresh.ID+"/search?q=PERFORM", "", nil, nil); code != 200 {
		t.Fatalf("fresh search: %d", code)
	}
}
