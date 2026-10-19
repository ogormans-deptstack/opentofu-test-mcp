package validate

import "testing"

func TestValidGood(t *testing.T) {
	src := `run "ok" {
  command = plan
  assert {
    condition     = output.name != null
    error_message = "name must be set"
  }
}`
	rep, err := Source([]byte(src), "x.tftest.hcl")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Valid {
		t.Errorf("expected valid, got findings: %+v", rep.Findings)
	}
	if rep.Runs != 1 {
		t.Errorf("expected 1 run, got %d", rep.Runs)
	}
}

func TestInvalidCommand(t *testing.T) {
	src := `run "bad" {
  command = destroy
  assert {
    condition     = true
    error_message = "x"
  }
}`
	rep, _ := Source([]byte(src), "x.tftest.hcl")
	if rep.Valid {
		t.Error("expected invalid for command=destroy")
	}
	if !hasMessage(rep, "command must be") {
		t.Errorf("expected command error, got %+v", rep.Findings)
	}
}

func TestAssertMissingFields(t *testing.T) {
	src := `run "bad" {
  command = plan
  assert {
    condition = true
  }
}`
	rep, _ := Source([]byte(src), "x.tftest.hcl")
	if rep.Valid {
		t.Error("expected invalid for assert missing error_message")
	}
	if !hasMessage(rep, "missing condition or error_message") {
		t.Errorf("expected assert error, got %+v", rep.Findings)
	}
}

func TestDuplicateRunNames(t *testing.T) {
	src := `run "dup" {
  command = plan
  assert {
    condition     = true
    error_message = "x"
  }
}
run "dup" {
  command = plan
  assert {
    condition     = true
    error_message = "x"
  }
}`
	rep, _ := Source([]byte(src), "x.tftest.hcl")
	if !hasMessage(rep, "duplicate run name") {
		t.Errorf("expected duplicate run error, got %+v", rep.Findings)
	}
}

func TestNoRunsWarns(t *testing.T) {
	rep, _ := Source([]byte(`variables { x = 1 }`), "x.tftest.hcl")
	if !hasMessage(rep, "no run blocks") {
		t.Errorf("expected no-run warning, got %+v", rep.Findings)
	}
}

func TestRunWithoutAssertWarns(t *testing.T) {
	rep, _ := Source([]byte(`run "r" { command = plan }`), "x.tftest.hcl")
	if !rep.Valid {
		t.Error("a run without asserts is valid, just warned")
	}
	if !hasMessage(rep, "no assert blocks") {
		t.Errorf("expected no-assert warning, got %+v", rep.Findings)
	}
}

func TestSyntaxError(t *testing.T) {
	rep, _ := Source([]byte(`run "x" { command = `), "x.tftest.hcl")
	if rep.Valid {
		t.Error("expected invalid for syntax error")
	}
}

func hasMessage(rep *Report, substr string) bool {
	for _, f := range rep.Findings {
		if contains(f.Message, substr) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
