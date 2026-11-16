//go:build !linux

package operations

func readCPUTimes() (uint64, uint64, error) { return 0, 0, errHostMetricsUnavailable }

func readMemory() (int64, int64, error) { return 0, 0, errHostMetricsUnavailable }

func readLoad1() (float64, error) { return 0, errHostMetricsUnavailable }
