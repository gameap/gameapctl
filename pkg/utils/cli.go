package utils

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

func readStdin(ctx context.Context) string {
	input := make(chan string)

	go func(lines chan string) {
		s := bufio.NewScanner(os.Stdin)
		s.Scan()
		lines <- s.Text()
	}(input)

	defer close(input)

	for {
		select {
		case line := <-input:
			return line
		case <-ctx.Done():
			return ""
		}
	}
}

func Ask(ctx context.Context, question string, allowEmpty bool, validate func(string) (bool, string)) (string, error) {
	fmt.Println("")

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		fmt.Print(question)

		result := readStdin(ctx)

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
