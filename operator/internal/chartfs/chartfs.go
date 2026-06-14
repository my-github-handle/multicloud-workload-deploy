// Package chartfs is the stable import surface for the embedded single-source workload
// Helm chart. The operator renders it in-process (Tier A); Terraform renders the same
// directory on disk (Tier B). The actual go:embed lives in the repo-root package mcwd
// (chartassets.go); this package re-exports it.
package chartfs

import (
	"io/fs"

	mcwd "github.com/ops-dev/multicloud-workload-deploy"
)

// FS holds the embedded charts/workload directory, re-exported from the module-root embed.
var FS fs.FS = mcwd.WorkloadChartFS
