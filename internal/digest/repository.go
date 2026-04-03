package digest

import "context"

type Repository interface {
	Create(ctx context.Context, delivery *Delivery) error
	GetByRecipientAndDate(ctx context.Context, recipient string, digestDate string) (*Delivery, error)
}
