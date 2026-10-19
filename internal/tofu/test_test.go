package tofu

import "testing"

func TestParseTestStream(t *testing.T) {
	stream := `{"@level":"info","@message":"starting"}
{"type":"test_run","test_run":{"path":"tests/main.tftest.hcl","run":"defaults","status":"pass"}}
{"type":"test_run","test_run":{"path":"tests/main.tftest.hcl","run":"bad_input","status":"fail"}}
{"@level":"error","@message":"assertion failed: bucket_name must not be empty"}
{"type":"test_summary","test_summary":{"status":"fail","passed":1,"failed":1,"errored":0,"skipped":0}}
not-json-line
`
	res, parsed := parseTestStream([]byte(stream))
	if !parsed {
		t.Fatal("expected parsedAny=true")
	}
	if res.Passed != 1 || res.Failed != 1 || res.Errored != 0 || res.Skipped != 0 {
		t.Errorf("summary mismatch: %+v", res)
	}
	if len(res.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(res.Runs))
	}
	if res.Runs[0].Run != "defaults" || res.Runs[0].Status != "pass" {
		t.Errorf("run 0 mismatch: %+v", res.Runs[0])
	}
	if len(res.Problems) != 1 {
		t.Errorf("expected 1 problem, got %v", res.Problems)
	}
}

func TestParseTestStreamEmpty(t *testing.T) {
	_, parsed := parseTestStream([]byte("garbage\nmore garbage\n"))
	if parsed {
		t.Error("expected parsedAny=false for non-JSON input")
	}
}

func TestBinaryFallback(t *testing.T) {
	// tofu/terraform are expected on PATH in CI and dev; an unknown override
	// still falls back to them.
	bin, err := Binary("definitely-not-a-real-binary-xyz")
	if err != nil {
		t.Skipf("no tofu/terraform on PATH: %v", err)
	}
	if bin == "" {
		t.Error("expected a non-empty binary path")
	}
}
