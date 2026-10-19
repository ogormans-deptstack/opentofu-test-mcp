package scaffold

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ogormans-deptstack/opentofu-test-mcp/internal/tofu"
)

// MockProvider renders a mock_provider block for providerName using its schema.
// For each resource type it emits a mock_resource with computed attributes
// pre-filled with type-appropriate placeholder values, which is the tedious
// part an agent otherwise gets wrong. If resourceFilter is non-empty only that
// resource type is emitted.
func MockProvider(providerName string, entry *tofu.ProviderEntry, resourceFilter string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "mock_provider %q {\n", shortName(providerName))

	types := make([]string, 0, len(entry.ResourceSchemas))
	for t := range entry.ResourceSchemas {
		if resourceFilter == "" || t == resourceFilter {
			types = append(types, t)
		}
	}
	sort.Strings(types)

	for _, t := range types {
		schema := entry.ResourceSchemas[t]
		if schema == nil || schema.Block == nil {
			continue
		}
		defaults := computedDefaults(schema.Block)
		if len(defaults) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n  mock_resource %q {\n", t)
		sb.WriteString("    defaults = {\n")
		for _, d := range defaults {
			fmt.Fprintf(&sb, "      %s = %s\n", d.name, d.value)
		}
		sb.WriteString("    }\n  }\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}

type kv struct {
	name  string
	value string
}

// computedDefaults returns placeholder HCL literals for the computed scalar
// attributes of a block, sorted by name. Complex types are skipped to keep the
// generated mock valid and readable.
func computedDefaults(block *tofu.SchemaBlock) []kv {
	var out []kv
	for name, attr := range block.Attributes {
		if attr == nil || !attr.Computed {
			continue
		}
		lit, ok := placeholder(name, attr.Type)
		if !ok {
			continue
		}
		out = append(out, kv{name: name, value: lit})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// placeholder maps a cty-encoded attribute type to an HCL literal. Only scalar
// primitives yield a value; everything else is skipped (ok=false).
func placeholder(name string, raw json.RawMessage) (string, bool) {
	var prim string
	if err := json.Unmarshal(raw, &prim); err != nil {
		return "", false // compound type (list/map/object/...) -> skip
	}
	switch prim {
	case "string":
		return fmt.Sprintf("%q", "mock-"+name), true
	case "number":
		return "0", true
	case "bool":
		return "false", true
	default:
		return "", false
	}
}

// shortName turns a provider source address (e.g. registry.opentofu.org/
// hashicorp/aws) into the local name used in a mock_provider label (aws).
func shortName(source string) string {
	parts := strings.Split(source, "/")
	return parts[len(parts)-1]
}
