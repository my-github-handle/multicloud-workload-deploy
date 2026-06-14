package preflight

import (
	"context"
	"testing"
)

// stubCheck is a Check that returns canned results for one stage.
type stubCheck struct {
	stage   StageID
	name    string
	results []CheckResult
	ran     *bool
}

func (s stubCheck) Stage() StageID { return s.stage }
func (s stubCheck) Name() string   { return s.name }
func (s stubCheck) Run(ctx context.Context) []CheckResult {
	if s.ran != nil {
		*s.ran = true
	}
	return s.results
}

func TestRunnerOrdersStages(t *testing.T) {
	checks := []Check{
		stubCheck{stage: StageWorkload, name: "workload", results: []CheckResult{{ID: "w", Status: StatusGreen}}},
		stubCheck{stage: StageIdentity, name: "identity", results: []CheckResult{{ID: "i", Status: StatusGreen}}},
		stubCheck{stage: StageNetwork, name: "network", results: []CheckResult{{ID: "n", Status: StatusGreen}}},
	}
	r := NewRunner(checks)
	report := r.Run(context.Background())

	if len(report.Stages) != 6 {
		t.Fatalf("expected 6 stages, got %d", len(report.Stages))
	}
	for i, s := range report.Stages {
		if int(s.ID) != i {
			t.Errorf("stage %d has ID %d, want %d", i, s.ID, i)
		}
	}
	if report.Verdict != StatusGreen {
		t.Errorf("verdict = %q, want green", report.Verdict)
	}
}

func TestRunnerShortCircuitsOnRed(t *testing.T) {
	netRan := false
	k8sRan := false
	checks := []Check{
		stubCheck{stage: StageIdentity, name: "identity", results: []CheckResult{{ID: "i", Status: StatusGreen}}},
		stubCheck{stage: StageKMS, name: "kms", results: []CheckResult{{ID: "k", Status: StatusRed, Message: "key disabled"}}},
		stubCheck{stage: StageNetwork, name: "network", results: []CheckResult{{ID: "n", Status: StatusGreen}}, ran: &netRan},
		stubCheck{stage: StageKubernetes, name: "kubernetes", results: []CheckResult{{ID: "k8s", Status: StatusGreen}}, ran: &k8sRan},
	}
	r := NewRunner(checks)
	report := r.Run(context.Background())

	if report.Verdict != StatusRed {
		t.Errorf("verdict = %q, want red", report.Verdict)
	}
	if report.Stages[StageKMS].Status != StatusRed {
		t.Errorf("KMS stage = %q, want red", report.Stages[StageKMS].Status)
	}
	// Stages after the red boundary must be marked skipped and must not run.
	for _, id := range []StageID{StageSecrets, StageNetwork, StageKubernetes, StageWorkload} {
		if report.Stages[id].Status != StatusSkipped {
			t.Errorf("stage %d = %q, want skipped", id, report.Stages[id].Status)
		}
	}
	if netRan {
		t.Error("network check ran despite earlier red")
	}
	if k8sRan {
		t.Error("kubernetes check ran despite earlier red")
	}
}

func TestRunnerAmberDoesNotShortCircuit(t *testing.T) {
	k8sRan := false
	checks := []Check{
		stubCheck{stage: StageIdentity, name: "identity", results: []CheckResult{{ID: "i", Status: StatusAmber, Message: "excess perms"}}},
		stubCheck{stage: StageKubernetes, name: "kubernetes", results: []CheckResult{{ID: "k8s", Status: StatusGreen}}, ran: &k8sRan},
	}
	r := NewRunner(checks)
	report := r.Run(context.Background())

	if report.Verdict != StatusAmber {
		t.Errorf("verdict = %q, want amber", report.Verdict)
	}
	if !k8sRan {
		t.Error("kubernetes check did not run after an amber stage")
	}
}

func TestRunnerEmptyStageIsGreen(t *testing.T) {
	r := NewRunner(nil)
	report := r.Run(context.Background())
	if len(report.Stages) != 6 {
		t.Fatalf("expected 6 stages, got %d", len(report.Stages))
	}
	for _, s := range report.Stages {
		if s.Status != StatusGreen {
			t.Errorf("empty stage %d = %q, want green", s.ID, s.Status)
		}
	}
	if report.Verdict != StatusGreen {
		t.Errorf("verdict = %q, want green", report.Verdict)
	}
}

