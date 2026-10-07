package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed web/* seed.json
var embedded embed.FS

type Entry struct {
	ID            int64  `json:"id"`
	Building      string `json:"building"`
	Floor         string `json:"floor"`
	Department    string `json:"department"`
	InternalPhone string `json:"internal_phone"`
	ExternalPhone string `json:"external_phone"`
	SortOrder     int    `json:"sort_order"`
}

type App struct{ db *sql.DB }

func main() {
	exe, _ := os.Executable()
	dataDir := filepath.Join(filepath.Dir(exe), "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "phonebook.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db}
	if err := app.migrate(); err != nil {
		log.Fatal(err)
	}
	if err := app.seedIfEmpty(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/entries", app.entriesHandler)
	mux.HandleFunc("/api/entries/", app.entryHandler)
	mux.HandleFunc("/api/meta", app.metaHandler)
	mux.HandleFunc("/api/backup", app.backupHandler)
	mux.HandleFunc("/api/import", app.importHandler)
	mux.HandleFunc("/api/stats", app.statsHandler)

	staticFS, err := fs.Sub(embedded, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", spaHandler(staticFS))

	addr := "127.0.0.1:8787"
	url := "http://" + addr
	log.Printf("BGH PhoneBook running at %s", url)
	go func() {
		time.Sleep(700 * time.Millisecond)
		_ = openBrowser(url)
	}()
	log.Fatal(http.ListenAndServe(addr, logRequest(mux)))
}

func (a *App) migrate() error {
	_, err := a.db.Exec(`
        CREATE TABLE IF NOT EXISTS entries (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            building TEXT NOT NULL,
            floor TEXT NOT NULL,
            department TEXT NOT NULL,
            internal_phone TEXT NOT NULL DEFAULT '',
            external_phone TEXT NOT NULL DEFAULT '',
            sort_order INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE INDEX IF NOT EXISTS idx_entries_building ON entries(building);
        CREATE INDEX IF NOT EXISTS idx_entries_floor ON entries(floor);
        CREATE INDEX IF NOT EXISTS idx_entries_department ON entries(department);
    `)
	return err
}

func (a *App) seedIfEmpty() error {
	var count int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	b, err := embedded.ReadFile("seed.json")
	if err != nil {
		return err
	}
	var items []Entry
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO entries(building,floor,department,internal_phone,external_phone,sort_order) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, e := range items {
		if _, err := stmt.Exec(e.Building, e.Floor, e.Department, e.InternalPhone, e.ExternalPhone, e.SortOrder); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (a *App) entriesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listEntries(w, r)
	case http.MethodPost:
		a.createEntry(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) entryHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/entries/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		a.updateEntry(w, r, id)
	case http.MethodDelete:
		a.deleteEntry(w, id)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (a *App) listEntries(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	building := strings.TrimSpace(r.URL.Query().Get("building"))
	floor := strings.TrimSpace(r.URL.Query().Get("floor"))
	args := []any{}
	where := []string{"1=1"}
	if building != "" && !strings.EqualFold(building, "all") && building != "ทั้งหมด" {
		where = append(where, "building = ?")
		args = append(args, building)
	}
	if floor != "" && !strings.EqualFold(floor, "all") && floor != "ทั้งหมด" {
		where = append(where, "floor = ?")
		args = append(args, floor)
	}
	if q != "" {
		where = append(where, "(building LIKE ? OR floor LIKE ? OR department LIKE ? OR internal_phone LIKE ? OR external_phone LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like)
	}
	sqlText := `SELECT id,building,floor,department,internal_phone,external_phone,sort_order FROM entries WHERE ` + strings.Join(where, " AND ") + ` ORDER BY sort_order,id`
	rows, err := a.db.Query(sqlText, args...)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Building, &e.Floor, &e.Department, &e.InternalPhone, &e.ExternalPhone, &e.SortOrder); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		items = append(items, e)
	}
	writeJSON(w, 200, items)
}

func decodeEntry(r *http.Request) (Entry, error) {
	var e Entry
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err := dec.Decode(&e); err != nil {
		return e, err
	}
	e.Building = strings.TrimSpace(e.Building)
	e.Floor = strings.TrimSpace(e.Floor)
	e.Department = strings.TrimSpace(e.Department)
	e.InternalPhone = strings.TrimSpace(e.InternalPhone)
	e.ExternalPhone = strings.TrimSpace(e.ExternalPhone)
	if e.Building == "" || e.Floor == "" || e.Department == "" {
		return e, errors.New("building, floor and department are required")
	}
	return e, nil
}

func (a *App) createEntry(w http.ResponseWriter, r *http.Request) {
	e, err := decodeEntry(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if e.SortOrder == 0 {
		_ = a.db.QueryRow("SELECT COALESCE(MAX(sort_order),0)+1 FROM entries").Scan(&e.SortOrder)
	}
	res, err := a.db.Exec(`INSERT INTO entries(building,floor,department,internal_phone,external_phone,sort_order) VALUES(?,?,?,?,?,?)`, e.Building, e.Floor, e.Department, e.InternalPhone, e.ExternalPhone, e.SortOrder)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	e.ID, _ = res.LastInsertId()
	writeJSON(w, 201, e)
}

func (a *App) updateEntry(w http.ResponseWriter, r *http.Request, id int64) {
	e, err := decodeEntry(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if e.SortOrder == 0 {
		e.SortOrder = int(id)
	}
	res, err := a.db.Exec(`UPDATE entries SET building=?,floor=?,department=?,internal_phone=?,external_phone=?,sort_order=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, e.Building, e.Floor, e.Department, e.InternalPhone, e.ExternalPhone, e.SortOrder, id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "not found")
		return
	}
	e.ID = id
	writeJSON(w, 200, e)
}

func (a *App) deleteEntry(w http.ResponseWriter, id int64) {
	res, err := a.db.Exec("DELETE FROM entries WHERE id=?", id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) metaHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT building,COUNT(*) FROM entries GROUP BY building ORDER BY MIN(sort_order),building`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type B struct {
		Building string `json:"building"`
		Count    int    `json:"count"`
	}
	out := []B{}
	for rows.Next() {
		var b B
		if err := rows.Scan(&b.Building, &b.Count); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		out = append(out, b)
	}
	writeJSON(w, 200, out)
}

func (a *App) backupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}
	rows, err := a.db.Query(`SELECT id,building,floor,department,internal_phone,external_phone,sort_order FROM entries ORDER BY sort_order,id`)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Building, &e.Floor, &e.Department, &e.InternalPhone, &e.ExternalPhone, &e.SortOrder); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		items = append(items, e)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="bgh-phonebook-backup.json"`)
	writeJSON(w, 200, items)
}

type ImportPayload struct {
	Mode  string  `json:"mode"` // "append" or "replace"
	Items []Entry `json:"items"`
}

func (a *App) importHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload ImportPayload
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20))
	if err := dec.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
		return
	}
	if len(payload.Items) == 0 {
		writeError(w, http.StatusBadRequest, "no items to import")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	if payload.Mode == "replace" {
		if _, err := tx.Exec("DELETE FROM entries"); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	var maxSort int
	_ = tx.QueryRow("SELECT COALESCE(MAX(sort_order),0) FROM entries").Scan(&maxSort)

	stmt, err := tx.Prepare(`INSERT INTO entries(building,floor,department,internal_phone,external_phone,sort_order) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer stmt.Close()

	importedCount := 0
	for _, e := range payload.Items {
		building := strings.TrimSpace(e.Building)
		floor := strings.TrimSpace(e.Floor)
		dept := strings.TrimSpace(e.Department)
		internal := strings.TrimSpace(e.InternalPhone)
		external := strings.TrimSpace(e.ExternalPhone)

		if building == "" || floor == "" || dept == "" {
			continue
		}

		sortOrder := e.SortOrder
		if sortOrder <= 0 {
			maxSort++
			sortOrder = maxSort
		} else if sortOrder > maxSort {
			maxSort = sortOrder
		}

		if _, err := stmt.Exec(building, floor, dept, internal, external, sortOrder); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		importedCount++
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"imported": importedCount,
	})
}

func (a *App) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var totalEntries, totalBuildings, totalFloors int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&totalEntries)
	_ = a.db.QueryRow("SELECT COUNT(DISTINCT building) FROM entries").Scan(&totalBuildings)
	_ = a.db.QueryRow("SELECT COUNT(DISTINCT floor) FROM entries").Scan(&totalFloors)
	writeJSON(w, http.StatusOK, map[string]int{
		"total_entries":   totalEntries,
		"total_buildings": totalBuildings,
		"total_floors":    totalFloors,
	})
}

func spaHandler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fsys, path); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		b, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

var _ = fmt.Sprintf
