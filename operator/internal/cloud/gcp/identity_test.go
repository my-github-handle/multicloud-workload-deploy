package gcp

import (
	"context"
	"errors"
	"testing"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func TestCheckIdentityPermissions(t *testing.T) {
	tests := []struct {
		name       string
		test       func(context.Context, string, []string) ([]string, error)
		wantStatus preflight.Status
	}{
		{
			name: "all required permissions granted -> green",
			test: func(_ context.Context, _ string, perms []string) ([]string, error) {
				return perms, nil // every requested permission is held
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name: "a required permission missing -> red",
			test: func(_ context.Context, _ string, perms []string) ([]string, error) {
				// Drop the first permission to simulate a missing grant.
				if len(perms) <= 1 {
					return nil, nil
				}
				return perms[1:], nil
			},
			wantStatus: preflight.StatusRed,
		},
		{
			name: "testIamPermissions error -> red",
			test: func(_ context.Context, _ string, _ []string) ([]string, error) {
				return nil, errors.New("PermissionDenied")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				ProjectID: "demo",
				Region:    "us-central1",
				IAM:       stubIAM{test: tt.test},
			}
			res := p.CheckIdentityPermissions(context.Background())
			if len(res) == 0 {
				t.Fatal("expected a result")
			}
			if res[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (msg: %s)", res[0].Status, tt.wantStatus, res[0].Message)
			}
		})
	}
}
