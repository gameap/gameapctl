package daemon

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gameap/gameapctl/pkg/releasesource"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/pkg/errors"
)

const remoteRepositoryReplacementsKey = "remote_repository_replacements"

type replacementRule struct {
	origin string
	mirror string
}

// planCDNReplacements returns an origin -> mirror rule for every unreachable
// CDN whose mirror is reachable.
func planCDNReplacements(availability map[string]bool) []replacementRule {
	rules := []replacementRule{
		{origin: releasesource.CDNGameAPCom, mirror: releasesource.CDNGameAPRu},
		{origin: releasesource.CDNGameAPRu, mirror: releasesource.CDNGameAPCom},
	}

	planned := make([]replacementRule, 0, len(rules))
	for _, rule := range rules {
		if !availability[rule.origin] && availability[rule.mirror] {
			planned = append(planned, rule)
		}
	}

	return planned
}

// SetupCDNReplacements probes the GameAP CDNs and adds remote repository
// replacements for unreachable ones to the daemon config. It never fails:
// all errors are reported as warnings.
func SetupCDNReplacements(ctx context.Context, configPath string) {
	availability := releasesource.CDNAvailability(ctx)

	anyAvailable := false
	for _, available := range availability {
		if available {
			anyAvailable = true

			break
		}
	}
	if !anyAvailable {
		log.Printf(
			"Warning: both %s and %s are unreachable, leaving %s unchanged\n",
			releasesource.CDNGameAPCom, releasesource.CDNGameAPRu, remoteRepositoryReplacementsKey,
		)

		return
	}

	if err := EnsureRepositoryReplacements(configPath, availability); err != nil {
		log.Printf("Warning: failed to setup remote repository replacements in %s: %v\n", configPath, err)
	}
}

// EnsureRepositoryReplacements adds replacement entries for unreachable CDNs
// to the daemon config, preserving comments. Existing entries are never
// modified or removed; the file is written only when something was added.
func EnsureRepositoryReplacements(configPath string, availability map[string]bool) error {
	plan := planCDNReplacements(availability)
	if len(plan) == 0 {
		return nil
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		return errors.WithMessage(err, "failed to load daemon config")
	}

	changed := false
	for _, rule := range plan {
		exists, existsErr := cfg.hasRepositoryReplacement(rule.origin)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			log.Printf(
				"Notice: %s entry for %s already exists, leaving it untouched\n",
				remoteRepositoryReplacementsKey, rule.origin,
			)

			continue
		}

		if addErr := cfg.addRepositoryReplacement(rule.origin, rule.mirror); addErr != nil {
			return addErr
		}
		log.Printf("Adding remote repository replacement %s -> %s to daemon config\n", rule.origin, rule.mirror)
		changed = true
	}

	if !changed {
		return nil
	}

	return errors.WithMessage(cfg.Save(), "failed to save daemon config")
}

func (c *ConfigFile) hasRepositoryReplacement(origin string) (bool, error) {
	root, err := c.rootMapping()
	if err != nil {
		return false, err
	}

	entry, found := mappingValueByKey(root, remoteRepositoryReplacementsKey)
	if !found {
		return false, nil
	}

	switch value := entry.Value.(type) {
	case *ast.MappingNode:
		_, found = mappingValueByKey(value, origin)

		return found, nil
	case nil, *ast.NullNode:
		return false, nil
	default:
		return false, errors.Errorf(
			"%s has unexpected structure (%T)", remoteRepositoryReplacementsKey, value,
		)
	}
}

// addRepositoryReplacement inserts an origin -> mirror entry; the caller must
// have checked that the origin entry does not exist yet.
func (c *ConfigFile) addRepositoryReplacement(origin, mirror string) error {
	root, err := c.rootMapping()
	if err != nil {
		return err
	}

	entry, found := mappingValueByKey(root, remoteRepositoryReplacementsKey)
	if !found {
		return c.appendReplacementsBlock(origin, mirror)
	}

	switch value := entry.Value.(type) {
	case *ast.MappingNode:
		if len(value.Values) == 0 {
			return c.recreateReplacementsBlock(origin, mirror)
		}
		if value.IsFlowStyle {
			return errors.Errorf(
				"%s uses flow style, cannot add %s entry", remoteRepositoryReplacementsKey, origin,
			)
		}

		return c.mergeReplacementEntry(origin, mirror)
	case nil, *ast.NullNode:
		return c.recreateReplacementsBlock(origin, mirror)
	default:
		return errors.Errorf(
			"%s has unexpected structure (%T)", remoteRepositoryReplacementsKey, value,
		)
	}
}

// recreateReplacementsBlock replaces an empty remote_repository_replacements
// key with a populated block.
func (c *ConfigFile) recreateReplacementsBlock(origin, mirror string) error {
	if err := c.DeleteKey("$." + remoteRepositoryReplacementsKey); err != nil {
		return err
	}

	return c.appendReplacementsBlock(origin, mirror)
}

func (c *ConfigFile) appendReplacementsBlock(origin, mirror string) error {
	rootPath, err := yaml.PathString("$")
	if err != nil {
		return errors.Wrap(err, "failed to build yaml root path")
	}

	snippet := fmt.Sprintf("%s:\n  %s:\n    - %s\n", remoteRepositoryReplacementsKey, origin, mirror)
	if err := rootPath.MergeFromReader(c.ast, strings.NewReader(snippet)); err != nil {
		return errors.Wrapf(err, "failed to merge %s block", remoteRepositoryReplacementsKey)
	}

	return nil
}

func (c *ConfigFile) mergeReplacementEntry(origin, mirror string) error {
	keyPath, err := yaml.PathString("$." + remoteRepositoryReplacementsKey)
	if err != nil {
		return errors.Wrapf(err, "failed to build yaml path $.%s", remoteRepositoryReplacementsKey)
	}

	snippet := fmt.Sprintf("%s:\n  - %s\n", origin, mirror)
	if err := keyPath.MergeFromReader(c.ast, strings.NewReader(snippet)); err != nil {
		return errors.Wrapf(err, "failed to merge %s into $.%s", origin, remoteRepositoryReplacementsKey)
	}

	return nil
}

func (c *ConfigFile) rootMapping() (*ast.MappingNode, error) {
	if c.ast == nil || len(c.ast.Docs) == 0 {
		return nil, errors.New("empty yaml document")
	}

	body := c.ast.Docs[0].Body
	mapping, ok := body.(*ast.MappingNode)
	if !ok {
		return nil, errors.Errorf("document root is not a mapping (got %T)", body)
	}

	return mapping, nil
}

func mappingValueByKey(mapping *ast.MappingNode, key string) (*ast.MappingValueNode, bool) {
	for _, value := range mapping.Values {
		tok := value.Key.GetToken()
		if tok == nil {
			continue
		}
		if tok.Value == key {
			return value, true
		}
	}

	return nil, false
}
