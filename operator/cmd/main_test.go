package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

// TestBuildManagerOptionsNamespaceScoped asserts that passing a watch namespace restricts the
// controller cache to exactly that namespace, which is what makes the controller
// namespace-scoped rather than cluster-wide.
func TestBuildManagerOptionsNamespaceScoped(t *testing.T) {
	s := runtime.NewScheme()
	opts := buildManagerOptions(s, ":8080", "demo")

	if got := len(opts.Cache.DefaultNamespaces); got != 1 {
		t.Fatalf("expected cache scoped to 1 namespace, got %d", got)
	}
	if _, ok := opts.Cache.DefaultNamespaces["demo"]; !ok {
		t.Errorf("expected cache scoped to namespace %q, got %v", "demo", opts.Cache.DefaultNamespaces)
	}
}

// TestBuildManagerOptionsClusterWide asserts that an empty watch namespace leaves the cache
// cluster-wide (no DefaultNamespaces restriction).
func TestBuildManagerOptionsClusterWide(t *testing.T) {
	s := runtime.NewScheme()
	opts := buildManagerOptions(s, ":8080", "")

	if len(opts.Cache.DefaultNamespaces) != 0 {
		t.Errorf("expected cluster-wide cache (no DefaultNamespaces), got %v", opts.Cache.DefaultNamespaces)
	}
}
