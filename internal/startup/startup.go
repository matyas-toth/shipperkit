package startup

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
)

//go:generate go run generate/main.go
//go:embed assets/*
var assets embed.FS

func Binary(arch string) ([]byte, error) {
	raw, e := assets.ReadFile("assets/linux-" + arch + ".gz")
	if e != nil {
		return nil, fmt.Errorf("Startup helper for %s is unavailable; build with go generate ./internal/startup", arch)
	}
	z, e := gzip.NewReader(bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	defer z.Close()
	return io.ReadAll(z)
}
