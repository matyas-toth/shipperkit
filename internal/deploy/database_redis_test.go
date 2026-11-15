package deploy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedisSnapshotExcludesReplicationTrailer(t *testing.T) {
	payload := []byte("REDIS0012-binary-snapshot\x00\xff")
	marker := strings.Repeat("0123456789", 4)
	for _, test := range []struct {
		name, trailer, diagnostic string
		valid                     bool
	}{
		{"diskless replication", marker, fmt.Sprintf("Transfer finished with success after %d bytes\n", len(payload)), true},
		{"length-delimited replication", "", "Transfer finished with success.\n", true},
		{"truncated trailer", marker[:39], fmt.Sprintf("Transfer finished with success after %d bytes\n", len(payload)), false},
		{"missing completion", marker, "I/O Error reading RDB payload from socket\n", false},
		{"invalid byte count", marker, "Transfer finished with success after -1 bytes\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "snapshot.rdb")
			file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err = file.Write(append(bytes.Clone(payload), []byte(test.trailer)...)); err != nil {
				t.Fatal(err)
			}
			size, err := finishRedisSnapshot(file, test.diagnostic)
			if (err == nil) != test.valid {
				t.Fatalf("Completion accepted=%v, expected %v: %v", err == nil, test.valid, err)
			}
			if !test.valid {
				return
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, payload) || size != int64(len(payload)) {
				t.Fatalf("RDB payload changed or retained transport bytes: size=%d error=%v", size, err)
			}
		})
	}
}
