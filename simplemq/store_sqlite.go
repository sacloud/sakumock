package simplemq

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    queue_name TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    acquired_at INTEGER NOT NULL DEFAULT 0,
    visibility_timeout_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_messages_queue_name ON messages(queue_name);
CREATE TABLE IF NOT EXISTS queues (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '[]',
    visibility_timeout_seconds INTEGER NOT NULL,
    expire_seconds INTEGER NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    modified_at INTEGER NOT NULL
);
`

// SQLiteStore is a Store backed by SQLite.
type SQLiteStore struct {
	db                *sql.DB
	visibilityTimeout time.Duration
	messageExpiration time.Duration
	nextID            int64
	idMu              sync.Mutex
	done              chan struct{}
	closeOnce         sync.Once
}

// NewSQLiteStore opens (or creates) the SQLite database at path and returns a Store.
func NewSQLiteStore(path string, visibilityTimeout, messageExpiration time.Duration) (*SQLiteStore, error) {
	if visibilityTimeout <= 0 {
		visibilityTimeout = defaultVisibilityTimeout
	}
	if messageExpiration <= 0 {
		messageExpiration = defaultMessageExpiration
	}

	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}
	// SQLite does not support concurrent writes well; serialize access.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	var maxID int64
	if err := db.QueryRow(`SELECT COALESCE(MAX(CAST(id AS INTEGER)), 0) FROM queues`).Scan(&maxID); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to get max queue id: %w", err)
	}
	// Resume from the highest persisted ID, but never below the base so a
	// fresh database starts at a realistic 12-digit value.
	nextID := queueIDBase
	if maxID >= nextID {
		nextID = maxID + 1
	}

	slog.Info("sqlite store opened", "path", path)
	s := &SQLiteStore{
		db:                db,
		visibilityTimeout: visibilityTimeout,
		messageExpiration: messageExpiration,
		nextID:            nextID,
		done:              make(chan struct{}),
	}
	go s.compactLoop()
	return s, nil
}

func (s *SQLiteStore) allocateID() string {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	id := s.nextID
	s.nextID++
	return strconv.FormatInt(id, 10)
}

func (s *SQLiteStore) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w; rollback failed: %v", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// queueSettingsFor returns the visibility timeout and message expiration for the named queue.
// Falls back to store defaults when the queue has no control plane resource.
func (s *SQLiteStore) queueSettingsFor(queueName string) (vt, me time.Duration) {
	var vtSecs, expSecs int64
	row := s.db.QueryRow(`SELECT visibility_timeout_seconds, expire_seconds FROM queues WHERE name = ?`, queueName)
	if err := row.Scan(&vtSecs, &expSecs); err == nil && vtSecs > 0 && expSecs > 0 {
		return time.Duration(vtSecs) * time.Second, time.Duration(expSecs) * time.Second
	}
	return s.visibilityTimeout, s.messageExpiration
}

func (s *SQLiteStore) Send(queueName, content string, now time.Time) (storedMessage, error) {
	_, msgExpiration := s.queueSettingsFor(queueName)
	msg := storedMessage{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(msgExpiration),
	}
	query := `INSERT INTO messages (id, queue_name, content, created_at, updated_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`
	args := []any{msg.ID, queueName, msg.Content, msg.CreatedAt.UnixMilli(), msg.UpdatedAt.UnixMilli(), msg.ExpiresAt.UnixMilli()}
	err := s.withTx(context.Background(), func(tx *sql.Tx) error {
		slog.Debug("sqlite exec", "sql", query, "args", args)
		_, err := tx.Exec(query, args...)
		return err
	})
	if err != nil {
		return storedMessage{}, fmt.Errorf("failed to insert message: %w", err)
	}
	return msg, nil
}

func (s *SQLiteStore) Receive(queueName string, now time.Time) (msg storedMessage, ok bool, retErr error) {
	visibilityTimeout, _ := s.queueSettingsFor(queueName)
	nowMilli := now.UnixMilli()
	visibilityTimeoutAtMilli := now.Add(visibilityTimeout).UnixMilli()

	err := s.withTx(context.Background(), func(tx *sql.Tx) error {
		// Find and atomically update the first available message
		query := `UPDATE messages SET acquired_at = ?, updated_at = ?, visibility_timeout_at = ?
		 WHERE rowid = (
		     SELECT rowid FROM messages
		     WHERE queue_name = ? AND expires_at > ? AND (visibility_timeout_at = 0 OR visibility_timeout_at <= ?)
		     ORDER BY rowid ASC LIMIT 1
		 )
		 RETURNING id, content, created_at, updated_at, expires_at, acquired_at, visibility_timeout_at`
		args := []any{nowMilli, nowMilli, visibilityTimeoutAtMilli, queueName, nowMilli, nowMilli}
		slog.Debug("sqlite query", "sql", query, "args", args)
		row := tx.QueryRow(query, args...)

		var createdAt, updatedAt, expiresAt, acquiredAt, visibilityTimeoutAt int64
		err := row.Scan(&msg.ID, &msg.Content, &createdAt, &updatedAt, &expiresAt, &acquiredAt, &visibilityTimeoutAt)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to receive message: %w", err)
		}
		msg.CreatedAt = time.UnixMilli(createdAt)
		msg.UpdatedAt = time.UnixMilli(updatedAt)
		msg.ExpiresAt = time.UnixMilli(expiresAt)
		msg.AcquiredAt = time.UnixMilli(acquiredAt)
		msg.VisibilityTimeoutAt = time.UnixMilli(visibilityTimeoutAt)
		ok = true
		return nil
	})
	if err != nil {
		return storedMessage{}, false, err
	}
	return msg, ok, nil
}

func (s *SQLiteStore) ExtendTimeout(queueName, id string, now time.Time) (msg storedMessage, retErr error) {
	visibilityTimeout, _ := s.queueSettingsFor(queueName)
	nowMilli := now.UnixMilli()
	visibilityTimeoutAtMilli := now.Add(visibilityTimeout).UnixMilli()

	query := `UPDATE messages SET visibility_timeout_at = ?, updated_at = ?
		 WHERE queue_name = ? AND id = ?
		 RETURNING id, content, created_at, updated_at, expires_at, acquired_at, visibility_timeout_at`
	args := []any{visibilityTimeoutAtMilli, nowMilli, queueName, id}

	err := s.withTx(context.Background(), func(tx *sql.Tx) error {
		slog.Debug("sqlite query", "sql", query, "args", args)
		row := tx.QueryRow(query, args...)

		var createdAt, updatedAt, expiresAt, acquiredAt, visibilityTimeoutAt int64
		err := row.Scan(&msg.ID, &msg.Content, &createdAt, &updatedAt, &expiresAt, &acquiredAt, &visibilityTimeoutAt)
		if err == sql.ErrNoRows {
			return fmt.Errorf("message not found: %s", id)
		}
		if err != nil {
			return fmt.Errorf("failed to extend timeout: %w", err)
		}
		msg.CreatedAt = time.UnixMilli(createdAt)
		msg.UpdatedAt = time.UnixMilli(updatedAt)
		msg.ExpiresAt = time.UnixMilli(expiresAt)
		msg.AcquiredAt = time.UnixMilli(acquiredAt)
		msg.VisibilityTimeoutAt = time.UnixMilli(visibilityTimeoutAt)
		return nil
	})
	if err != nil {
		return storedMessage{}, err
	}
	return msg, nil
}

func (s *SQLiteStore) Delete(queueName, id string) error {
	query := `DELETE FROM messages WHERE queue_name = ? AND id = ?`
	args := []any{queueName, id}
	return s.withTx(context.Background(), func(tx *sql.Tx) error {
		slog.Debug("sqlite exec", "sql", query, "args", args)
		result, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to check rows affected: %w", err)
		}
		if rows == 0 {
			return fmt.Errorf("message not found: %s", id)
		}
		return nil
	})
}

func (s *SQLiteStore) compactLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			query := `DELETE FROM messages WHERE expires_at <= ?`
			args := []any{now.UnixMilli()}
			slog.Debug("sqlite compact", "sql", query, "args", args)
			if _, err := s.db.Exec(query, args...); err != nil {
				slog.Error("sqlite compact failed", "err", err)
			}
		}
	}
}

func (s *SQLiteStore) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		err = s.db.Close()
	})
	return err
}

// Control plane operations

func scanQueueRow(rows *sql.Rows) (storedQueue, error) {
	var q storedQueue
	var tagsJSON string
	var createdAt, modifiedAt int64
	if err := rows.Scan(&q.ID, &q.Name, &q.Description, &tagsJSON, &q.VisibilityTimeoutSeconds, &q.ExpireSeconds, &q.APIKey, &createdAt, &modifiedAt); err != nil {
		return storedQueue{}, fmt.Errorf("failed to scan queue row: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &q.Tags); err != nil {
		q.Tags = []string{}
	}
	q.CreatedAt = time.UnixMilli(createdAt)
	q.ModifiedAt = time.UnixMilli(modifiedAt)
	return q, nil
}

func scanQueueSingleRow(row *sql.Row) (storedQueue, error) {
	var q storedQueue
	var tagsJSON string
	var createdAt, modifiedAt int64
	err := row.Scan(&q.ID, &q.Name, &q.Description, &tagsJSON, &q.VisibilityTimeoutSeconds, &q.ExpireSeconds, &q.APIKey, &createdAt, &modifiedAt)
	if err == sql.ErrNoRows {
		return storedQueue{}, ErrQueueNotFound
	}
	if err != nil {
		return storedQueue{}, fmt.Errorf("failed to scan queue: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &q.Tags); err != nil {
		q.Tags = []string{}
	}
	q.CreatedAt = time.UnixMilli(createdAt)
	q.ModifiedAt = time.UnixMilli(modifiedAt)
	return q, nil
}

const selectQueueColumns = `id, name, description, tags, visibility_timeout_seconds, expire_seconds, api_key, created_at, modified_at`

func (s *SQLiteStore) CreateQueue(name, description string, tags []string, vtSecs, expSecs int, now time.Time) (storedQueue, error) {
	if tags == nil {
		tags = []string{}
	}
	if vtSecs <= 0 {
		vtSecs = int(s.visibilityTimeout.Seconds())
	}
	if expSecs <= 0 {
		expSecs = int(s.messageExpiration.Seconds())
	}

	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return storedQueue{}, fmt.Errorf("failed to marshal tags: %w", err)
	}

	id := s.allocateID()
	nowMilli := now.UnixMilli()

	err = s.withTx(context.Background(), func(tx *sql.Tx) error {
		// ON CONFLICT(name) DO NOTHING leaves a duplicate-name insert as a
		// no-op (0 rows affected), so we detect conflicts via RowsAffected
		// instead of matching a driver-specific error string.
		res, err := tx.Exec(
			`INSERT INTO queues (id, name, description, tags, visibility_timeout_seconds, expire_seconds, api_key, created_at, modified_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(name) DO NOTHING`,
			id, name, description, string(tagsJSON), vtSecs, expSecs, "", nowMilli, nowMilli,
		)
		if err != nil {
			return fmt.Errorf("failed to insert queue: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrQueueConflict
		}
		return nil
	})
	if err != nil {
		return storedQueue{}, err
	}

	return storedQueue{
		ID:                       id,
		Name:                     name,
		Description:              description,
		Tags:                     tags,
		VisibilityTimeoutSeconds: vtSecs,
		ExpireSeconds:            expSecs,
		CreatedAt:                now,
		ModifiedAt:               now,
	}, nil
}

func (s *SQLiteStore) ListQueues() ([]storedQueue, error) {
	rows, err := s.db.Query(`SELECT ` + selectQueueColumns + ` FROM queues ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}
	defer rows.Close()

	var result []storedQueue
	for rows.Next() {
		q, err := scanQueueRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, q)
	}
	if result == nil {
		result = []storedQueue{}
	}
	return result, rows.Err()
}

