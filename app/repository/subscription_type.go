package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
)

var ErrSubscriptionTypeNotFound = errors.New("subscription type not found")

type SubscriptionTypeRepository struct {
	db DBTX
}

func NewSubscriptionTypeRepository(db DBTX) *SubscriptionTypeRepository {
	return &SubscriptionTypeRepository{db: db}
}

func (r *SubscriptionTypeRepository) List(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error) {
	query := `
		SELECT id, type, display_name, status, created_at, updated_at
		FROM subscription_types
	`

	conditions := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)
	if strings.TrimSpace(typeFilter) != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, typeFilter)
	}
	if hasStatus {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*entity.SubscriptionType, 0)
	for rows.Next() {
		item := &entity.SubscriptionType{}
		if err := rows.Scan(&item.ID, &item.Type, &item.DisplayName, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *SubscriptionTypeRepository) FindByID(ctx context.Context, id uint64) (*entity.SubscriptionType, error) {
	query := `
		SELECT id, type, display_name, status, created_at, updated_at
		FROM subscription_types
		WHERE id = ?
	`

	item := &entity.SubscriptionType{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.Type,
		&item.DisplayName,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return item, nil
}
