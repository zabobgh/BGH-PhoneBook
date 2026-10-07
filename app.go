package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type Entry struct {
	ID            int64  `json:"id"`
	Building      string `json:"building"`
	Floor         string `json:"floor"`
	Department    string `json:"department"`
	InternalPhone string `json:"internal_phone"`
	ExternalPhone string `json:"external_phone"`
	SortOrder     int    `json:"sort_order"`
}

type BuildingMeta struct {
	Building string `json:"building"`
	Count    int    `json:"count"`
}

type Stats struct {
	TotalEntries   int `json:"total_entries"`
	TotalBuildings int `json:"total_buildings"`
	TotalFloors    int `json:"total_floors"`
}

type ImportPayload struct {
	Mode  string  `json:"mode"`
	Items []Entry `json:"items"`
}

type App struct {
	ctx context.Context
	db  *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.initDB(); err != nil {
		log.Printf("Error initializing database: %v", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		_ = a.db.Close()
	}
}

func (a *App) initDB() error {
	baseDir := "."
	exe, err := os.Executable()
	if err == nil {
		exeLower := strings.ToLower(exe)
		if !strings.Contains(exeLower, "temp") && !strings.Contains(exeLower, "tmp") {
			baseDir = filepath.Dir(exe)
		}
	}
	dataDir := filepath.Join(baseDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	dbPath := filepath.Join(dataDir, "phonebook.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	a.db = db

	if err := a.migrate(); err != nil {
		return err
	}
	return a.seedIfEmpty()
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
	if err != nil {
		return err
	}
	_, _ = a.db.Exec(`UPDATE entries SET internal_phone = '' WHERE internal_phone = department AND internal_phone != ''`)
	return nil
}

func (a *App) seedIfEmpty() error {
	var count int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var items []Entry
	if err := json.Unmarshal(seedJSON, &items); err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO entries(building,floor,department,internal_phone,external_phone,sort_order) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, e := range items {
		if _, err := stmt.Exec(e.Building, e.Floor, e.Department, e.InternalPhone, e.ExternalPhone, e.SortOrder); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ==================== Wails Exposed Go Methods ====================

func (a *App) ListEntries(q, building, floor string) ([]Entry, error) {
	if a.db == nil {
		return nil, errors.New("database not initialized")
	}
	q = strings.TrimSpace(q)
	building = strings.TrimSpace(building)
	floor = strings.TrimSpace(floor)
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
		words := strings.Fields(q)
		for _, w := range words {
			like := "%" + w + "%"
			cleanDigits := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, w)

			if len(cleanDigits) >= 2 {
				cleanLike := "%" + cleanDigits + "%"
				where = append(where, "(building LIKE ? OR floor LIKE ? OR department LIKE ? OR internal_phone LIKE ? OR external_phone LIKE ? OR REPLACE(REPLACE(internal_phone, '-', ''), ' ', '') LIKE ? OR REPLACE(REPLACE(external_phone, '-', ''), ' ', '') LIKE ?)")
				args = append(args, like, like, like, like, like, cleanLike, cleanLike)
			} else {
				where = append(where, "(building LIKE ? OR floor LIKE ? OR department LIKE ? OR internal_phone LIKE ? OR external_phone LIKE ?)")
				args = append(args, like, like, like, like, like)
			}
		}
	}

	sqlText := `SELECT id,building,floor,department,internal_phone,external_phone,sort_order FROM entries WHERE ` + strings.Join(where, " AND ") + ` ORDER BY sort_order,id`
	rows, err := a.db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Building, &e.Floor, &e.Department, &e.InternalPhone, &e.ExternalPhone, &e.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

func (a *App) CreateEntry(e Entry) (Entry, error) {
	if a.db == nil {
		return e, errors.New("database not initialized")
	}
	e.Building = strings.TrimSpace(e.Building)
	e.Floor = strings.TrimSpace(e.Floor)
	e.Department = strings.TrimSpace(e.Department)
	e.InternalPhone = strings.TrimSpace(e.InternalPhone)
	e.ExternalPhone = strings.TrimSpace(e.ExternalPhone)

	if e.Building == "" || e.Floor == "" || e.Department == "" {
		return e, errors.New("building, floor and department are required")
	}
	if e.SortOrder == 0 {
		_ = a.db.QueryRow("SELECT COALESCE(MAX(sort_order),0)+1 FROM entries").Scan(&e.SortOrder)
	}
	res, err := a.db.Exec(`INSERT INTO entries(building,floor,department,internal_phone,external_phone,sort_order) VALUES(?,?,?,?,?,?)`, e.Building, e.Floor, e.Department, e.InternalPhone, e.ExternalPhone, e.SortOrder)
	if err != nil {
		return e, err
	}
	e.ID, _ = res.LastInsertId()
	return e, nil
}

func (a *App) UpdateEntry(e Entry) (Entry, error) {
	if a.db == nil {
		return e, errors.New("database not initialized")
	}
	if e.ID <= 0 {
		return e, errors.New("invalid entry ID")
	}
	e.Building = strings.TrimSpace(e.Building)
	e.Floor = strings.TrimSpace(e.Floor)
	e.Department = strings.TrimSpace(e.Department)
	e.InternalPhone = strings.TrimSpace(e.InternalPhone)
	e.ExternalPhone = strings.TrimSpace(e.ExternalPhone)

	if e.Building == "" || e.Floor == "" || e.Department == "" {
		return e, errors.New("building, floor and department are required")
	}
	if e.SortOrder == 0 {
		e.SortOrder = int(e.ID)
	}

	res, err := a.db.Exec(`UPDATE entries SET building=?,floor=?,department=?,internal_phone=?,external_phone=?,sort_order=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, e.Building, e.Floor, e.Department, e.InternalPhone, e.ExternalPhone, e.SortOrder, e.ID)
	if err != nil {
		return e, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return e, errors.New("entry not found")
	}
	return e, nil
}

func (a *App) DeleteEntry(id int64) error {
	if a.db == nil {
		return errors.New("database not initialized")
	}
	res, err := a.db.Exec("DELETE FROM entries WHERE id=?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("entry not found")
	}
	return nil
}

func (a *App) GetMeta() ([]BuildingMeta, error) {
	if a.db == nil {
		return nil, errors.New("database not initialized")
	}
	rows, err := a.db.Query(`SELECT building,COUNT(*) FROM entries GROUP BY building ORDER BY MIN(sort_order),building`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []BuildingMeta{}
	for rows.Next() {
		var b BuildingMeta
		if err := rows.Scan(&b.Building, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (a *App) GetStats() (Stats, error) {
	var s Stats
	if a.db == nil {
		return s, errors.New("database not initialized")
	}
	_ = a.db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&s.TotalEntries)
	_ = a.db.QueryRow("SELECT COUNT(DISTINCT building) FROM entries").Scan(&s.TotalBuildings)
	_ = a.db.QueryRow("SELECT COUNT(DISTINCT floor) FROM entries").Scan(&s.TotalFloors)
	return s, nil
}

func (a *App) ImportBatch(mode string, items []Entry) (int, error) {
	if a.db == nil {
		return 0, errors.New("database not initialized")
	}
	if len(items) == 0 {
		return 0, errors.New("no items to import")
	}

	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if mode == "replace" {
		if _, err := tx.Exec("DELETE FROM entries"); err != nil {
			return 0, err
		}
	}

	var maxSort int
	_ = tx.QueryRow("SELECT COALESCE(MAX(sort_order),0) FROM entries").Scan(&maxSort)

	stmt, err := tx.Prepare(`INSERT INTO entries(building,floor,department,internal_phone,external_phone,sort_order) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, e := range items {
		b := strings.TrimSpace(e.Building)
		f := strings.TrimSpace(e.Floor)
		d := strings.TrimSpace(e.Department)
		ip := strings.TrimSpace(e.InternalPhone)
		ep := strings.TrimSpace(e.ExternalPhone)
		if b == "" || f == "" || d == "" {
			continue
		}
		sortOrder := e.SortOrder
		if sortOrder <= 0 {
			maxSort++
			sortOrder = maxSort
		} else if sortOrder > maxSort {
			maxSort = sortOrder
		}
		if _, err := stmt.Exec(b, f, d, ip, ep, sortOrder); err != nil {
			return 0, err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (a *App) BackupJSON() ([]Entry, error) {
	return a.ListEntries("", "", "")
}

func (a *App) RelocateEntries(ids []int64, targetBuilding, targetFloor string) (int, error) {
	if a.db == nil {
		return 0, errors.New("database not initialized")
	}
	targetBuilding = strings.TrimSpace(targetBuilding)
	targetFloor = strings.TrimSpace(targetFloor)
	if targetBuilding == "" || targetFloor == "" {
		return 0, errors.New("target building and floor are required")
	}
	if len(ids) == 0 {
		return 0, errors.New("no entries selected")
	}

	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE entries SET building=?, floor=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	updatedCount := 0
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		res, err := stmt.Exec(targetBuilding, targetFloor, id)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			updatedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updatedCount, nil
}

func (a *App) RenameBuilding(oldBuilding, newBuilding string) (int64, error) {
	if a.db == nil {
		return 0, errors.New("database not initialized")
	}
	oldBuilding = strings.TrimSpace(oldBuilding)
	newBuilding = strings.TrimSpace(newBuilding)
	if oldBuilding == "" || newBuilding == "" {
		return 0, errors.New("invalid building names")
	}
	res, err := a.db.Exec("UPDATE entries SET building = ? WHERE building = ?", newBuilding, oldBuilding)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (a *App) DeleteBuilding(building string) (int64, error) {
	if a.db == nil {
		return 0, errors.New("database not initialized")
	}
	building = strings.TrimSpace(building)
	if building == "" {
		return 0, errors.New("invalid building name")
	}
	var count int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM entries WHERE building = ?", building).Scan(&count); err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, fmt.Errorf("cannot delete building with %d entries", count)
	}
	return 0, nil
}

// ==================== HTTP Server & AssetServer Integration ====================

func (a *App) HttpHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/buildings/rename", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, 405, "method not allowed")
			return
		}
		var payload struct {
			OldBuilding string `json:"old_building"`
			NewBuilding string `json:"new_building"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		count, err := a.RenameBuilding(payload.OldBuilding, payload.NewBuilding)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"success": true, "updatedEntries": count, "updatedLocations": 0})
	})

	mux.HandleFunc("/api/buildings/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, 405, "method not allowed")
			return
		}
		var payload struct {
			Building string `json:"building"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		_, err := a.DeleteBuilding(payload.Building)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"success": true})
	})

	mux.HandleFunc("/api/entries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query().Get("q")
			building := r.URL.Query().Get("building")
			floor := r.URL.Query().Get("floor")
			items, err := a.ListEntries(q, building, floor)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, items)
		case http.MethodPost:
			var e Entry
			if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
				writeError(w, 400, err.Error())
				return
			}
			created, err := a.CreateEntry(e)
			if err != nil {
				writeError(w, 400, err.Error())
				return
			}
			writeJSON(w, 201, created)
		default:
			writeError(w, 405, "method not allowed")
		}
	})

	mux.HandleFunc("/api/entries/relocate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, 405, "method not allowed")
			return
		}
		var payload struct {
			IDs            []int64 `json:"ids"`
			TargetBuilding string  `json:"target_building"`
			TargetFloor    string  `json:"target_floor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		count, err := a.RelocateEntries(payload.IDs, payload.TargetBuilding, payload.TargetFloor)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"success": true, "count": count})
	})

	mux.HandleFunc("/api/entries/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/entries/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, 400, "invalid id")
			return
		}
		switch r.Method {
		case http.MethodPut:
			var e Entry
			if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
				writeError(w, 400, err.Error())
				return
			}
			e.ID = id
			updated, err := a.UpdateEntry(e)
			if err != nil {
				writeError(w, 400, err.Error())
				return
			}
			writeJSON(w, 200, updated)
		case http.MethodDelete:
			if err := a.DeleteEntry(id); err != nil {
				writeError(w, 400, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, 405, "method not allowed")
		}
	})

	mux.HandleFunc("/api/meta", func(w http.ResponseWriter, r *http.Request) {
		meta, err := a.GetMeta()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, meta)
	})

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := a.GetStats()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, stats)
	})

	mux.HandleFunc("/api/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, 405, "method not allowed")
			return
		}
		var payload ImportPayload
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20)).Decode(&payload); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		count, err := a.ImportBatch(payload.Mode, payload.Items)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"success": true, "imported": count})
	})

	mux.HandleFunc("/api/backup", func(w http.ResponseWriter, r *http.Request) {
		items, err := a.BackupJSON()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="bgh-phonebook-backup.json"`)
		writeJSON(w, 200, items)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
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
