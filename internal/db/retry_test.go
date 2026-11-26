package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// lockedError produces the refusal SQLite returns when a write follows a read
// snapshot that another connection has since replaced.
func lockedError(t *testing.T) error {
	t.Helper()
	database, err := Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if _, err := database.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO t (id, v) VALUES (1, 'a')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	database.SetMaxOpenConns(2)

	ctx := context.Background()
	reader, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("reader connection: %v", err)
	}
	defer reader.Close()
	writer, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("writer connection: %v", err)
	}
	defer writer.Close()

	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	var value string
	if err := tx.QueryRowContext(ctx, `SELECT v FROM t WHERE id = 1`).Scan(&value); err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := writer.ExecContext(ctx, `UPDATE t SET v = 'b' WHERE id = 1`); err != nil {
		t.Fatalf("other writer: %v", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE t SET v = 'c' WHERE id = 1`)
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		t.Fatalf("rollback: %v", rollbackErr)
	}
	if err == nil {
		t.Fatal("the stale write succeeded, want SQLite to refuse it")
	}
	return err
}

type fakeExec struct {
	failures []error
	calls    int
}

func (f *fakeExec) Exec(string, ...any) (sql.Result, error) {
	f.calls++
	if len(f.failures) == 0 {
		return nil, nil
	}
	err := f.failures[0]
	f.failures = f.failures[1:]
	return nil, err
}

func TestExecRetryRetriesALockedWrite(t *testing.T) {
	locked := lockedError(t)
	executor := &fakeExec{failures: []error{locked, locked}}

	if _, err := ExecRetry(executor, `UPDATE t SET v = 'c' WHERE id = 1`); err != nil {
		t.Fatalf("ExecRetry: %v", err)
	}
	if executor.calls != 3 {
		t.Fatalf("calls = %d, want 3", executor.calls)
	}
}

func TestExecRetryReturnsOtherErrorsAtOnce(t *testing.T) {
	boom := errors.New("boom")
	executor := &fakeExec{failures: []error{boom}}

	if _, err := ExecRetry(executor, `UPDATE t SET v = 'c' WHERE id = 1`); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the original", err)
	}
	if executor.calls != 1 {
		t.Fatalf("calls = %d, want 1", executor.calls)
	}
}

func TestExecRetryGivesUpAfterRepeatedLocking(t *testing.T) {
	locked := lockedError(t)
	executor := &fakeExec{failures: []error{locked, locked, locked, locked, locked, locked, locked, locked, locked, locked}}

	if _, err := ExecRetry(executor, `UPDATE t SET v = 'c' WHERE id = 1`); err == nil {
		t.Fatal("ExecRetry succeeded, want the last refusal")
	}
	if executor.calls != 10 {
		t.Fatalf("calls = %d, want 10", executor.calls)
	}
}