// In full / greenfield mode the cloud stages (0..3) are provisioned, hence informational: a red
// there must NOT short-circuit, so the kubernetes deploy target (stages 4..5) STILL runs and is
// still blocking. The provisioned red is REPORTED red (not rewritten to amber) but does not gate.
func TestRunnerProvisionedStageDoesNotSkipKubernetes(t *testing.T) {
	k8sRan := false
	checks := []Check{
		// Red identity stage — expected in greenfield (the identity doesn't exist yet);
		// provisioning will create it.
		stubCheck{stage: StageIdentity, name: "identity", results: []CheckResult{{ID: "i", Status: StatusRed, Message: "role missing"}}},
		// The real deploy target. Must run and must stay blocking.
		stubCheck{stage: StageKubernetes, name: "kubernetes", results: []CheckResult{{ID: "k8s", Status: StatusGreen}}, ran: &k8sRan},
	}
	r := NewRunner(checks)
	report := r.RunWithProvisionedStages(context.Background(),
		StageIdentity, StageKMS, StageSecrets, StageNetwork)

	if !k8sRan {
		t.Fatal("kubernetes (deploy target) check did not run — provisioned cloud red wrongly short-circuited")
	}
	// The provisioned identity stage keeps its TRUE severity (red) in the report — it is NOT
	// rewritten to amber. It is just marked non-blocking.
	if report.Stages[StageIdentity].Status != StatusRed {
		t.Errorf("identity stage Status = %q, want red (true severity preserved, not rewritten)", report.Stages[StageIdentity].Status)
	}
	if report.Stages[StageIdentity].Blocking {
		t.Error("identity stage should be non-blocking (informational) in full mode")
	}
	if !report.Stages[StageIdentity].Informational {
		t.Error("identity stage should carry informational:true in full mode")
	}
	// The individual CheckResult is also still red — the operator sees the true gap.
	if report.Stages[StageIdentity].Results[0].Status != StatusRed {
		t.Errorf("identity result Status = %q, want red (not downgraded)", report.Stages[StageIdentity].Results[0].Status)
	}
	if report.Stages[StageKubernetes].Status != StatusGreen {
		t.Errorf("kubernetes stage = %q, want green", report.Stages[StageKubernetes].Status)
	}
	// Overall: the only red is in a non-blocking stage, masked to amber in the verdict → amber.
	if report.Verdict != StatusAmber {
		t.Errorf("verdict = %q, want amber (non-blocking red masked)", report.Verdict)
	}
}

// A provisioned stage's red is masked from the verdict (non-blocking), but a red in a BLOCKING
// stage (kubernetes) still short-circuits and still produces a red verdict.
func TestRunnerProvisionedModeStillBlocksOnKubernetesRed(t *testing.T) {
	workloadRan := false
	checks := []Check{
		stubCheck{stage: StageIdentity, name: "identity", results: []CheckResult{{ID: "i", Status: StatusRed}}},
		stubCheck{stage: StageKubernetes, name: "kubernetes", results: []CheckResult{{ID: "k8s", Status: StatusRed, Message: "no NetworkPolicy support"}}},
		stubCheck{stage: StageWorkload, name: "workload", results: []CheckResult{{ID: "w", Status: StatusGreen}}, ran: &workloadRan},
	}
	r := NewRunner(checks)
	report := r.RunWithProvisionedStages(context.Background(),
		StageIdentity, StageKMS, StageSecrets, StageNetwork)

	if report.Verdict != StatusRed {
		t.Errorf("verdict = %q, want red (kubernetes stage is blocking even in full mode)", report.Verdict)
	}
	if report.Stages[StageWorkload].Status != StatusSkipped {
		t.Errorf("workload stage = %q, want skipped after kubernetes red", report.Stages[StageWorkload].Status)
	}
	if workloadRan {
		t.Error("workload check ran despite kubernetes red")
	}
}
