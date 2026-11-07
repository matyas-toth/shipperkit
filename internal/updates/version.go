package updates

import (
	"strconv"
	"strings"
)

type version struct {
	major, minor, patch int
	pre                 []string
}

// Newer reports whether candidate is a higher release than current.
// It accepts the historical "<release>-edge.<sha>" nightly form by treating
// it as "<release>.nightly.<sha>".
func Newer(candidate, current string) bool {
	a, ok := parseVersion(candidate)
	if !ok {
		return false
	}
	b, ok := parseVersion(current)
	if !ok {
		return false
	}
	return compareVersion(a, b) > 0
}

func parseVersion(value string) (version, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.Replace(value, "-edge.", ".nightly.", 1)
	if i := strings.IndexByte(value, '+'); i >= 0 {
		value = value[:i]
	}
	core := value
	var pre []string
	if i := strings.IndexByte(value, '-'); i >= 0 {
		core = value[:i]
		pre = strings.Split(value[i+1:], ".")
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	numbers := make([]int, 3)
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || part == "" {
			return version{}, false
		}
		numbers[i] = number
	}
	for _, identifier := range pre {
		if identifier == "" {
			return version{}, false
		}
	}
	return version{major: numbers[0], minor: numbers[1], patch: numbers[2], pre: pre}, true
}

func compareVersion(a, b version) int {
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if result := compareIdentifier(a.pre[i], b.pre[i]); result != 0 {
			return result
		}
	}
	if len(a.pre) < len(b.pre) {
		return -1
	}
	if len(a.pre) > len(b.pre) {
		return 1
	}
	return 0
}

func compareIdentifier(a, b string) int {
	an, aNumeric := numericIdentifier(a)
	bn, bNumeric := numericIdentifier(b)
	switch {
	case aNumeric && bNumeric:
		if an < bn {
			return -1
		}
		if an > bn {
			return 1
		}
		return 0
	case aNumeric:
		return -1
	case bNumeric:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func numericIdentifier(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	number, err := strconv.ParseInt(value, 10, 64)
	return number, err == nil
}
