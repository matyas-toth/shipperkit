//go:build ignore

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	source, e := os.Stat("../../cmd/shipper-start/main.go")
	must(e)
	for _, arch := range []string{"amd64", "arm64"} {
		if asset, e := os.Stat("assets/linux-" + arch + ".gz"); e == nil && asset.ModTime().After(source.ModTime()) {
			continue
		}
		tmp, e := os.MkdirTemp("", "shipper-start-")
		must(e)
		bin := filepath.Join(tmp, "start")
		cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-trimpath", "-ldflags=-s -w", "-o", bin, "../../cmd/shipper-start")
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		must(cmd.Run())
		raw, e := os.ReadFile(bin)
		must(e)
		var b bytes.Buffer
		z, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
		_, e = z.Write(raw)
		must(e)
		must(z.Close())
		must(os.WriteFile("assets/linux-"+arch+".gz", b.Bytes(), 0644))
		os.RemoveAll(tmp)
	}
}
func must(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
