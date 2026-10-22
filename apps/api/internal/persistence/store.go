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
		for _, file := range migrations.Files() {
			for _, q := range strings.Split(file, ";") {
				if strings.TrimSpace(q) != "" {
					if _, err = db.ExecContext(ctx, q); err != nil {
						return nil, fmt.Errorf("migration: %w", err)
					}
				}
			}
		}
		for v := 1; v <= migrations.Version; v++ {
			if _, err = db.ExecContext(ctx, "INSERT IGNORE INTO schema_migrations(version) VALUES(?)", v); err != nil {
				return nil, err
			}
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

// clone returns an isolated copy of the cached state, so a handler that
// fails mutates nothing. The caller must hold s.mu.
func (s *Store) clone() (*domain.State, error) {
	data, err := json.Marshal(s.state)
	if err != nil {
		return nil, err
	}
	next := domain.NewState()
	if err = json.Unmarshal(data, next); err != nil {
		return nil, err
	}
	return next, nil
}

// persist writes the state with temp-file+rename and only then publishes it as
// the cached one, so an interrupted write leaves the previous file intact. The
// caller must hold s.mu.
func (s *Store) persist(next *domain.State) error {
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
	return nil
}
func (s *Store) Run(ctx context.Context, write bool, fn func(*domain.State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		next, err := s.clone()
		if err != nil {
			return err
		}
		if err = fn(next); err != nil {
			return err
		}
		if write {
			return s.persist(next)
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
		}, func() error {
			return readMap(ctx, tx, "email_verifications", s.Verifications, func(x domain.EmailVerification) string { return x.Hash })
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
	for _, v := range s.Verifications {
		if err := put(ctx, tx, "email_verifications", []string{"token_hash", "user_id"}, []any{v.Hash, v.UserID}, v); err != nil {
			return err
		}
	}
	return nil
}

// purgeOrder deletes an organization's rows child-first. The schema declares
// no ON DELETE CASCADE, so the order is load-bearing: a parent deleted first
// is refused by the foreign key. Tables owned through the user (sessions,
// resets, verifications) are reached by subquery because they carry no
// organization column.
var purgeOrder = []string{
	"DELETE FROM document_files WHERE organization_id=?",
	"DELETE FROM document_request_items WHERE organization_id=?",
	"DELETE FROM public_access_tokens WHERE organization_id=?",
	"DELETE FROM recurring_schedule_runs WHERE schedule_id IN (SELECT id FROM recurring_schedules WHERE organization_id=?)",
	// The runs table carries its own organization column as well, so a row is
	// removed whether it is reached through the schedule or through the owner.
	"DELETE FROM recurring_schedule_runs WHERE organization_id=?",
	"DELETE FROM document_requests WHERE organization_id=?",
	"DELETE FROM recurring_schedules WHERE organization_id=?",
	"DELETE FROM document_template_items WHERE organization_id=?",
	"DELETE FROM document_templates WHERE organization_id=?",
	"DELETE FROM clients WHERE organization_id=?",
	"DELETE FROM notification_deliveries WHERE organization_id=?",
	"DELETE FROM audit_events WHERE organization_id=?",
	"DELETE FROM email_verifications WHERE user_id IN (SELECT id FROM users WHERE organization_id=?)",
	"DELETE FROM password_resets WHERE user_id IN (SELECT id FROM users WHERE organization_id=?)",
	"DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE organization_id=?)",
	"DELETE FROM organization_members WHERE organization_id=?",
	"DELETE FROM users WHERE organization_id=?",
	"DELETE FROM organizations WHERE id=?",
}

// PurgeOrganizations removes organizations and everything they own.
//
// It exists as its own method because save() only ever upserts: dropping an
// entity from the in-memory State is invisible to MySQL, and the next load
// would bring it back. Deletion therefore cannot go through Run — it needs
// real DELETE statements. Objects in the bucket are the caller's
// responsibility and must be removed before this returns, since the keys are
// only reachable through the state this deletes.
func (s *Store) PurgeOrganizations(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		next, err := s.clone()
		if err != nil {
			return err
		}
		for _, id := range ids {
			purgeState(next, id)
		}
		return s.persist(next)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var ok int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK('docping_write_v1', 15)").Scan(&ok); err != nil || ok != 1 {
		return errors.New("database writer busy")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('docping_write_v1')")
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		for _, q := range purgeOrder {
			if _, err = tx.ExecContext(ctx, q, id); err != nil {
				return fmt.Errorf("purge %s: %w", id, err)
			}
		}
	}
	return tx.Commit()
}

// purgeState is the JSON driver's equivalent of purgeOrder.
func purgeState(s *domain.State, org string) {
	users := map[string]bool{}
	for id, u := range s.Users {
		if u.OrgID == org {
			users[id] = true
			delete(s.Users, id)
		}
	}
	for h, v := range s.Sessions {
		if users[v.UserID] {
			delete(s.Sessions, h)
		}
	}
	for h, v := range s.Resets {
		if users[v.UserID] {
			delete(s.Resets, h)
		}
	}
	for h, v := range s.Verifications {
		if users[v.UserID] {
			delete(s.Verifications, h)
		}
	}
	for id, v := range s.Clients {
		if v.OrgID == org {
			delete(s.Clients, id)
		}
	}
	for id, v := range s.Templates {
		if v.OrgID == org {
			delete(s.Templates, id)
		}
	}
	schedules := map[string]bool{}
	for id, v := range s.Schedules {
		if v.OrgID == org {
			schedules[id] = true
			delete(s.Schedules, id)
		}
	}
	for id, v := range s.Runs {
		if v.OrgID == org || schedules[v.ScheduleID] {
			delete(s.Runs, id)
		}
	}
	for id, v := range s.Requests {
		if v.OrgID == org {
			delete(s.Requests, id)
		}
	}
	for h, v := range s.Tokens {
		if v.OrgID == org {
			delete(s.Tokens, h)
		}
	}
	for id, v := range s.Deliveries {
		if v.OrgID == org {
			delete(s.Deliveries, id)
		}
	}
	for id, v := range s.Audits {
		if v.OrgID == org {
			delete(s.Audits, id)
		}
	}
	delete(s.Organizations, org)
}
