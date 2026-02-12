package repository

import (
	"context"
	"database/sql"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
)

type PlanTypeRepository struct {
	db DBTX
}

func NewPlanTypeRepository(db DBTX) *PlanTypeRepository {
	return &PlanTypeRepository{db: db}
}

func (r *PlanTypeRepository) FindBySubscriptionTypeID(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error) {
	query := `
		SELECT id, subscription_type_id, plan_code, display_name, description,
		       price_cents, currency, duration_days, features, created_at, updated_at
		FROM plan_types
		WHERE subscription_type_id = ?
	`

	item := &entity.PlanType{}
	var description sql.NullString
	var features sql.NullString
	err := r.db.QueryRowContext(ctx, query, subscriptionTypeID).Scan(
		&item.ID,
		&item.SubscriptionTypeID,
		&item.PlanCode,
		&item.DisplayName,
		&description,
		&item.PriceCents,
		&item.Currency,
		&item.DurationDays,
		&features,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if description.Valid {
		item.Description = description.String
	}
	if features.Valid {
		item.Features = features.String
	}

	return item, nil
}
