package store

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"shipper/internal/auth"
)

var sourceID = regexp.MustCompile(`^[a-f0-9]{24}$`)

func SourceExcluded(name string) bool {
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		lower := strings.ToLower(part)
		if lower == ".git" || lower == "node_modules" || lower == ".shipper" || lower == ".ssh" || lower == ".aws" || lower == ".env" || strings.HasPrefix(lower, ".env.") && lower != ".env.example" && lower != ".env.sample" && lower != ".env.template" || lower == "id_rsa" || lower == "id_ed25519" || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") {
			return true
		}
	}
	return false
}
func (s *Store) SourcePath(id string) (string, error) {
	if !sourceID.MatchString(id) {
		return "", errors.New("Invalid source identity")
	}
	return filepath.Join(filepath.Dir(s.file), "sources", id+".tar"), nil
}

// AcceptSource normalizes regular files into a bounded immutable tar. No user
// path is extracted here; validation is repeated when the worker materializes it.
func (s *Store) AcceptSource(ctx context.Context, reader io.Reader, owner, app, provenance string) (v SourceSnapshot, err error) {
	if len(provenance) > 256 {
		return v, errors.New("Source provenance is too long")
	}
	var count int
	var usage int64
	if err = s.db.QueryRowContext(ctx, "SELECT count(*),COALESCE(sum(bytes),0) FROM source_snapshots").Scan(&count, &usage); err != nil {
		return
	}
	if count >= 1000 || usage >= 896<<20 {
		return v, errors.New("Local-source quota reached; review retention before uploading")
	}
	v = SourceSnapshot{ID: auth.Token()[:24], OwnerID: owner, ApplicationID: app, Provenance: provenance}
	file, _ := s.SourcePath(v.ID)
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return v, err
	}
	committed := false
	defer func() {
		f.Close()
		if !committed {
			os.Remove(file)
		}
	}()
	digest := sha256.New()
	writer := tar.NewWriter(io.MultiWriter(f, digest))
	defer writer.Close()
	input := tar.NewReader(io.LimitReader(reader, 70<<20))
	seen := map[string]bool{}
	for {
		if err = ctx.Err(); err != nil {
			return
		}
		var h *tar.Header
		h, err = input.Next()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		name := path.Clean(h.Name)
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg || name == "." || name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) || strings.ContainsAny(name, "\\:\x00") || len(name) > 512 || seen[name] || SourceExcluded(name) {
			err = fmt.Errorf("Excluded or unsafe source file: %s", name)
			return
		}
		v.Files++
		v.Bytes += h.Size
		if h.Size < 0 || v.Files > 10000 || v.Bytes > 64<<20 {
			err = errors.New("Source is limited to 64 MiB and 10000 files")
			return
		}
		seen[name] = true
		if err = writer.WriteHeader(&tar.Header{Name: name, Mode: h.Mode & 0755, Size: h.Size, Typeflag: tar.TypeReg}); err != nil {
			return
		}
		if _, err = io.CopyN(writer, input, h.Size); err != nil {
			return
		}
	}
	if v.Files == 0 {
		err = errors.New("The source contains no files")
		return
	}
	if err = writer.Close(); err != nil {
		return
	}
	if info, e := f.Stat(); e == nil {
		v.Bytes = info.Size()
	} else {
		err = e
		return
	}
	if err = f.Sync(); err != nil {
		return
	}
	if err = f.Close(); err != nil {
		return
	}
	v.Digest = hex.EncodeToString(digest.Sum(nil))
	err = s.SaveSourceSnapshot(ctx, v)
	committed = err == nil
	return
}
func (s *Store) MaterializeSource(ctx context.Context, id, dir string) (string, error) {
	v, e := s.SourceSnapshot(ctx, id)
	if e != nil {
		return "", errors.New("Local source is unavailable; upload it again")
	}
	file, e := s.SourcePath(id)
	if e != nil {
		return "", e
	}
	f, e := os.Open(file)
	if e != nil {
		return "", e
	}
	defer f.Close()
	hash := sha256.New()
	if _, e = io.Copy(hash, io.LimitReader(f, 70<<20)); e != nil {
		return "", e
	}
	if hex.EncodeToString(hash.Sum(nil)) != v.Digest {
		return "", errors.New("Local source integrity check failed")
	}
	f.Seek(0, 0)
	root := filepath.Join(dir, "repository")
	if e = os.MkdirAll(root, 0700); e != nil {
		return "", e
	}
	fs, e := os.OpenRoot(root)
	if e != nil {
		return "", e
	}
	defer fs.Close()
	reader := tar.NewReader(f)
	total := int64(0)
	files := 0
	for {
		h, e := reader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
		name := path.Clean(h.Name)
		if h.Typeflag != tar.TypeReg || SourceExcluded(name) || strings.ContainsAny(name, "\\:\x00") || name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return "", errors.New("Invalid local source archive")
		}
		files++
		total += h.Size
		if files > 10000 || total > 64<<20 || h.Size < 0 {
			return "", errors.New("Local source exceeds its limits")
		}
		if e = fs.MkdirAll(path.Dir(name), 0700); e != nil {
			return "", e
		}
		out, e := fs.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0755)
		if e != nil {
			return "", e
		}
		_, e = io.CopyN(out, reader, h.Size)
		closeErr := out.Close()
		if e != nil {
			return "", e
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return v.Digest, nil
}
