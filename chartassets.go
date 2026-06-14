// Package mcwd is the module-root package; its sole job is to embed the repo-root
// charts/ directory. A go:embed pattern is relative to the embedding file's own
// directory and cannot escape it, so only a file at the repo root can embed charts/.
// operator/internal/chartfs re-exports this FS as the stable import surface.
package mcwd

import "embed"

// WorkloadChartFS holds the embedded charts/workload directory (the single source of the
// workload child objects, shared by Tier A render and Tier B helm_release).
//
//go:embed all:charts/workload
var WorkloadChartFS embed.FS
