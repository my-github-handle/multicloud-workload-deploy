package azure

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"
)

func strp(s string) *string { return &s }

func TestCheckEgress(t *testing.T) {
	vnetWithUDR := func(_ context.Context, _, _ string) (armnetwork.VirtualNetworksClientGetResponse, error) {
		return armnetwork.VirtualNetworksClientGetResponse{
			VirtualNetwork: armnetwork.VirtualNetwork{
				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
					Subnets: []*armnetwork.Subnet{
						{
							Name: strp("nodes"),
							Properties: &armnetwork.SubnetPropertiesFormat{
								RouteTable: &armnetwork.RouteTable{ID: strp("/subscriptions/x/.../routeTables/rt")},
							},
						},
					},
				},
			},
		}, nil
	}
	vnetNoUDR := func(_ context.Context, _, _ string) (armnetwork.VirtualNetworksClientGetResponse, error) {
		return armnetwork.VirtualNetworksClientGetResponse{
			VirtualNetwork: armnetwork.VirtualNetwork{
				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
					Subnets: []*armnetwork.Subnet{
						{Name: strp("nodes"), Properties: &armnetwork.SubnetPropertiesFormat{}},
					},
				},
			},
		}, nil
	}

	tests := []struct {
		name       string
		get        func(context.Context, string, string) (armnetwork.VirtualNetworksClientGetResponse, error)
		wantStatus preflight.Status
	}{
		{"vnet with UDR egress path -> green", vnetWithUDR, preflight.StatusGreen},
		{"vnet without route table -> amber", vnetNoUDR, preflight.StatusAmber},
		{
			"get error -> red",
			func(_ context.Context, _, _ string) (armnetwork.VirtualNetworksClientGetResponse, error) {
				return armnetwork.VirtualNetworksClientGetResponse{}, errors.New("NotFound")
			},
			preflight.StatusRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				VNetID: "/subscriptions/x/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet",
				VNets:  stubVNets{get: tt.get},
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
