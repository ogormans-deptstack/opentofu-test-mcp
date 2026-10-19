// Package tofu wraps the OpenTofu (or Terraform) CLI for the commands the MCP
// server needs: provider schema introspection and running the native test
// framework.
package tofu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Binary locates the CLI to drive. OpenTofu is preferred; Terraform is used as
// a fallback so the server is useful in either ecosystem. An explicit override
// (e.g. an absolute path) always wins.
func Binary(override string) (string, error) {
	candidates := []string{override, "tofu", "terraform"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no tofu or terraform binary found on PATH (set the binary argument to override)")
}

// Run executes the CLI in dir and returns stdout. Stderr is folded into the
// error so the agent sees the actual CLI diagnostic rather than a bare exit
// code.
func Run(ctx context.Context, bin, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %w\n%s", bin, strings.Join(args, " "), err, msg)
	}
	return stdout.Bytes(), nil
}

// ProvidersSchema returns the parsed output of `providers schema -json` for the
// working directory. The directory must already be initialized (the schema
// comes from the installed providers under .terraform).
func ProvidersSchema(ctx context.Context, bin, dir string) (*ProviderSchemas, error) {
	out, err := Run(ctx, bin, dir, "providers", "schema", "-json")
	if err != nil {
		return nil, err
	}
	var schemas ProviderSchemas
	if err := json.Unmarshal(out, &schemas); err != nil {
		return nil, fmt.Errorf("parsing providers schema json: %w", err)
	}
	return &schemas, nil
}

// ProviderSchemas is a minimal view over `providers schema -json`. Only the
// pieces needed to synthesize mock_provider / mock_resource blocks are modeled.
type ProviderSchemas struct {
	FormatVersion   string                    `json:"format_version"`
	ProviderSchemas map[string]*ProviderEntry `json:"provider_schemas"`
}

type ProviderEntry struct {
	ResourceSchemas   map[string]*ResourceSchema `json:"resource_schemas"`
	DataSourceSchemas map[string]*ResourceSchema `json:"data_source_schemas"`
}

type ResourceSchema struct {
	Block *SchemaBlock `json:"block"`
}

type SchemaBlock struct {
	Attributes map[string]*SchemaAttribute `json:"attributes"`
	BlockTypes map[string]*NestedBlock     `json:"block_types"`
}

type SchemaAttribute struct {
	Type      json.RawMessage `json:"type"`
	Required  bool            `json:"required"`
	Optional  bool            `json:"optional"`
	Computed  bool            `json:"computed"`
	Sensitive bool            `json:"sensitive"`
}

type NestedBlock struct {
	Block       *SchemaBlock `json:"block"`
	NestingMode string       `json:"nesting_mode"`
}
