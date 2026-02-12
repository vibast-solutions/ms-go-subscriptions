package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
)

type fakeDB struct {
	execFn func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

func (f *fakeDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if f.execFn != nil {
		return f.execFn(ctx, query, args...)
	}
	return fakeResult{lastInsertID: 1, rowsAffected: 1}, nil
}

func (f *fakeDB) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeDB) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

type fakeResult struct {
	lastInsertID int64
	rowsAffected int64
	lastErr      error
	rowsErr      error
}

func (r fakeResult) LastInsertId() (int64, error) {
	return r.lastInsertID, r.lastErr
}

func (r fakeResult) RowsAffected() (int64, error) {
	return r.rowsAffected, r.rowsErr
}

func TestCreateSuccess(t *testing.T) {
	repo := NewSubscriptionRepository(&fakeDB{execFn: func(_ context.Context, _ string, _ ...interface{}) (sql.Result, error) {
		return fakeResult{lastInsertID: 22}, nil
	}})

	now := time.Now().UTC()
	s := &entity.Subscription{
		SubscriptionTypeID: 1,
		Status:             entity.SubscriptionStatusActive,
		AutoRenew:          false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.ID != 22 {
		t.Fatalf("expected id=22, got %d", s.ID)
	}
}

func TestCreateMapsDuplicate(t *testing.T) {
	repo := NewSubscriptionRepository(&fakeDB{execFn: func(_ context.Context, _ string, _ ...interface{}) (sql.Result, error) {
		return nil, &mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"}
	}})

	err := repo.Create(context.Background(), &entity.Subscription{})
	if !errors.Is(err, ErrSubscriptionAlreadyExists) {
		t.Fatalf("expected ErrSubscriptionAlreadyExists, got %v", err)
	}
}

func TestUpdateNoRowsAffected(t *testing.T) {
	repo := NewSubscriptionRepository(&fakeDB{execFn: func(_ context.Context, _ string, _ ...interface{}) (sql.Result, error) {
		return fakeResult{rowsAffected: 0}, nil
	}})

	err := repo.Update(context.Background(), &entity.Subscription{ID: 1})
	if !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestIsDuplicateEntryError(t *testing.T) {
	if !isDuplicateEntryError(&mysqlDriver.MySQLError{Number: 1062}) {
		t.Fatal("expected true for mysql duplicate error")
	}
	if isDuplicateEntryError(errors.New("boom")) {
		t.Fatal("expected false for generic error")
	}
}

func TestNullableHelpers(t *testing.T) {
	if nullableStringValue(nil) != nil {
		t.Fatal("expected nil for nil string")
	}
	s := "  a@example.com  "
	if got := nullableStringValue(&s); got != "a@example.com" {
		t.Fatalf("expected trimmed value, got %#v", got)
	}
	tm := time.Now().UTC()
	if nullableTimeValue(nil) != nil {
		t.Fatal("expected nil for nil time")
	}
	if got := nullableTimeValue(&tm); got == nil {
		t.Fatal("expected non-nil for time value")
	}
}

type fakeRowScanner struct {
	id                 uint64
	subscriptionTypeID uint64
	userID             sql.NullString
	email              sql.NullString
	status             int32
	startAt            sql.NullTime
	endAt              sql.NullTime
	renewAt            sql.NullTime
	autoRenew          bool
	createdAt          time.Time
	updatedAt          time.Time
	err                error
}

func (f fakeRowScanner) Scan(dest ...interface{}) error {
	if f.err != nil {
		return f.err
	}
	*(dest[0].(*uint64)) = f.id
	*(dest[1].(*uint64)) = f.subscriptionTypeID
	*(dest[2].(*sql.NullString)) = f.userID
	*(dest[3].(*sql.NullString)) = f.email
	*(dest[4].(*int32)) = f.status
	*(dest[5].(*sql.NullTime)) = f.startAt
	*(dest[6].(*sql.NullTime)) = f.endAt
	*(dest[7].(*sql.NullTime)) = f.renewAt
	*(dest[8].(*bool)) = f.autoRenew
	*(dest[9].(*time.Time)) = f.createdAt
	*(dest[10].(*time.Time)) = f.updatedAt
	return nil
}

func TestScanSubscription(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	renew := now.Add(30 * time.Minute)

	item := &entity.Subscription{}
	err := scanSubscription(fakeRowScanner{
		id:                 9,
		subscriptionTypeID: 2,
		userID:             sql.NullString{String: "u-1", Valid: true},
		email:              sql.NullString{String: "u-1@example.com", Valid: true},
		status:             entity.SubscriptionStatusActive,
		startAt:            sql.NullTime{Time: start, Valid: true},
		endAt:              sql.NullTime{Time: end, Valid: true},
		renewAt:            sql.NullTime{Time: renew, Valid: true},
		autoRenew:          true,
		createdAt:          now,
		updatedAt:          now,
	}, item)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ID != 9 || item.SubscriptionTypeID != 2 || item.UserID == nil || item.Email == nil {
		t.Fatalf("unexpected scan result: %+v", item)
	}
	if item.StartAt == nil || item.EndAt == nil || item.RenewAt == nil {
		t.Fatalf("expected all time pointers to be populated: %+v", item)
	}
}
