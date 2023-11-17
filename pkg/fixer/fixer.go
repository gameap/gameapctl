package fixer

import (
	"context"
	"log"

	"github.com/pkg/errors"
)

type Item struct {
	Condition func(ctx context.Context) (bool, error)
	FixFunc   func(ctx context.Context) error
}

type CheckFunc func(ctx context.Context) error

func RunFixer(ctx context.Context, checkFunc CheckFunc, items []Item) error {
	for _, item := range items {
		condition, err := item.Condition(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to check condition")
		}

		if !condition {
			continue
		}

		err = item.FixFunc(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to fix")
		}

		err = checkFunc(ctx)
		if err != nil {
			log.Println(err)
		} else {
			break
		}
	}

	return nil
}
