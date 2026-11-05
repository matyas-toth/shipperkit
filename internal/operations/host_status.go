package operations

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

type HostStatus struct {
	ObservedAt             int64 `json:"observedAt"`
	DockerAtBoot           bool  `json:"dockerAtBoot"`
	ShipperAtBoot          bool  `json:"shipperAtBoot"`
	RebootRequired         bool  `json:"rebootRequired"`
	SecurityServiceEnabled bool  `json:"securityServiceEnabled"`
	ClockSynchronized      bool  `json:"clockSynchronized"`
	Stale                  bool  `json:"stale"`
}

func InstalledHostStatus() *HostStatus {
	if os.Getenv("SHIPPER_MANAGED_INSTALL") != "1" {
		return nil
	}
	file, e := os.Open("/run/shipper-host/status.json")
	if e != nil {
		return nil
	}
	defer file.Close()
	var v HostStatus
	if e = json.NewDecoder(io.LimitReader(file, 4096)).Decode(&v); e != nil || v.ObservedAt <= 0 {
		return nil
	}
	v.Stale = time.Now().Unix()-v.ObservedAt > 180
	return &v
}
