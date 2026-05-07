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

type setupParams struct {
	domains       []string
	email         string
	challengeType string
	dnsProvider   string
	envKVs        []string
	staging       bool
}

func Setup(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	params, err := collectSetupParams(cliCtx)
	if err != nil {
		return err
	}

	if err := validateSetupParams(params); err != nil {
		return err
	}

	configPath := ConfigPath()

	lines, _, err := readEnv(configPath)
	if err != nil {
		return err
	}

	updates, err := buildUpdates(params)
	if err != nil {
		return err
	}

	if err := writeEnv(configPath, lines, updates); err != nil {
		return errors.WithMessage(err, "failed to write config")
	}

	log.Println("config.env updated. Restarting gameap service ...")

	if err := service.Restart(ctx, "gameap"); err != nil {
		return errors.WithMessage(err, "failed to restart gameap service")
	}

	log.Println("gameap restarted. Check logs (journalctl -u gameap) and " +
		"GET /api/admin/letsencrypt/status to confirm certificate issuance.")

	if params.challengeType == ChallengeHTTP01 {
		log.Println("HTTP-01 reminder: ensure port 80 is reachable from the public internet " +
			"and HTTP_PORT in config.env is set to 80 (or a reverse proxy forwards " +
			"/.well-known/acme-challenge/* to gameap-api).")
	}

	return nil
}

func collectSetupParams(cliCtx *cli.Context) (setupParams, error) {
	p := setupParams{
		domains:       splitAndTrim(cliCtx.String("domains")),
		email:         strings.TrimSpace(cliCtx.String("email")),
		challengeType: strings.TrimSpace(cliCtx.String("challenge")),
		dnsProvider:   strings.TrimSpace(cliCtx.String("dns-provider")),
		staging:       cliCtx.Bool("staging"),
		envKVs:        cliCtx.StringSlice("env"),
	}

	if cliCtx.Bool("non-interactive") {
		return p, nil
	}

	configPath := ConfigPath()
	log.Printf("Reading config from: %s\n", configPath)

	_, values, err := readEnv(configPath)
	if err != nil {
		return p, err
	}

	domains, email, challengeType, dnsProvider, envKVs, staging, err := promptMissing(
		values, p.domains, p.email, p.challengeType, p.dnsProvider, p.envKVs, p.staging,
	)
	if err != nil {
		return p, err
	}

	p.domains = domains
	p.email = email
	p.challengeType = challengeType
	p.dnsProvider = dnsProvider
	p.envKVs = envKVs
	p.staging = staging

	return p, nil
}

func validateSetupParams(p setupParams) error {
	if p.challengeType == "" {
		return errors.New("challenge type is required")
	}

	if p.challengeType != ChallengeHTTP01 && p.challengeType != ChallengeDNS01 {
		return errors.Errorf("unsupported challenge type %q (expected %q or %q)",
			p.challengeType, ChallengeHTTP01, ChallengeDNS01)
	}

	if len(p.domains) == 0 {
		return errors.New("at least one domain is required (--domains)")
	}

	if p.email == "" {
		return errors.New("email is required (--email)")
	}

	if _, err := mail.ParseAddress(p.email); err != nil {
		return errors.WithMessage(err, "invalid email")
	}

	if p.challengeType == ChallengeDNS01 && p.dnsProvider == "" {
		return errors.New("DNS provider is required for dns-01 (--dns-provider)")
	}

	if p.challengeType == ChallengeHTTP01 {
		for _, d := range p.domains {
			if strings.HasPrefix(d, "*") {
				return errors.Errorf("wildcard domain %q requires dns-01 challenge", d)
			}
		}
	}

	return nil
}

func buildUpdates(p setupParams) (map[string]string, error) {
	directoryURL := prodDirectoryURL
	if p.staging {
		directoryURL = stagingDirectoryURL
	}

	updates := map[string]string{
		"ACME_ENABLED":        "true",
		"ACME_CHALLENGE_TYPE": p.challengeType,
		"ACME_EMAIL":          p.email,
		"ACME_DOMAINS":        strings.Join(p.domains, ","),
		"ACME_DIRECTORY_URL":  directoryURL,
	}

	if p.challengeType == ChallengeDNS01 {
		updates["ACME_DNS_PROVIDER"] = p.dnsProvider
	} else {
		updates["ACME_DNS_PROVIDER"] = removeMarker
	}

	for _, kv := range p.envKVs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, errors.Errorf("invalid --env entry %q (expected KEY=VALUE)", kv)
		}

		updates[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	return updates, nil
}

