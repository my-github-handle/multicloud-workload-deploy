// Package cloud holds per-cloud preflight provider implementations for the cloud-facing stages
// (0 identity, 1 kms, 2 secrets, 3 network). Real AWS/GCP/Azure implementations are added per
// cloud; a fake lives in ./fake for testing without cloud credentials.
//
// The interface and result type are owned by the preflight package (the consumer), and aliased
// here so cloud-provider code reads against the cloud.* names while the dependency flows one way
// (cloud → preflight, never the reverse).
package cloud

import "github.com/ops-dev/multicloud-workload-deploy/operator/internal/preflight"

// CheckResult is exactly preflight.CheckResult.
type CheckResult = preflight.CheckResult

// PreflightProvider is exactly preflight.PreflightProvider — the cloud-facing stage interface.
type PreflightProvider = preflight.PreflightProvider
