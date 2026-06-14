// Package preflight implements the staged preflight checks (identity → kms → secrets → network →
// kubernetes → workload). It produces a single green/amber/red Report that the Terraform
// preflight module consumes via the hashicorp external data source.
package preflight

// Status is the outcome of a check, a stage, or the whole report.
type Status string

const (
	// StatusGreen means the check passed.
	StatusGreen Status = "green"
	// StatusAmber means a documented gap; deployment may proceed with a warning.
	StatusAmber Status = "amber"
	// StatusRed means a blocking failure; deployment must stop.
	StatusRed Status = "red"
	// StatusSkipped means the stage did not run (a prior stage was red).
	StatusSkipped Status = "skipped"
)

// StageID identifies one of the six ordered stages (0..5).
type StageID int

const (
	StageIdentity   StageID = 0 // Stage 0 — identity & access
	StageKMS        StageID = 1 // Stage 1 — KMS / encryption keys
	StageSecrets    StageID = 2 // Stage 2 — secrets backend
	StageNetwork    StageID = 3 // Stage 3 — network / egress
	StageKubernetes StageID = 4 // Stage 4 — Kubernetes infra
	StageWorkload   StageID = 5 // Stage 5 — workload readiness
)

// CheckResult is the outcome of a single check.
type CheckResult struct {
	// ID is a stable identifier, e.g. "iam.missing", "k8s.cilium".
	ID string `json:"id"`
	// Status is green, amber, or red. (A CheckResult is never "skipped";
	// only stages are skipped.)
	Status Status `json:"status"`
	// Message is a human-readable description of what was found.
	Message string `json:"message"`
	// Remediation tells the operator how to fix an amber/red result.
	Remediation string `json:"remediation,omitempty"`
}

// Stage is one of the six ordered preflight stages and its results.
type Stage struct {
	ID      StageID       `json:"id"`
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Results []CheckResult `json:"results"`
	// Blocking reports whether this stage gates the deploy. In agnostic / BYOC mode every stage
	// is blocking. In full / greenfield mode the provisioned cloud stages (0..3) are NON-blocking
	// (Blocking=false): their Status is kept at its TRUE computed value in the report (a disabled
	// BYO key is still red), but ComputeVerdict and the short-circuit logic IGNORE a non-blocking
	// stage's severity — it neither flips the top-level verdict nor skips later stages. The zero
	// value is false, so the Runner sets it explicitly; tests that build Stages by hand must set
	// Blocking:true to gate.
	Blocking bool `json:"blocking"`
	// Informational is the inverse human-facing annotation: true on a non-blocking stage whose
	// results are reported-but-not-gated. It is !Blocking, carried explicitly so the JSON report
	// is self-describing.
	Informational bool `json:"informational,omitempty"`
}

// Report is the full staged preflight output — the contract serialized into report_json and
// surfaced to the operator.
type Report struct {
	Verdict Status  `json:"verdict"`
	Stages  []Stage `json:"stages"`
}

// worse returns the more severe of two statuses, treating skipped as benign.
// Precedence (most→least severe): red > amber > green; skipped is ignored.
func worse(a, b Status) Status {
	rank := func(s Status) int {
		switch s {
		case StatusRed:
			return 3
		case StatusAmber:
			return 2
		case StatusGreen:
			return 1
		default: // skipped or unknown
			return 0
		}
	}
	if rank(a) >= rank(b) {
		if a == StatusSkipped {
			return StatusGreen
		}
		return a
	}
	if b == StatusSkipped {
		return StatusGreen
	}
	return b
}

// StageStatus folds a slice of CheckResults into a single stage status:
// red if any red; else amber if any amber; else green.
func StageStatus(results []CheckResult) Status {
	out := StatusGreen
	for _, r := range results {
		out = worse(out, r.Status)
	}
	return out
}

// ComputeVerdict folds the stage statuses into the top-level verdict: red if any BLOCKING stage
// red; else amber if any blocking stage amber; else green. A non-blocking (informational) stage
// contributes its severity at most as amber — it can never make the verdict red, but a real
// informational gap still warrants amber visibility — and skipped stages do not affect the
// verdict.
//
// This is the blocking-mask model: full-mode does NOT rewrite a provisioned stage's red to amber
// inline (which would corrupt the report by masking e.g. a disabled BYO key). The Stage keeps its
// true Status; only its gating effect is masked here.
func (r Report) ComputeVerdict() Status {
	out := StatusGreen
	for _, s := range r.Stages {
		eff := s.Status
		if !s.Blocking {
			// Mask the gating effect: a non-blocking red surfaces as at most amber in the
			// verdict, never red, and never short-circuits.
			if eff == StatusRed {
				eff = StatusAmber
			}
		}
		out = worse(out, eff)
	}
	return out
}
