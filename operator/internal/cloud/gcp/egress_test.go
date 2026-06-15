package gcp

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func strp(s string) *string { return &s }

func TestCheckEgress(t *testing.T) {
	tests := []struct {
		name       string
		getNet     func(context.Context, *computepb.GetNetworkRequest) (*computepb.Network, error)
		listNat    func(context.Context, string, string, string) ([]*computepb.RouterNat, error)
		wantStatus preflight.Status
	}{
		{
			name: "network resolves with a Cloud NAT -> green",
			getNet: func(_ context.Context, _ *computepb.GetNetworkRequest) (*computepb.Network, error) {
				return &computepb.Network{SelfLink: strp("https://www.googleapis.com/compute/v1/projects/demo/global/networks/demo-vpc")}, nil
			},
			listNat: func(_ context.Context, _, _, _ string) ([]*computepb.RouterNat, error) {
				return []*computepb.RouterNat{{Name: strp("demo-nat")}}, nil
			},
			wantStatus: preflight.StatusGreen,
		},
		{
			name: "network resolves but no Cloud NAT -> amber",
			getNet: func(_ context.Context, _ *computepb.GetNetworkRequest) (*computepb.Network, error) {
				return &computepb.Network{SelfLink: strp("https://www.googleapis.com/compute/v1/projects/demo/global/networks/demo-vpc")}, nil
			},
			listNat: func(_ context.Context, _, _, _ string) ([]*computepb.RouterNat, error) {
				return nil, nil
			},
			wantStatus: preflight.StatusAmber,
		},
		{
			name: "network not found -> red",
			getNet: func(_ context.Context, _ *computepb.GetNetworkRequest) (*computepb.Network, error) {
				return nil, errors.New("notFound")
			},
			wantStatus: preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				ProjectID:       "demo",
				Region:          "us-central1",
				NetworkSelfLink: "https://www.googleapis.com/compute/v1/projects/demo/global/networks/demo-vpc",
				RouterName:      "demo-router",
				Compute:         stubCompute{getNet: tt.getNet, listNat: tt.listNat},
			}
			res := p.CheckEgress(context.Background())
			if len(res) == 0 {
				t.Fatal("expected a result")
			}
			if res[0].Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (msg: %s)", res[0].Status, tt.wantStatus, res[0].Message)
			}
		})
	}
}
