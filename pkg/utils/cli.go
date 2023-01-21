package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
)

func Ask(question string, allowEmpty bool, validate func(string) (bool, string)) (string, error) {
	fmt.Println("")
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(question)

		result, err := reader.ReadString('\n')
		if err != nil {
			return result, errors.WithMessage(err, "failed to read string")
		}
		result = strings.TrimSpace(result)

		if allowEmpty && result == "" {
			return result, nil
		}

		if validate != nil {
			ok, message := validate(result)
			if !ok {
				fmt.Println(message)
				continue
			}
		}

		if result != "" {
			return result, nil
		}
	}

}
