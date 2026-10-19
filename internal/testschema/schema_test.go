package testschema

import (
	"strings"
	"testing"
)

func TestExplainOverview(t *testing.T) {
	out, err := Explain("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "native test framework") {
		t.Errorf("overview missing header: %q", out)
	}
	if !strings.Contains(out, "run:") {
		t.Errorf("overview should list the run topic")
	}
}

func TestExplainKnownTopics(t *testing.T) {
	for _, topic := range []string{"run", "assert", "mock_provider", "mock_resource", "expect_failures", "tftest", "tfmock"} {
		out, err := Explain(topic)
		if err != nil {
			t.Fatalf("Explain(%q): %v", topic, err)
		}
		if !strings.Contains(out, "## Example") {
			t.Errorf("Explain(%q) missing example section", topic)
		}
	}
}

func TestExplainCaseInsensitive(t *testing.T) {
	a, _ := Explain("RUN")
	b, _ := Explain("run")
	if a != b {
		t.Error("Explain should be case-insensitive")
	}
}

func TestExplainUnknown(t *testing.T) {
	_, err := Explain("not_a_block")
	if err == nil {
		t.Fatal("expected error for unknown topic")
	}
	if !strings.Contains(err.Error(), "valid topics") {
		t.Errorf("error should list valid topics: %v", err)
	}
}

func TestTopicsSorted(t *testing.T) {
	topics := Topics()
	if len(topics) == 0 {
		t.Fatal("expected topics")
	}
	for i := 1; i < len(topics); i++ {
		if topics[i-1] > topics[i] {
			t.Errorf("topics not sorted: %v", topics)
		}
	}
}
