package deploy

import (
	"errors"
	"math"
	"net/http"
	"regexp"
	"shipper/internal/store"
	"strings"
)

func ValidateRuntime(p store.RuntimePolicy) error {
	if math.IsNaN(p.CPUs) || math.IsInf(p.CPUs, 0) || p.CPUs < 0 || p.CPUs > 64 {
		return errors.New("CPU limit must be between 0 and 64")
	}
	if p.PIDs != 0 && (p.PIDs < 16 || p.PIDs > 32768) {
		return errors.New("PID limit must be between 16 and 32768")
	}
	if p.Restart != "" && p.Restart != "unless-stopped" && p.Restart != "on-failure" && p.Restart != "no" {
		return errors.New("Choose a supported restart policy")
	}
	if p.StartupSeconds < 0 || p.StartupSeconds > 600 || p.StopSeconds < 0 || p.StopSeconds > 120 || p.DrainSeconds < 0 || p.DrainSeconds > 60 {
		return errors.New("Startup, stop or drain timeout is outside its range")
	}
	return nil
}
func StopSeconds(p store.RuntimePolicy) int {
	if p.StopSeconds > 0 {
		return p.StopSeconds
	}
	return 10
}
func StartupSeconds(p store.RuntimePolicy) int {
	if p.StartupSeconds > 0 {
		return p.StartupSeconds
	}
	return 45
}

var routePath = regexp.MustCompile(`^/[A-Za-z0-9/._~!$&'()*+,;=:@%-]*$`)
var headerName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)

func ValidateRoute(d store.Domain) error {
	if len(d.PathPrefix) > 256 || d.PathPrefix != "" && (!routePath.MatchString(d.PathPrefix) || strings.HasPrefix(d.PathPrefix, "//") || strings.Contains(d.PathPrefix, "..") || strings.Contains(d.PathPrefix, "%")) {
		return errors.New("Enter a plain path prefix starting with /")
	}
	if d.StripPrefix && (d.PathPrefix == "" || d.PathPrefix == "/") {
		return errors.New("Set a path prefix before stripping it")
	}
	if len(d.ResponseHeaders) > 16 {
		return errors.New("Use at most 16 response headers")
	}
	for name, value := range d.ResponseHeaders {
		canonical := http.CanonicalHeaderKey(name)
		if !headerName.MatchString(name) || len(value) > 2048 || strings.ContainsAny(value, "\r\n\x00") {
			return errors.New("Enter valid response header names and values")
		}
		switch canonical {
		case "X-Shipper-Deployment", "Set-Cookie", "Content-Length", "Transfer-Encoding", "Connection", "Location", "Upgrade", "Trailer":
			return errors.New("This response header is managed by Shipper or HTTP")
		}
	}
	return nil
}
