package letsencrypt

import (
	"bufio"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/gameap/gameapctl/pkg/service"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	prodDirectoryURL    = "https://acme-v02.api.letsencrypt.org/directory"
	stagingDirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

func Setup(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	domains := splitAndTrim(cliCtx.String("domains"))
	email := strings.TrimSpace(cliCtx.String("email"))
	dnsProvider := strings.TrimSpace(cliCtx.String("dns-provider"))
	staging := cliCtx.Bool("staging")
	envKVs := cliCtx.StringSlice("env")
	nonInteractive := cliCtx.Bool("non-interactive")

	configPath := ConfigPath()
	log.Printf("Reading config from: %s\n", configPath)

	lines, values, err := readEnv(configPath)
	if err != nil {
		return err
	}

	if !nonInteractive {
		domains, email, dnsProvider, envKVs, staging, err = promptMissing(
			values, domains, email, dnsProvider, envKVs, staging,
		)
		if err != nil {
			return err
		}
	}

	if len(domains) == 0 {
		return errors.New("at least one domain is required (--domains)")
	}

	if email == "" {
		return errors.New("email is required (--email)")
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return errors.WithMessage(err, "invalid email")
	}

	if dnsProvider == "" {
		return errors.New("DNS provider is required (--dns-provider, format <plugin-id>:<provider-name>)")
	}

	directoryURL := prodDirectoryURL
	if staging {
		directoryURL = stagingDirectoryURL
	}

	updates := map[string]string{
		"ACME_ENABLED":       "true",
		"ACME_EMAIL":         email,
		"ACME_DOMAINS":       strings.Join(domains, ","),
		"ACME_DIRECTORY_URL": directoryURL,
		"ACME_DNS_PROVIDER":  dnsProvider,
	}

	for _, kv := range envKVs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return errors.Errorf("invalid --env entry %q (expected KEY=VALUE)", kv)
		}

		updates[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	if err := writeEnv(configPath, lines, updates); err != nil {
		return errors.WithMessage(err, "failed to write config")
	}

	log.Println("config.env updated.")
	log.Println("Restarting gameap service ...")

	if err := service.Restart(ctx, "gameap"); err != nil {
		return errors.WithMessage(err, "failed to restart gameap service")
	}

	log.Println("gameap restarted. Check logs (journalctl -u gameap) and " +
		"GET /api/admin/letsencrypt/status to confirm certificate issuance.")

	return nil
}

func promptMissing(
	current map[string]string,
	domains []string,
	email, dnsProvider string,
	envKVs []string,
	staging bool,
) ([]string, string, string, []string, bool, error) {
	reader := bufio.NewReader(os.Stdin)

	if len(domains) == 0 {
		def := strings.TrimSpace(current["HTTP_HOST"])
		if def == "" || def == "0.0.0.0" {
			def = ""
		}

		input, err := promptLine(reader, fmt.Sprintf("Domains (comma-separated, e.g. *.example.com,example.com) [%s]: ", def))
		if err != nil {
			return nil, "", "", nil, false, err
		}

		if input == "" {
			input = def
		}

		domains = splitAndTrim(input)
	}

	if email == "" {
		input, err := promptLine(reader, "ACME account email: ")
		if err != nil {
			return nil, "", "", nil, false, err
		}

		email = input
	}

	if dnsProvider == "" {
		input, err := promptLine(reader, "DNS provider identifier (<plugin-id>:<provider-name>): ")
		if err != nil {
			return nil, "", "", nil, false, err
		}

		dnsProvider = input
	}

	if !staging {
		input, err := promptLine(reader, "Use Let's Encrypt staging endpoint? [y/N]: ")
		if err != nil {
			return nil, "", "", nil, false, err
		}

		switch strings.ToLower(input) {
		case "y", "yes":
			staging = true
		}
	}

	if len(envKVs) == 0 {
		log.Println("Enter DNS provider credentials as KEY=VALUE, one per line. Empty line to finish.")

		for {
			input, err := promptLine(reader, "  > ")
			if err != nil {
				return nil, "", "", nil, false, err
			}

			if input == "" {
				break
			}

			envKVs = append(envKVs, input)
		}
	}

	return domains, email, dnsProvider, envKVs, staging, nil
}

func promptLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", errors.WithMessage(err, "failed to read input")
	}

	return strings.TrimSpace(line), nil
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}

var _ cli.ActionFunc = Setup