type promptInputs struct {
	domains       []string
	email         string
	challengeType string
	dnsProvider   string
	envKVs        []string
	staging       bool
}

func promptMissing(
	current map[string]string,
	domains []string,
	email, challengeType, dnsProvider string,
	envKVs []string,
	staging bool,
) ([]string, string, string, string, []string, bool, error) {
	reader := bufio.NewReader(os.Stdin)
	in := promptInputs{
		domains: domains, email: email, challengeType: challengeType,
		dnsProvider: dnsProvider, envKVs: envKVs, staging: staging,
	}

	if err := promptChallenge(reader, &in); err != nil {
		return nil, "", "", "", nil, false, err
	}

	if err := promptDomains(reader, current, &in); err != nil {
		return nil, "", "", "", nil, false, err
	}

	if err := promptEmail(reader, &in); err != nil {
		return nil, "", "", "", nil, false, err
	}

	if err := promptDNSProvider(reader, &in); err != nil {
		return nil, "", "", "", nil, false, err
	}

	if err := promptStaging(reader, &in); err != nil {
		return nil, "", "", "", nil, false, err
	}

	if err := promptDNSCredentials(reader, &in); err != nil {
		return nil, "", "", "", nil, false, err
	}

	return in.domains, in.email, in.challengeType, in.dnsProvider, in.envKVs, in.staging, nil
}

func promptChallenge(reader *bufio.Reader, in *promptInputs) error {
	if in.challengeType != "" {
		return nil
	}

	input, err := promptLine(reader, fmt.Sprintf("Challenge type (%s/%s) [%s]: ",
		ChallengeHTTP01, ChallengeDNS01, ChallengeHTTP01))
	if err != nil {
		return err
	}

	in.challengeType = strings.ToLower(input)
	if in.challengeType == "" {
		in.challengeType = ChallengeHTTP01
	}

	return nil
}

func promptDomains(reader *bufio.Reader, current map[string]string, in *promptInputs) error {
	if len(in.domains) > 0 {
		return nil
	}

	def := strings.TrimSpace(current["HTTP_HOST"])
	if def == "" || def == "0.0.0.0" {
		def = ""
	}

	input, err := promptLine(reader, fmt.Sprintf("Domains (comma-separated, e.g. panel.example.com) [%s]: ", def))
	if err != nil {
		return err
	}

	if input == "" {
		input = def
	}

	in.domains = splitAndTrim(input)

	return nil
}

func promptEmail(reader *bufio.Reader, in *promptInputs) error {
	if in.email != "" {
		return nil
	}

	input, err := promptLine(reader, "ACME account email: ")
	if err != nil {
		return err
	}

	in.email = input

	return nil
}

func promptDNSProvider(reader *bufio.Reader, in *promptInputs) error {
	if in.challengeType != ChallengeDNS01 || in.dnsProvider != "" {
		return nil
	}

	input, err := promptLine(reader, "DNS provider identifier (<plugin-id>:<provider-name>): ")
	if err != nil {
		return err
	}

	in.dnsProvider = input

	return nil
}

func promptStaging(reader *bufio.Reader, in *promptInputs) error {
	if in.staging {
		return nil
	}

	input, err := promptLine(reader, "Use Let's Encrypt staging endpoint? [y/N]: ")
	if err != nil {
		return err
	}

	if l := strings.ToLower(input); l == "y" || l == "yes" {
		in.staging = true
	}

	return nil
}

func promptDNSCredentials(reader *bufio.Reader, in *promptInputs) error {
	if in.challengeType != ChallengeDNS01 || len(in.envKVs) > 0 {
		return nil
	}

	log.Println("Enter DNS provider credentials as KEY=VALUE, one per line. Empty line to finish.")

	for {
		input, err := promptLine(reader, "  > ")
		if err != nil {
			return err
		}

		if input == "" {
			return nil
		}

		in.envKVs = append(in.envKVs, input)
	}
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
