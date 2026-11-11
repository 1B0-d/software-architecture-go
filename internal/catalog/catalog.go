// Package catalog demonstrates controller, application service, repository port and SQLite adapter.
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"example.local/architecture-demo/internal/shared"
	"html/template"
	_ "modernc.org/sqlite"
	"net/http"
	"strconv"
	"strings"
)

type Vehicle struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Repository interface {
	Create(context.Context, string) (Vehicle, error)
	Get(context.Context, int64) (Vehicle, error)
	List(context.Context) ([]Vehicle, error)
}
type SQLiteRepo struct{ db *sql.DB }

func Open(path string) (*SQLiteRepo, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS vehicles(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL)`); e != nil {
		db.Close()
		return nil, e
	}
	return &SQLiteRepo{db}, nil
}
func (r *SQLiteRepo) Close() error { return r.db.Close() }
func (r *SQLiteRepo) Create(ctx context.Context, name string) (Vehicle, error) {
	res, e := r.db.ExecContext(ctx, `INSERT INTO vehicles(name) VALUES(?)`, name)
	if e != nil {
		return Vehicle{}, e
	}
	id, e := res.LastInsertId()
	return Vehicle{id, name}, e
}
func (r *SQLiteRepo) Get(ctx context.Context, id int64) (Vehicle, error) {
	var v Vehicle
	e := r.db.QueryRowContext(ctx, `SELECT id,name FROM vehicles WHERE id=?`, id).Scan(&v.ID, &v.Name)
	return v, e
}
func (r *SQLiteRepo) List(ctx context.Context) ([]Vehicle, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT id,name FROM vehicles ORDER BY id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Vehicle{}
	for rows.Next() {
		var v Vehicle
		if e = rows.Scan(&v.ID, &v.Name); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

type Service struct{ Repo Repository }

var ErrInvalidName = errors.New("name must contain 1 to 100 bytes")

func (s Service) Create(ctx context.Context, name string) (Vehicle, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 100 {
		return Vehicle{}, ErrInvalidName
	}
	return s.Repo.Create(ctx, name)
}

var vehicleView = template.Must(template.New("vehicles").Parse(`<!doctype html><html><head><title>Vehicle catalogue</title></head><body><h1>Vehicles</h1><ul>{{range .}}<li>{{.ID}}: {{.Name}}</li>{{else}}<li>No vehicles</li>{{end}}</ul></body></html>`))

func Handler(repo Repository) http.Handler {
	s := Service{repo}
	mux := http.NewServeMux()
	mux.HandleFunc("/vehicles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			v, e := s.Repo.List(r.Context())
			if e != nil {
				shared.JSON(w, 500, map[string]string{"error": "database error"})
				return
			}
			shared.JSON(w, 200, v)
		case http.MethodPost:
			var in struct {
				Name string `json:"name"`
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if dec.Decode(&in) != nil {
				shared.JSON(w, 400, map[string]string{"error": "invalid JSON"})
				return
			}
			v, e := s.Create(r.Context(), in.Name)
			if e != nil {
				if errors.Is(e, ErrInvalidName) {
					shared.JSON(w, 400, map[string]string{"error": e.Error()})
				} else {
					shared.JSON(w, 500, map[string]string{"error": "database error"})
				}
				return
			}
			shared.JSON(w, 201, v)
		default:
			w.Header().Set("Allow", "GET, POST")
			shared.JSON(w, 405, map[string]string{"error": "method not allowed"})
		}
	})
	mux.HandleFunc("/vehicles/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			shared.JSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		id, e := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/vehicles/"), 10, 64)
		if e != nil || id < 1 {
			shared.JSON(w, 400, map[string]string{"error": "invalid ID"})
			return
		}
		v, e := s.Repo.Get(r.Context(), id)
		if errors.Is(e, sql.ErrNoRows) {
			shared.JSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		if e != nil {
			shared.JSON(w, 500, map[string]string{"error": "database error"})
			return
		}
		shared.JSON(w, 200, v)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		vehicles, err := s.Repo.List(r.Context())
		if err != nil {
			http.Error(w, "database error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = vehicleView.Execute(w, vehicles)
	})
	return shared.Middleware(mux)
}
