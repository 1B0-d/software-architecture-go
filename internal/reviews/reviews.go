// Package reviews owns its SQLite database and validates a vehicle through the catalog service.
package reviews

import (
	"context"
	"database/sql"
	"encoding/json"
	"example.local/architecture-demo/internal/shared"
	"fmt"
	_ "modernc.org/sqlite"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Review struct {
	ID        int64  `json:"id"`
	VehicleID int64  `json:"vehicle_id"`
	Text      string `json:"text"`
}
type Repository interface {
	Create(context.Context, int64, string) (Review, error)
	List(context.Context, int64) ([]Review, error)
}
type SQLiteRepo struct{ db *sql.DB }

func Open(path string) (*SQLiteRepo, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS reviews(id INTEGER PRIMARY KEY AUTOINCREMENT,vehicle_id INTEGER NOT NULL,text TEXT NOT NULL)`); e != nil {
		db.Close()
		return nil, e
	}
	return &SQLiteRepo{db}, nil
}
func (r *SQLiteRepo) Close() error { return r.db.Close() }
func (r *SQLiteRepo) Create(ctx context.Context, id int64, text string) (Review, error) {
	res, e := r.db.ExecContext(ctx, `INSERT INTO reviews(vehicle_id,text) VALUES(?,?)`, id, text)
	if e != nil {
		return Review{}, e
	}
	n, e := res.LastInsertId()
	return Review{n, id, text}, e
}
func (r *SQLiteRepo) List(ctx context.Context, id int64) ([]Review, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT id,vehicle_id,text FROM reviews WHERE vehicle_id=? ORDER BY id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []Review{}
	for rows.Next() {
		var v Review
		if e = rows.Scan(&v.ID, &v.VehicleID, &v.Text); e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

type VehicleChecker interface {
	Exists(context.Context, int64) (bool, error)
}
type HTTPVehicleChecker struct {
	BaseURL string
	Client  *http.Client
}

func (c HTTPVehicleChecker) Exists(ctx context.Context, id int64) (bool, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/vehicles/%d", strings.TrimRight(c.BaseURL, "/"), id), nil)
	if e != nil {
		return false, e
	}
	cl := c.Client
	if cl == nil {
		cl = &http.Client{Timeout: 3 * time.Second}
	}
	res, e := cl.Do(req)
	if e != nil {
		return false, e
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return false, nil
	}
	if res.StatusCode != 200 {
		return false, fmt.Errorf("catalog returned %d", res.StatusCode)
	}
	return true, nil
}

type Service struct {
	Repo    Repository
	Catalog VehicleChecker
}

func (s Service) Create(ctx context.Context, id int64, text string) (Review, error) {
	text = strings.TrimSpace(text)
	if id < 1 || len(text) < 1 || len(text) > 1000 {
		return Review{}, fmt.Errorf("invalid review")
	}
	ok, e := s.Catalog.Exists(ctx, id)
	if e != nil {
		return Review{}, e
	}
	if !ok {
		return Review{}, ErrVehicleMissing
	}
	return s.Repo.Create(ctx, id, text)
}

var ErrVehicleMissing = fmt.Errorf("vehicle not found")

func Handler(repo Repository, checker VehicleChecker) http.Handler {
	s := Service{repo, checker}
	mux := http.NewServeMux()
	mux.HandleFunc("/reviews", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var in struct {
				VehicleID int64  `json:"vehicle_id"`
				Text      string `json:"text"`
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if dec.Decode(&in) != nil || in.VehicleID < 1 || len(strings.TrimSpace(in.Text)) < 1 || len(in.Text) > 1000 {
				shared.JSON(w, 400, map[string]string{"error": "invalid review"})
				return
			}
			v, e := s.Create(r.Context(), in.VehicleID, in.Text)
			if e == ErrVehicleMissing {
				shared.JSON(w, 404, map[string]string{"error": e.Error()})
				return
			}
			if e != nil {
				shared.JSON(w, 502, map[string]string{"error": "catalog or storage unavailable"})
				return
			}
			shared.JSON(w, 201, v)
		case http.MethodGet:
			id, e := strconv.ParseInt(r.URL.Query().Get("vehicle_id"), 10, 64)
			if e != nil || id < 1 {
				shared.JSON(w, 400, map[string]string{"error": "invalid vehicle ID"})
				return
			}
			v, e := s.Repo.List(r.Context(), id)
			if e != nil {
				shared.JSON(w, 500, map[string]string{"error": "database error"})
				return
			}
			shared.JSON(w, 200, v)
		default:
			w.Header().Set("Allow", "GET, POST")
			shared.JSON(w, 405, map[string]string{"error": "method not allowed"})
		}
	})
	return shared.Middleware(mux)
}
