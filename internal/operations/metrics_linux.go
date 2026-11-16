//go:build linux

package operations

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func readCPUTimes() (uint64, uint64, error) {
	file, e := os.Open("/proc/stat")
	if e != nil {
		return 0, 0, e
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if e = scanner.Err(); e != nil {
			return 0, 0, e
		}
		return 0, 0, errHostMetricsUnavailable
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, errHostMetricsUnavailable
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return 0, 0, parseErr
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return total, idle, nil
}

func readMemory() (int64, int64, error) {
	file, e := os.Open("/proc/meminfo")
	if e != nil {
		return 0, 0, e
	}
	defer file.Close()
	values := map[string]int64{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr == nil {
			values[key] = value * 1024
		}
		if len(values) > 8 {
			break
		}
	}
	total := values["MemTotal"]
	available, ok := values["MemAvailable"]
	if !ok {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if total <= 0 {
		return 0, 0, errHostMetricsUnavailable
	}
	return max(0, total-available), total, scanner.Err()
}

func readLoad1() (float64, error) {
	raw, e := os.ReadFile("/proc/loadavg")
	if e != nil {
		return 0, e
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, errHostMetricsUnavailable
	}
	return strconv.ParseFloat(fields[0], 64)
}
