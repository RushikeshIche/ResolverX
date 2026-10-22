package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rushi/ResolverX/internal/records"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrDuplicateRecord = errors.New("a record with this domain, type, and value already exists")
)

// Store provides thread-safe persistent SQLite storage for custom DNS records.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore initializes a SQLite database at dbPath and executes schema migrations.
// Use ":memory:" for an in-memory database during tests.
func NewStore(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory for database: %w", err)
			}
		}
	}

	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Single writer / connection pool settings optimal for SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute migrations: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS records (
		id TEXT PRIMARY KEY,
		domain TEXT NOT NULL,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		ttl INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_records_domain_type ON records(domain, type);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_records_unique_entry ON records(domain, type, value);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// CreateRecord persists a new custom DNS record.
func (s *Store) CreateRecord(ctx context.Context, r *records.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	// Normalize domain before storage
	r.Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(r.Domain), "."))

	query := `
	INSERT INTO records (id, domain, type, value, ttl, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, r.ID, r.Domain, string(r.Type), r.Value, r.TTL, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicateRecord
		}
		return fmt.Errorf("failed to insert record: %w", err)
	}
	return nil
}

// UpdateRecord updates an existing record's value and TTL.
func (s *Store) UpdateRecord(ctx context.Context, r *records.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.UpdatedAt = time.Now().UTC()
	r.Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(r.Domain), "."))

	query := `
	UPDATE records
	SET domain = ?, type = ?, value = ?, ttl = ?, updated_at = ?
	WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, r.Domain, string(r.Type), r.Value, r.TTL, r.UpdatedAt, r.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicateRecord
		}
		return fmt.Errorf("failed to update record: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// DeleteRecord removes a record by ID.
func (s *Store) DeleteRecord(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM records WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetRecord retrieves a single record by ID.
func (s *Store) GetRecord(ctx context.Context, id string) (*records.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, domain, type, value, ttl, created_at, updated_at FROM records WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var r records.Record
	var tStr string
	err := row.Scan(&r.ID, &r.Domain, &tStr, &r.Value, &r.TTL, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to query record: %w", err)
	}
	r.Type = records.RecordType(tStr)
	return &r, nil
}

// ListRecords returns all custom DNS records sorted by domain and type.
func (s *Store) ListRecords(ctx context.Context) ([]*records.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, domain, type, value, ttl, created_at, updated_at FROM records ORDER BY domain ASC, type ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	defer rows.Close()

	var result []*records.Record
	for rows.Next() {
		var r records.Record
		var tStr string
		if err := rows.Scan(&r.ID, &r.Domain, &tStr, &r.Value, &r.TTL, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan record row: %w", err)
		}
		r.Type = records.RecordType(tStr)
		result = append(result, &r)
	}

	return result, rows.Err()
}

// FindRecords looks up records matching domain and record type.
// The domain can have or omit trailing dots; it is normalized automatically.
func (s *Store) FindRecords(ctx context.Context, domain string, recType records.RecordType) ([]*records.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normDomain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))

	query := `SELECT id, domain, type, value, ttl, created_at, updated_at FROM records WHERE domain = ? AND type = ?`
	rows, err := s.db.QueryContext(ctx, query, normDomain, string(recType))
	if err != nil {
		return nil, fmt.Errorf("failed to find records: %w", err)
	}
	defer rows.Close()

	var result []*records.Record
	for rows.Next() {
		var r records.Record
		var tStr string
		if err := rows.Scan(&r.ID, &r.Domain, &tStr, &r.Value, &r.TTL, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan record row: %w", err)
		}
		r.Type = records.RecordType(tStr)
		result = append(result, &r)
	}

	return result, rows.Err()
}
