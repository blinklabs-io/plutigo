//go:build !unix

package replay

// processCPUNanos reports no CPU time where getrusage is unavailable, which
// makes BenchmarkMainnetEvalOnly omit its cpu-ns/op metric.
func processCPUNanos() int64 {
	return 0
}
