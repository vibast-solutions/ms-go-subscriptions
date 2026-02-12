package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
)

var (
	ErrSubscriptionNotFound      = errors.New("subscription not found")
	ErrSubscriptionAlreadyExists = errors.New("subscription already exists")
)

type SubscriptionRepository struct {
	db DBTX
}

func NewSubscriptionRepository(db DBTX) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Create(ctx context.Context, subscription *entity.Subscription) error {
	query := `
		INSERT INTO subscriptions (
			subscription_type_id, user_id, email, status,
			start_at, end_at, renew_at, auto_renew,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		subscription.SubscriptionTypeID,
		nullableStringValue(subscription.UserID),
		nullableStringValue(subscription.Email),
		subscription.Status,
		nullableTimeValue(subscription.StartAt),
		nullableTimeValue(subscription.EndAt),
		nullableTimeValue(subscription.RenewAt),
		subscription.AutoRenew,
		subscription.CreatedAt,
		subscription.UpdatedAt,
	)
	if err != nil {
		if isDuplicateEntryError(err) {
			return ErrSubscriptionAlreadyExists
		}
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	subscription.ID = uint64(id)
	return nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, subscription *entity.Subscription) error {
	query := `
		UPDATE subscriptions
		SET status = ?, start_at = ?, end_at = ?, renew_at = ?, auto_renew = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		subscription.Status,
		nullableTimeValue(subscription.StartAt),
		nullableTimeValue(subscription.EndAt),
		nullableTimeValue(subscription.RenewAt),
		subscription.AutoRenew,
		subscription.UpdatedAt,
		subscription.ID,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrSubscriptionNotFound
	}

	return nil
}

func (r *SubscriptionRepository) FindByID(ctx context.Context, id uint64) (*entity.Subscription, error) {
	query := `
		SELECT id, subscription_type_id, user_id, email, status,
		       start_at, end_at, renew_at, auto_renew,
		       created_at, updated_at
		FROM subscriptions
		WHERE id = ?
	`

	item := &entity.Subscription{}
	if err := scanSubscription(
		r.db.QueryRowContext(ctx, query, id),
		item,
	); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return item, nil
}

func (r *SubscriptionRepository) FindByTypeAndIdentity(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error) {
	query := `
		SELECT id, subscription_type_id, user_id, email, status,
		       start_at, end_at, renew_at, auto_renew,
		       created_at, updated_at
		FROM subscriptions
		WHERE subscription_type_id = ?
		  AND user_id <=> ?
		  AND email <=> ?
		LIMIT 1
	`

	item := &entity.Subscription{}
	if err := scanSubscription(
		r.db.QueryRowContext(ctx, query, subscriptionTypeID, nullableStringValue(userID), nullableStringValue(email)),
		item,
	); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return item, nil
}

func (r *SubscriptionRepository) List(ctx context.Context, userID, email string) ([]*entity.Subscription, error) {
	query := `
		SELECT id, subscription_type_id, user_id, email, status,
		       start_at, end_at, renew_at, auto_renew,
		       created_at, updated_at
		FROM subscriptions
	`

	conditions := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)
	if strings.TrimSpace(userID) != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, userID)
	}
	if strings.TrimSpace(email) != "" {
		conditions = append(conditions, "email = ?")
		args = append(args, email)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*entity.Subscription, 0)
	for rows.Next() {
		item, err := scanSubscriptionFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *SubscriptionRepository) ListDueAutoRenew(ctx context.Context, nowSQLTime time.Time) ([]*entity.Subscription, error) {
	query := `
		SELECT id, subscription_type_id, user_id, email, status,
		       start_at, end_at, renew_at, auto_renew,
		       created_at, updated_at
		FROM subscriptions
		WHERE auto_renew = 1
		  AND renew_at <= ?
		  AND status = ?
		ORDER BY id ASC
	`

	return r.listByQuery(ctx, query, nowSQLTime, entity.SubscriptionStatusActive)
}

func (r *SubscriptionRepository) ListPendingPaymentStale(ctx context.Context, cutoffSQLTime time.Time) ([]*entity.Subscription, error) {
	query := `
		SELECT id, subscription_type_id, user_id, email, status,
		       start_at, end_at, renew_at, auto_renew,
		       created_at, updated_at
		FROM subscriptions
		WHERE status = ?
		  AND updated_at < ?
		ORDER BY id ASC
	`

	return r.listByQuery(ctx, query, entity.SubscriptionStatusPendingPayment, cutoffSQLTime)
}

func (r *SubscriptionRepository) ListExpiredActive(ctx context.Context, nowSQLTime time.Time) ([]*entity.Subscription, error) {
	query := `
		SELECT id, subscription_type_id, user_id, email, status,
		       start_at, end_at, renew_at, auto_renew,
		       created_at, updated_at
		FROM subscriptions
		WHERE status = ?
		  AND end_at IS NOT NULL
		  AND end_at < ?
		ORDER BY id ASC
	`

	return r.listByQuery(ctx, query, entity.SubscriptionStatusActive, nowSQLTime)
}

func (r *SubscriptionRepository) listByQuery(ctx context.Context, query string, args ...interface{}) ([]*entity.Subscription, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*entity.Subscription, 0)
	for rows.Next() {
		item, err := scanSubscriptionFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanSubscription(scanner rowScanner, item *entity.Subscription) error {
	var userID sql.NullString
	var email sql.NullString
	var startAt sql.NullTime
	var endAt sql.NullTime
	var renewAt sql.NullTime

	err := scanner.Scan(
		&item.ID,
		&item.SubscriptionTypeID,
		&userID,
		&email,
		&item.Status,
		&startAt,
		&endAt,
		&renewAt,
		&item.AutoRenew,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if userID.Valid {
		item.UserID = &userID.String
	} else {
		item.UserID = nil
	}
	if email.Valid {
		item.Email = &email.String
	} else {
		item.Email = nil
	}
	if startAt.Valid {
		item.StartAt = &startAt.Time
	} else {
		item.StartAt = nil
	}
	if endAt.Valid {
		item.EndAt = &endAt.Time
	} else {
		item.EndAt = nil
	}
	if renewAt.Valid {
		item.RenewAt = &renewAt.Time
	} else {
		item.RenewAt = nil
	}

	return nil
}

func scanSubscriptionFromRows(rows *sql.Rows) (*entity.Subscription, error) {
	item := &entity.Subscription{}
	if err := scanSubscription(rows, item); err != nil {
		return nil, err
	}
	return item, nil
}

func nullableStringValue(v *string) interface{} {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return strings.TrimSpace(*v)
}

func nullableTimeValue(v *time.Time) interface{} {
	if v == nil {
		return nil
	}
	return *v
}
