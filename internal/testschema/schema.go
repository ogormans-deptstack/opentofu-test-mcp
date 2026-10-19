// Package testschema holds a static reference to the OpenTofu/Terraform native
// test framework grammar (.tftest.hcl / .tfmock.hcl). It lets the server answer
// "how do I write this block?" without a cluster or network round-trip.
package testschema

import (
	"fmt"
	"sort"
	"strings"
)

// Block describes one construct in a test file.
type Block struct {
	Name      string
	Summary   string
	Reference string
	Example   string
}

// blocks is keyed by the canonical block name.
var blocks = map[string]Block{
	"run": {
		Name:    "run",
		Summary: "A single test step. Each run block plans or applies the module and then checks assertions. Runs execute in order and share state within a file unless a run targets a separate module.",
		Reference: `run "LABEL" {
  command = plan | apply   # default: apply
  plan_options {           # optional
    mode    = normal | refresh-only
    refresh = true | false
  }
  variables { ... }        # override input variables for this run
  module { source = "./..." }  # optional: run against a helper module
  assert { ... }           # zero or more assertions
  expect_failures = [ ... ]    # resources/vars expected to fail validation
}`,
		Example: `run "defaults_apply" {
  command = apply

  assert {
    condition     = output.bucket_name != ""
    error_message = "bucket_name output must not be empty"
  }
}`,
	},
	"assert": {
		Name:    "assert",
		Summary: "A boolean check evaluated after a run. condition must evaluate to true; otherwise error_message is reported and the run fails.",
		Reference: `assert {
  condition     = <bool expression, may reference output.*, var.*, resource attrs>
  error_message = "shown when condition is false"
}`,
		Example: `assert {
  condition     = aws_instance.web.instance_type == "t3.micro"
  error_message = "web instance must be t3.micro"
}`,
	},
	"variables": {
		Name:    "variables",
		Summary: "Supplies input variable values. At file scope it applies to every run; inside a run block it overrides values for that run only.",
		Reference: `variables {
  region = "eu-west-1"
  tags   = { env = "test" }
}`,
		Example: `variables {
  bucket_prefix = "unit-test"
}`,
	},
	"provider": {
		Name:    "provider",
		Summary: "Configures a real provider for the test, exactly as in a normal configuration. Use mock_provider instead when you want to avoid real API calls.",
		Reference: `provider "NAME" {
  # normal provider arguments
}`,
		Example: `provider "aws" {
  region = "eu-west-1"
}`,
	},
	"mock_provider": {
		Name:    "mock_provider",
		Summary: "Replaces a provider with a mock so plans/applies make no real API calls. Resources and data sources return generated values, overridable with mock_resource / mock_data and override_* blocks.",
		Reference: `mock_provider "NAME" {
  alias = "optional"

  mock_resource "TYPE" {
    defaults = { id = "mock-id" }
  }
  mock_data "TYPE" {
    defaults = { arn = "arn:aws:mock" }
  }
  override_resource {
    target = TYPE.NAME
    values = { ... }
  }
}`,
		Example: `mock_provider "aws" {
  mock_resource "aws_s3_bucket" {
    defaults = {
      arn = "arn:aws:s3:::mock-bucket"
    }
  }
}`,
	},
	"mock_resource": {
		Name:    "mock_resource",
		Summary: "Inside mock_provider (or a .tfmock.hcl file): default attribute values returned for every instance of a resource type that the provider would otherwise compute.",
		Reference: `mock_resource "TYPE" {
  defaults = { attr = value }
}`,
		Example: `mock_resource "aws_instance" {
  defaults = {
    id            = "i-mock123"
    instance_type = "t3.micro"
  }
}`,
	},
	"mock_data": {
		Name:    "mock_data",
		Summary: "Like mock_resource but for data sources: default values returned when the mocked data source is read.",
		Reference: `mock_data "TYPE" {
  defaults = { attr = value }
}`,
		Example: `mock_data "aws_ami" {
  defaults = {
    id = "ami-mock"
  }
}`,
	},
	"override_resource": {
		Name:    "override_resource",
		Summary: "Overrides the mocked/computed values of a specific resource instance (by address) rather than every instance of a type.",
		Reference: `override_resource {
  target = TYPE.NAME
  values = { attr = value }
}`,
		Example: `override_resource {
  target = aws_s3_bucket.this
  values = { id = "fixed-bucket-id" }
}`,
	},
	"expect_failures": {
		Name:    "expect_failures",
		Summary: "A list of checkable objects (input variables, resources, outputs, or check blocks) whose custom conditions are expected to fail. The run passes only if each listed object fails as expected.",
		Reference: `expect_failures = [
  var.must_be_positive,
  aws_instance.web,
]`,
		Example: `run "rejects_bad_input" {
  command         = plan
  variables { count = -1 }
  expect_failures = [var.count]
}`,
	},
}

// fileKinds documents the two file types the framework recognizes.
var fileKinds = map[string]Block{
	"tftest": {
		Name:      "tftest",
		Summary:   "A .tftest.hcl (or .tftest.json) file holds one or more run blocks, plus optional file-scope variables, provider, and mock_provider blocks. Test files live alongside the module or under a tests/ directory.",
		Reference: "Top-level blocks allowed: variables, provider, mock_provider, override_resource/override_data/override_module, run.",
		Example: `variables {
  region = "eu-west-1"
}

run "plan_is_clean" {
  command = plan
}`,
	},
	"tfmock": {
		Name:      "tfmock",
		Summary:   "A .tfmock.hcl (or .tfmock.json) file holds reusable mock_resource / mock_data / override_* definitions that a mock_provider block can load via its source argument, so mocks can be shared across test files.",
		Reference: "Top-level blocks allowed: mock_resource, mock_data, override_resource, override_data, override_module.",
		Example: `mock_resource "aws_s3_bucket" {
  defaults = {
    arn = "arn:aws:s3:::shared-mock"
  }
}`,
	},
}

// Explain returns the reference for a single block or file kind. An empty topic
// returns the full overview. Unknown topics return an error listing valid ones.
func Explain(topic string) (string, error) {
	topic = strings.TrimSpace(strings.ToLower(topic))
	if topic == "" {
		return overview(), nil
	}
	if b, ok := blocks[topic]; ok {
		return render(b), nil
	}
	if b, ok := fileKinds[topic]; ok {
		return render(b), nil
	}
	return "", fmt.Errorf("unknown topic %q; valid topics: %s", topic, strings.Join(Topics(), ", "))
}

// Topics lists every explainable topic, sorted.
func Topics() []string {
	var out []string
	for k := range blocks {
		out = append(out, k)
	}
	for k := range fileKinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func render(b Block) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n%s\n\n## Reference\n%s\n\n## Example\n%s\n", b.Name, b.Summary, b.Reference, b.Example)
	return sb.String()
}

func overview() string {
	var sb strings.Builder
	sb.WriteString("# OpenTofu / Terraform native test framework\n\n")
	sb.WriteString("Tests are written in .tftest.hcl files containing run blocks. ")
	sb.WriteString("Each run plans or applies the module and checks assert conditions. ")
	sb.WriteString("mock_provider blocks (and shared .tfmock.hcl files) avoid real API calls.\n\n")
	sb.WriteString("Call this tool with a specific topic for the full block reference and an example.\n\n")
	sb.WriteString("## Topics\n")
	for _, t := range Topics() {
		b, ok := blocks[t]
		if !ok {
			b = fileKinds[t]
		}
		fmt.Fprintf(&sb, "- %s: %s\n", t, firstSentence(b.Summary))
	}
	return sb.String()
}

func firstSentence(s string) string {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i+1]
	}
	return s
}
