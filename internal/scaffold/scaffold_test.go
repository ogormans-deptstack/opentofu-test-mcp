package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ogormans-deptstack/opentofu-test-mcp/internal/tofu"
)

func writeModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	main := `variable "region" {
  type = string
}

variable "bucket_prefix" {
  type    = string
  default = "demo"
}

output "bucket_name" {
  value = "x"
}

output "bucket_arn" {
  value = "y"
}

provider "aws" {
  region = var.region
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInspect(t *testing.T) {
	mod, err := Inspect(writeModule(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %+v", mod.Variables)
	}
	// region is required (no default), bucket_prefix is not.
	byName := map[string]bool{}
	for _, v := range mod.Variables {
		byName[v.Name] = v.Required
	}
	if !byName["region"] {
		t.Error("region should be required")
	}
	if byName["bucket_prefix"] {
		t.Error("bucket_prefix should not be required")
	}
	if len(mod.Outputs) != 2 {
		t.Errorf("expected 2 outputs, got %v", mod.Outputs)
	}
	if len(mod.Providers) != 1 || mod.Providers[0] != "aws" {
		t.Errorf("expected [aws] providers, got %v", mod.Providers)
	}
}

func TestTestFile(t *testing.T) {
	mod, _ := Inspect(writeModule(t))
	out := TestFile(mod, "apply", true)

	for _, want := range []string{
		`mock_provider "aws" {}`,
		"command = apply",
		"region = null # TODO",
		"output.bucket_name != null",
		"output.bucket_arn != null",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated test missing %q:\n%s", want, out)
		}
	}
	// A variable with a default must not appear as a required TODO.
	if strings.Contains(out, "bucket_prefix = null") {
		t.Error("bucket_prefix has a default and should not be a required TODO")
	}
}

func TestTestFileDefaultsCommand(t *testing.T) {
	out := TestFile(&Module{}, "nonsense", false)
	if !strings.Contains(out, "command = plan") {
		t.Errorf("invalid command should default to plan:\n%s", out)
	}
}

func TestMockProvider(t *testing.T) {
	entry := &tofu.ProviderEntry{
		ResourceSchemas: map[string]*tofu.ResourceSchema{
			"aws_instance": {
				Block: &tofu.SchemaBlock{
					Attributes: map[string]*tofu.SchemaAttribute{
						"id":            {Type: mustRaw(`"string"`), Computed: true},
						"instance_type": {Type: mustRaw(`"string"`), Required: true},
						"public":        {Type: mustRaw(`"bool"`), Computed: true},
						"tags":          {Type: mustRaw(`["map","string"]`), Computed: true},
					},
				},
			},
		},
	}
	out := MockProvider("registry.opentofu.org/hashicorp/aws", entry, "")

	if !strings.Contains(out, `mock_provider "aws"`) {
		t.Errorf("missing mock_provider label:\n%s", out)
	}
	if !strings.Contains(out, `mock_resource "aws_instance"`) {
		t.Errorf("missing mock_resource:\n%s", out)
	}
	if !strings.Contains(out, `id = "mock-id"`) {
		t.Errorf("computed string should be mocked:\n%s", out)
	}
	if !strings.Contains(out, "public = false") {
		t.Errorf("computed bool should be mocked:\n%s", out)
	}
	// Required (non-computed) and complex-typed attrs are skipped.
	if strings.Contains(out, "instance_type") {
		t.Errorf("non-computed attr should be skipped:\n%s", out)
	}
	if strings.Contains(out, "tags") {
		t.Errorf("complex-typed attr should be skipped:\n%s", out)
	}
}

func mustRaw(s string) []byte { return []byte(s) }
