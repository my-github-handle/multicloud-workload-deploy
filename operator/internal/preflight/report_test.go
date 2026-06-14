package preflight

import (
	"encoding/json"
	"testing"
)

func TestComputeVerdict(t *testing.T) {
	tests := []struct {
		name    string
		results []Status
		want    Status
	}{
		{"all green", []Status{StatusGreen, StatusGreen}, StatusGreen},
		{"one amber", []Status{StatusGreen, StatusAmber, StatusGreen}, StatusAmber},
		{"one red dominates amber", []Status{StatusAmber, StatusRed, StatusGreen}, StatusRed},
		{"empty is green", []Status{}, StatusGreen},
		{"skipped ignored", []Status{StatusGreen, StatusSkipped}, StatusGreen},
		{"skipped does not mask red", []Status{StatusRed, StatusSkipped}, StatusRed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stages []Stage
			for i, s := range tt.results {
				stages = append(stages, Stage{
					ID:       StageID(i),
					Name:     "stage",
					Status:   s,
					Blocking: true, // these cases test the all-blocking (agnostic) verdict
					Results:  []CheckResult{{ID: "c", Status: s, Message: "m"}},
				})
			}
			r := Report{Stages: stages}
			if got := r.ComputeVerdict(); got != tt.want {
				t.Errorf("ComputeVerdict() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A non-blocking stage that is genuinely red keeps its red Status (it is still shown red in the
// report) but does NOT force the verdict red — it surfaces as amber. A blocking red still
// dominates.
func TestComputeVerdictMasksNonBlockingRed(t *testing.T) {
	r := Report{Stages: []Stage{
		// provisioned cloud stage, genuinely red (e.g. disabled BYO key), masked.
		{ID: StageKMS, Name: "kms", Status: StatusRed, Blocking: false, Informational: true,
			Results: []CheckResult{{ID: "kms.key", Status: StatusRed, Message: "key disabled"}}},
		// blocking k8s stage, green.
		{ID: StageKubernetes, Name: "kubernetes", Status: StatusGreen, Blocking: true,
			Results: []CheckResult{{ID: "k8s.installtier", Status: StatusGreen}}},
	}}
	if got := r.ComputeVerdict(); got != StatusAmber {
		t.Errorf("ComputeVerdict() = %q, want amber (non-blocking red masked to amber)", got)
	}
	// The stage Status must remain red — the report is NOT corrupted.
	if r.Stages[0].Status != StatusRed {
		t.Errorf("non-blocking stage Status = %q, want red (true severity preserved)", r.Stages[0].Status)
	}
}

func TestReportJSONRoundTrip(t *testing.T) {
	r := Report{
		Verdict: StatusAmber,
		Stages: []Stage{
			{
				ID:            0,
				Name:          "identity",
				Status:        StatusAmber,
				Blocking:      true,
				Informational: false,
				Results: []CheckResult{
					{
						ID:          "iam.excess",
						Status:      StatusAmber,
						Message:     "deploy identity has excess permissions",
						Remediation: "remove kms:* from the policy",
					},
				},
			},
		},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Verdict != StatusAmber {
		t.Errorf("verdict = %q, want amber", got.Verdict)
	}
	if len(got.Stages) != 1 || len(got.Stages[0].Results) != 1 {
		t.Fatalf("structure not preserved: %+v", got)
	}
	if got.Stages[0].Results[0].Remediation != "remove kms:* from the policy" {
		t.Errorf("remediation lost: %q", got.Stages[0].Results[0].Remediation)
	}
}
