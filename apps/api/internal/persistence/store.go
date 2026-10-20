// Package persistence implements transactional aggregate storage. MySQL uses
// explicit ownership columns, foreign keys and recurrence uniqueness, with JSON
// snapshots for evolving fields. All writers coordinate through a DB advisory
// lock. This intentionally simple first version loads aggregates per transaction;
// replace scans with scoped SQL before large scale or multiple busy offices.
package persistence

import (
	"context"
	"database/sql"
	"docping/api/internal/domain"
	"docping/api/migrations"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu    sync.Mutex
	file  string
	db    *sql.DB
	state *domain.State
}

func Open(driver, dsn, path, env string) (*Store, error) {
	s := &Store{state: domain.NewState()}
	switch driver {
	case "mysql":
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(10)
		db.SetConnMaxLifetime(3 * time.Minute)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err = db.PingContext(ctx); err != nil {
			return nil, err
		}
		for _, q := range strings.Split(migrations.SQL, ";") {
			if strings.TrimSpace(q) != "" {
				if _, err = db.ExecContext(ctx, q); err != nil {
					return nil, fmt.Errorf("migration: %w", err)
				}
			}
		}
		if _, err = db.ExecContext(ctx, "INSERT IGNORE INTO schema_migrations(version) VALUES(1)"); err != nil {
			return nil, err
		}
		s.db = db
	case "json":
		if env != "development" {
			return nil, errors.New("JSON storage is allowed only with APP_ENV=development")
		}
		s.file = path
		data, err := os.ReadFile(path)
		if err == nil {
			if err = json.Unmarshal(data, s.state); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("STORE_DRIVER must be mysql, or explicit development json")
	}
	return s, nil
}
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
func (s *Store) Run(ctx context.Context, write bool, fn func(*domain.State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		copyData, err := json.Marshal(s.state)
		if err != nil {
			return err
		}
		next := domain.NewState()
		if err = json.Unmarshal(copyData, next); err != nil {
			return err
		}
		if err = fn(next); err != nil {
			return err
		}
		if write {
			data, err := json.Marshal(next)
			if err != nil {
				return err
			}
			tmp := s.file + ".tmp"
			if err = os.WriteFile(tmp, data, 0600); err != nil {
				return err
			}
			f, err := os.OpenFile(tmp, os.O_RDWR, 0600)
			if err != nil {
				return err
			}
			err = f.Sync()
			_ = f.Close()
			if err != nil {
				return err
			}
			if err = os.Rename(tmp, s.file); err != nil {
				return err
			}
			s.state = next
		}
		return nil
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if write {
		var ok int
		if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK('docping_write_v1', 15)").Scan(&ok); err != nil || ok != 1 {
			return errors.New("database writer busy")
		}
		defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('docping_write_v1')")
	}
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: !write})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	next := domain.NewState()
	if err = load(ctx, tx, next); err != nil {
		return err
	}
	if err = fn(next); err != nil {
		return err
	}
	if write {
		if err = save(ctx, tx, next); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func readMap[T any](ctx context.Context, tx *sql.Tx, table string, dst map[string]T, key func(T) string) error {
	rows, err := tx.QueryContext(ctx, "SELECT payload FROM "+table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var data []byte
		var value T
		if err = rows.Scan(&data); err != nil {
			return err
		}
		if err = json.Unmarshal(data, &value); err != nil {
			return err
		}
		dst[key(value)] = value
	}
	return rows.Err()
}
func load(ctx context.Context, tx *sql.Tx, s *domain.State) error {
	steps := []func() error{
		func() error {
			return readMap(ctx, tx, "organizations", s.Organizations, func(x domain.Organization) string { return x.ID })
		}, func() error { return readMap(ctx, tx, "users", s.Users, func(x domain.User) string { return x.ID }) }, func() error {
			return readMap(ctx, tx, "sessions", s.Sessions, func(x domain.Session) string { return x.Hash })
		}, func() error {
			return readMap(ctx, tx, "clients", s.Clients, func(x domain.Client) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "document_templates", s.Templates, func(x domain.Template) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "document_requests", s.Requests, func(x domain.Request) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "public_access_tokens", s.Tokens, func(x domain.PublicToken) string { return x.Hash })
		}, func() error {
			return readMap(ctx, tx, "recurring_schedules", s.Schedules, func(x domain.Schedule) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "recurring_schedule_runs", s.Runs, func(x domain.ScheduleRun) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "notification_deliveries", s.Deliveries, func(x domain.Delivery) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "audit_events", s.Audits, func(x domain.Audit) string { return x.ID })
		}, func() error {
			return readMap(ctx, tx, "password_resets", s.Resets, func(x domain.PasswordReset) string { return x.Hash })
		}}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
func put(ctx context.Context, tx *sql.Tx, table string, cols []string, vals []any, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cols = append(cols, "payload")
	vals = append(vals, string(data))
	marks := make([]string, len(cols))
	updates := make([]string, 0, len(cols)-1)
	for i, c := range cols {
		marks[i] = "?"
		if i > 0 {
			updates = append(updates, c+"=VALUES("+c+")")
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO "+table+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+") ON DUPLICATE KEY UPDATE "+strings.Join(updates, ","), vals...)
	return err
}
func save(ctx context.Context, tx *sql.Tx, s *domain.State) error {
	for _, v := range s.Organizations {
		if err := put(ctx, tx, "organizations", []string{"id", "name", "demo"}, []any{v.ID, v.Name, v.Demo}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Users {
		if err := put(ctx, tx, "users", []string{"id", "organization_id", "email"}, []any{v.ID, v.OrgID, v.Email}, v); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO organization_members(organization_id,user_id,role) VALUES(?,?,'owner')", v.OrgID, v.ID); err != nil {
			return err
		}
	}
	for _, v := range s.Sessions {
		if err := put(ctx, tx, "sessions", []string{"token_hash", "user_id"}, []any{v.Hash, v.UserID}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Clients {
		if err := put(ctx, tx, "clients", []string{"id", "organization_id", "name", "active"}, []any{v.ID, v.OrgID, v.Name, v.Active}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Templates {
		if err := put(ctx, tx, "document_templates", []string{"id", "organization_id", "name", "active"}, []any{v.ID, v.OrgID, v.Name, v.Active}, v); err != nil {
			return err
		}
		for n, item := range v.Items {
			if err := put(ctx, tx, "document_template_items", []string{"id", "organization_id", "template_id", "position"}, []any{item.ID, v.OrgID, v.ID, n}, item); err != nil {
				return err
			}
		}
	}
	for _, v := range s.Schedules {
		if err := put(ctx, tx, "recurring_schedules", []string{"id", "organization_id", "template_id", "active"}, []any{v.ID, v.OrgID, v.TemplateID, v.Active}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Requests {
		var schedule any
		if v.ScheduleID != "" {
			schedule = v.ScheduleID
		}
		if err := put(ctx, tx, "document_requests", []string{"id", "organization_id", "client_id", "template_id", "competence", "due_date", "status", "schedule_id"}, []any{v.ID, v.OrgID, v.ClientID, v.TemplateID, v.Competence, v.DueDate, v.Status, schedule}, v); err != nil {
			return err
		}
		for n, item := range v.Items {
			if err := put(ctx, tx, "document_request_items", []string{"id", "organization_id", "request_id", "position", "status"}, []any{item.ID, v.OrgID, v.ID, n, item.Status}, item); err != nil {
				return err
			}
			for _, f := range item.Files {
				if err := put(ctx, tx, "document_files", []string{"id", "organization_id", "item_id", "object_key", "scan_status"}, []any{f.ID, v.OrgID, item.ID, f.ObjectKey, f.ScanStatus}, f); err != nil {
					return err
				}
			}
		}
	}
	for _, v := range s.Tokens {
		if err := put(ctx, tx, "public_access_tokens", []string{"token_hash", "organization_id", "request_id"}, []any{v.Hash, v.OrgID, v.RequestID}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Runs {
		if err := put(ctx, tx, "recurring_schedule_runs", []string{"id", "organization_id", "schedule_id", "client_id", "competence", "request_id"}, []any{v.ID, v.OrgID, v.ScheduleID, v.ClientID, v.Competence, v.RequestID}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Deliveries {
		if err := put(ctx, tx, "notification_deliveries", []string{"id", "organization_id", "request_id", "status"}, []any{v.ID, v.OrgID, v.RequestID, v.Status}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Audits {
		if err := put(ctx, tx, "audit_events", []string{"id", "organization_id", "action", "entity_id"}, []any{v.ID, v.OrgID, v.Action, v.EntityID}, v); err != nil {
			return err
		}
	}
	for _, v := range s.Resets {
		if err := put(ctx, tx, "password_resets", []string{"token_hash", "user_id"}, []any{v.Hash, v.UserID}, v); err != nil {
			return err
		}
	}
	return nil
}
