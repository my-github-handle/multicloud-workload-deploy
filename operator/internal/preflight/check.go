package preflight

import "context"

// stageNames maps each StageID to its human-readable name.
var stageNames = [6]string{
	StageIdentity:   "identity & access",
	StageKMS:        "kms / encryption keys",
	StageSecrets:    "secrets backend",
	StageNetwork:    "network / egress",
	StageKubernetes: "kubernetes infra",
	StageWorkload:   "workload readiness",
}

// Check is one unit of preflight validation, bound to a single stage. A stage may contain
// multiple Checks; their results are concatenated.
type Check interface {
	// Stage returns which of the six stages this check belongs to.
	Stage() StageID
	// Name is a human-readable label for diagnostics.
	Name() string
	// Run executes the check and returns its results. It must not panic; failures are reported as
	// red CheckResults, not errors.
	Run(ctx context.Context) []CheckResult
}

// Runner sequences Checks into the six ordered stages and assembles a Report.
type Runner struct {
	// byStage groups checks by their stage in execution order.
	byStage [6][]Check
}

// NewRunner buckets the supplied checks by stage. Order within a stage is the order supplied;
// stages always run 0..5.
func NewRunner(checks []Check) *Runner {
	r := &Runner{}
	for _, c := range checks {
		id := c.Stage()
		if id < 0 || int(id) >= len(r.byStage) {
			continue
		}
		r.byStage[id] = append(r.byStage[id], c)
	}
	return r
}

// Run executes stages 0..5 in order with ALL stages blocking (agnostic / BYOC mode): as soon as a
// stage is red, every later stage is marked skipped and its checks are not executed. The returned
// Report has its Verdict computed.
func (r *Runner) Run(ctx context.Context) Report {
	return r.run(ctx, [6]bool{}) // no informational stages → all blocking
}

// RunWithProvisionedStages runs full / greenfield mode: the supplied stages are treated as
// INFORMATIONAL because <cloud>-full provisions the resources they check (the stages it satisfies
// by provisioning are informational rather than blocking). "Informational" means
// reported-at-true-severity-but-not-gating — it does NOT rewrite severities. An informational
// stage:
//   - still runs its checks (so the report is complete),
//   - keeps each CheckResult and the Stage.Status at its TRUE computed value (a disabled BYO key
//     in a -full run is STILL shown red, with informational:true), and
//   - is marked Blocking=false, so ComputeVerdict masks its gating effect (red → at most amber in
//     the verdict) and the Runner NEVER short-circuits on it — the blocking stages that follow
//     (notably the kubernetes deploy target, stages 4..5) ALWAYS execute.
func (r *Runner) RunWithProvisionedStages(ctx context.Context, provisioned ...StageID) Report {
	var informational [6]bool
	for _, id := range provisioned {
		if id >= 0 && int(id) < len(informational) {
			informational[id] = true
		}
	}
	return r.run(ctx, informational)
}

// run is the shared core. informational[id]==true marks a stage NON-blocking: its true Status is
// preserved in the report, but it does not short-circuit and ComputeVerdict masks its gating
// effect.
func (r *Runner) run(ctx context.Context, informational [6]bool) Report {
	report := Report{Stages: make([]Stage, 6)}
	shortCircuited := false

	for id := 0; id < 6; id++ {
		blocking := !informational[id]
		stage := Stage{
			ID:            StageID(id),
			Name:          stageNames[id],
			Blocking:      blocking,
			Informational: !blocking,
		}

		if shortCircuited {
			stage.Status = StatusSkipped
			stage.Results = nil
			report.Stages[id] = stage
			continue
		}

		var results []CheckResult
		for _, c := range r.byStage[id] {
			results = append(results, c.Run(ctx)...)
		}

		// Results keep their TRUE severity here — no inline rewrite. A red in an informational
		// stage stays red in the report; only its gating effect is masked (Blocking=false →
		// ComputeVerdict + short-circuit ignore it).
		stage.Results = results
		stage.Status = StageStatus(results)
		report.Stages[id] = stage

		// Only a BLOCKING red short-circuits the remaining stages. An informational (provisioned)
		// red must NOT skip the kubernetes deploy target — it will be closed by provisioning.
		if blocking && stage.Status == StatusRed {
			shortCircuited = true
		}
	}

	report.Verdict = report.ComputeVerdict()
	return report
}