func (s *SQLiteStore) GetQueueByID(id string) (storedQueue, error) {
	row := s.db.QueryRow(`SELECT `+selectQueueColumns+` FROM queues WHERE id = ?`, id)
	return scanQueueSingleRow(row)
}

func (s *SQLiteStore) GetQueueByName(name string) (storedQueue, error) {
	row := s.db.QueryRow(`SELECT `+selectQueueColumns+` FROM queues WHERE name = ?`, name)
	return scanQueueSingleRow(row)
}

func (s *SQLiteStore) UpdateQueue(id, description string, tags []string, vtSecs, expSecs int, now time.Time) (storedQueue, error) {
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return storedQueue{}, fmt.Errorf("failed to marshal tags: %w", err)
	}

	nowMilli := now.UnixMilli()
	err = s.withTx(context.Background(), func(tx *sql.Tx) error {
		result, err := tx.Exec(
			`UPDATE queues SET description = ?, tags = ?, visibility_timeout_seconds = ?, expire_seconds = ?, modified_at = ? WHERE id = ?`,
			description, string(tagsJSON), vtSecs, expSecs, nowMilli, id,
		)
		if err != nil {
			return fmt.Errorf("failed to update queue: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrQueueNotFound
		}
		return nil
	})
	if err != nil {
		return storedQueue{}, err
	}
	return s.GetQueueByID(id)
}

