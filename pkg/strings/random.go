package strings

import (
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
)

func GeneratePassword(length int) (string, error) {
	passwordGenerator, err := password.NewGenerator(&password.GeneratorInput{
		Symbols: "",
	})

	if err != nil {
		return "", errors.Wrap(err, "failed to create password generator")
	}

	pass, err := passwordGenerator.Generate(length, 0, 0, false, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate password")
	}

	return pass, nil
}
