package jobs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	Status    Status  `json:"status"`
	Progress  float64 `json:"progress"`
	Message   string  `json:"message"`
	Report    any     `json:"report,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Service struct {
	db *sql.DB
	mu sync.Mutex
}

func New(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(jobType string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`INSERT INTO jobs (type, status, progress, message, report, created_at, updated_at) VALUES (?, ?, 0, '', '', ?, ?)`,
		jobType, StatusQueued, now, now)
	if err != nil {
		return Job{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Job{}, err
	}
	return Job{ID: id, Type: jobType, Status: StatusQueued, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Service) Get(id int64) (Job, error) {
	var j Job
	var report string
	err := s.db.QueryRow(`SELECT id, type, status, progress, message, report, created_at, updated_at FROM jobs WHERE id=?`, id).
		Scan(&j.ID, &j.Type, &j.Status, &j.Progress, &j.Message, &report, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return Job{}, err
	}
	if report != "" {
		_ = json.Unmarshal([]byte(report), &j.Report)
	}
	return j, nil
}

func (s *Service) List(limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, type, status, progress, message, report, created_at, updated_at FROM jobs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		var report string
		if err := rows.Scan(&j.ID, &j.Type, &j.Status, &j.Progress, &j.Message, &report, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		if report != "" {
			_ = json.Unmarshal([]byte(report), &j.Report)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Service) Update(id int64, status Status, progress float64, message string, report any) error {
	raw := ""
	if report != nil {
		b, err := json.Marshal(report)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE jobs SET status=?, progress=?, message=?, report=?, updated_at=? WHERE id=?`,
		status, progress, message, raw, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %d not found", id)
	}
	return nil
}