func (s *SQLiteStore) DeleteQueueByID(id string) error {
	return s.withTx(context.Background(), func(tx *sql.Tx) error {
		var name string
		if err := tx.QueryRow(`SELECT name FROM queues WHERE id = ?`, id).Scan(&name); err == sql.ErrNoRows {
			return ErrQueueNotFound
		} else if err != nil {
			return fmt.Errorf("failed to get queue: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM messages WHERE queue_name = ?`, name); err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM queues WHERE id = ?`, id); err != nil {
			return fmt.Errorf("failed to delete queue: %w", err)
		}
		return nil
	})
}

func (s *SQLiteStore) CountMessages(id string, now time.Time) (int, error) {
	var name string
	if err := s.db.QueryRow(`SELECT name FROM queues WHERE id = ?`, id).Scan(&name); err == sql.ErrNoRows {
		return 0, ErrQueueNotFound
	} else if err != nil {
		return 0, fmt.Errorf("failed to get queue: %w", err)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE queue_name = ? AND expires_at > ?`, name, now.UnixMilli()).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	return count, nil
}

func (s *SQLiteStore) RotateAPIKey(id, newKey string, now time.Time) (storedQueue, error) {
	nowMilli := now.UnixMilli()
	err := s.withTx(context.Background(), func(tx *sql.Tx) error {
		result, err := tx.Exec(`UPDATE queues SET api_key = ?, modified_at = ? WHERE id = ?`, newKey, nowMilli, id)
		if err != nil {
			return fmt.Errorf("failed to rotate api key: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrQueueNotFound
		}
		return nil
	})
	if err != nil {
		return storedQueue{}, err
	}
	return s.GetQueueByID(id)
}

func (s *SQLiteStore) ClearMessages(id string) error {
	return s.withTx(context.Background(), func(tx *sql.Tx) error {
		var name string
		if err := tx.QueryRow(`SELECT name FROM queues WHERE id = ?`, id).Scan(&name); err == sql.ErrNoRows {
			return ErrQueueNotFound
		} else if err != nil {
			return fmt.Errorf("failed to get queue: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM messages WHERE queue_name = ?`, name); err != nil {
			return fmt.Errorf("failed to clear messages: %w", err)
		}
		return nil
	})
}
