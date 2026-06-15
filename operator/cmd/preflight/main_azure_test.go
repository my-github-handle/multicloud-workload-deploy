package main

import (
	"testing"

	azureprovider "github.com/ops-dev/multicloud-workload-deploy/operator/internal/cloud/azure"
)

// TestSelectProviderAzureReturnsRealProvider asserts the real Azure provider is
// wired into selectProvider (it previously returned a not-implemented error).
func TestSelectProviderAzureReturnsRealProvider(t *testing.T) {
	p, err := selectProvider("azure")
	if err != nil {
		t.Fatalf("selectProvider(\"azure\") returned error: %v", err)
	}
	if _, ok := p.(*azureprovider.Provider); !ok {
		t.Fatalf("expected *azure.Provider, got %T", p)
	}
}
