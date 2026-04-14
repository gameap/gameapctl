package daemon

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/pkg/errors"
)

const configFileMode os.FileMode = 0o600

type ConfigFile struct {
	ast  *ast.File
	data []byte
	path string
}

func LoadConfig(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read daemon config %s", path)
	}

	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse daemon config %s", path)
	}

	return &ConfigFile{ast: file, data: data, path: path}, nil
}

func (c *ConfigFile) Path() string {
	return c.path
}

// ReadString returns the scalar string value at the given YAMLPath ("$.api_host").
// Returns ("", false, nil) if path is not found.
func (c *ConfigFile) ReadString(yamlPath string) (string, bool, error) {
	p, err := yaml.PathString(yamlPath)
	if err != nil {
		return "", false, errors.Wrapf(err, "invalid yaml path %q", yamlPath)
	}

	var v string
	if err := p.Read(bytes.NewReader(c.data), &v); err != nil {
		if errors.Is(err, yaml.ErrNotFoundNode) {
			return "", false, nil
		}

		return "", false, errors.Wrapf(err, "failed to read yaml path %q", yamlPath)
	}

	return v, true, nil
}

// ReadUint returns the scalar unsigned-int value at the given YAMLPath.
// Returns (0, false, nil) if path is not found.
func (c *ConfigFile) ReadUint(yamlPath string) (uint, bool, error) {
	p, err := yaml.PathString(yamlPath)
	if err != nil {
		return 0, false, errors.Wrapf(err, "invalid yaml path %q", yamlPath)
	}

	var v uint
	if err := p.Read(bytes.NewReader(c.data), &v); err != nil {
		if errors.Is(err, yaml.ErrNotFoundNode) {
			return 0, false, nil
		}

		return 0, false, errors.Wrapf(err, "failed to read yaml path %q", yamlPath)
	}

	return v, true, nil
}

// EnsureGRPCEnabled sets grpc.enabled=true and optionally grpc.address.
// If the grpc: block is missing, it is appended; existing keys under grpc:
// are replaced in place to preserve surrounding comments.
func (c *ConfigFile) EnsureGRPCEnabled(address string) error {
	grpcPath, err := yaml.PathString("$.grpc")
	if err != nil {
		return errors.Wrap(err, "failed to build yaml path $.grpc")
	}

	_, filterErr := grpcPath.FilterFile(c.ast)
	switch {
	case filterErr == nil:
		if err := c.setUnderGRPC("enabled", "true"); err != nil {
			return err
		}
		if address != "" {
			if err := c.setUnderGRPC("address", fmt.Sprintf("%q", address)); err != nil {
				return err
			}
		}
	case errors.Is(filterErr, yaml.ErrNotFoundNode):
		if err := c.appendGRPCBlock(address); err != nil {
			return err
		}
	default:
		return errors.Wrap(filterErr, "failed to probe $.grpc")
	}

	return nil
}

func (c *ConfigFile) setUnderGRPC(key, scalarSrc string) error {
	keyPath, err := yaml.PathString("$.grpc." + key)
	if err != nil {
		return errors.Wrapf(err, "failed to build yaml path $.grpc.%s", key)
	}

	if _, filterErr := keyPath.FilterFile(c.ast); filterErr == nil {
		if err := keyPath.ReplaceWithReader(c.ast, strings.NewReader(scalarSrc)); err != nil {
			return errors.Wrapf(err, "failed to replace $.grpc.%s", key)
		}

		return nil
	} else if !errors.Is(filterErr, yaml.ErrNotFoundNode) {
		return errors.Wrapf(filterErr, "failed to probe $.grpc.%s", key)
	}

	grpcPath, err := yaml.PathString("$.grpc")
	if err != nil {
		return errors.Wrap(err, "failed to build yaml path $.grpc")
	}
	snippet := fmt.Sprintf("%s: %s\n", key, scalarSrc)
	if err := grpcPath.MergeFromReader(c.ast, strings.NewReader(snippet)); err != nil {
		return errors.Wrapf(err, "failed to merge %s into $.grpc", key)
	}

	return nil
}

func (c *ConfigFile) appendGRPCBlock(address string) error {
	var b strings.Builder
	b.WriteString("grpc:\n  enabled: true\n")
	if address != "" {
		fmt.Fprintf(&b, "  address: %q\n", address)
	}

	rootPath, err := yaml.PathString("$")
	if err != nil {
		return errors.Wrap(err, "failed to build yaml root path")
	}
	if err := rootPath.MergeFromReader(c.ast, strings.NewReader(b.String())); err != nil {
		return errors.Wrap(err, "failed to merge grpc block")
	}

	return nil
}

// DeleteKey removes a top-level key from the document. Missing keys are a no-op.
// Only paths of the form "$.<key>" are supported; nested paths return an error.
func (c *ConfigFile) DeleteKey(yamlPath string) error {
	const topLevelPrefix = "$."
	if !strings.HasPrefix(yamlPath, topLevelPrefix) {
		return errors.Errorf("DeleteKey: path %q must start with %q", yamlPath, topLevelPrefix)
	}
	key := yamlPath[len(topLevelPrefix):]
	if key == "" || strings.ContainsAny(key, ".[") {
		return errors.Errorf("DeleteKey supports only top-level keys, got %q", yamlPath)
	}

	if c.ast == nil || len(c.ast.Docs) == 0 {
		return errors.New("DeleteKey: empty yaml document")
	}

	body := c.ast.Docs[0].Body
	mapping, ok := body.(*ast.MappingNode)
	if !ok {
		return errors.Errorf("DeleteKey: document root is not a mapping (got %T)", body)
	}

	for i, v := range mapping.Values {
		tok := v.Key.GetToken()
		if tok == nil {
			continue
		}
		if tok.Value == key {
			mapping.Values = append(mapping.Values[:i], mapping.Values[i+1:]...)

			return nil
		}
	}

	return nil
}

func (c *ConfigFile) Save() error {
	out := c.ast.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(c.path, []byte(out), configFileMode); err != nil {
		return errors.Wrapf(err, "failed to write daemon config %s", c.path)
	}
	c.data = []byte(out)

	return nil
}

func Backup(path string) (string, error) {
	backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())

	src, err := os.Open(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open %s", path)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, configFileMode)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create backup %s", backupPath)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(backupPath)

		return "", errors.Wrap(err, "failed to copy backup")
	}

	return backupPath, nil
}

func Restore(backupPath, targetPath string) error {
	src, err := os.Open(backupPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open backup %s", backupPath)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, configFileMode)
	if err != nil {
		return errors.Wrapf(err, "failed to open target %s", targetPath)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return errors.Wrap(err, "failed to restore")
	}

	return nil
}
