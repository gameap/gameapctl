package fixer

import (
	"context"
	"log"

	"github.com/pkg/errors"
)

type Item struct {
	Name      string
	Condition func(ctx context.Context) (bool, error)
	FixFunc   func(ctx context.Context) error
}

type CheckFunc func(ctx context.Context) error

func RunFixer(ctx context.Context, checkFunc CheckFunc, items []Item) error {
	for _, item := range items {
		condition, err := item.Condition(ctx)
		if err != nil {
			return errors.WithMessagef(err, "failed to check condition in '%s' fixer", item.Name)
		}

		if !condition {
			continue
		}

		log.Println("Trying to run fix", item.Name)
		err = item.FixFunc(ctx)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to fix '%s'", item.Name))
			return errors.WithMessagef(err, "failed to fix '%s'", item.Name)
		}

		err = checkFunc(ctx)
		if err != nil {
			log.Println(errors.WithMessagef(err, "failed to check after '%s' fix", item.Name))
		} else {
			log.Println("Fix applied successfully")

			return nil
		}
	}

	err := checkFunc(ctx)
	if err != nil {
		return errors.WithMessage(err, "no more options left to fix the problem")
	}

	return nil
}
