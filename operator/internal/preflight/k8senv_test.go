package preflight_test

import (
	"os"
	"testing"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// testCfg is the rest.Config for the envtest API server, shared by all k8s-stage tests in this
// package. It is nil if envtest could not start (e.g. KUBEBUILDER_ASSETS unset), in which case
// the k8s tests self-skip.
var testCfg *rest.Config

func TestMain(m *testing.M) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		testCfg = nil
		os.Exit(m.Run())
	}
	testCfg = cfg
	code := m.Run()
	_ = env.Stop()
	os.Exit(code)
}
