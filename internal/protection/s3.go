package protection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

type S3 struct {
	Endpoint, Region, Bucket, Prefix, AccessKey, SecretKey string
	Private                                                bool
}

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
var objectPart = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (s S3) Validate() error {
	u, e := url.Parse(s.Endpoint)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("Use an S3 endpoint without a path or credentials")
	}
	if u.Scheme != "https" && !(s.Private && u.Scheme == "http") {
		return errors.New("S3 requires HTTPS; private endpoints require explicit opt-in")
	}
	if !bucketName.MatchString(s.Bucket) || s.Region == "" || len(s.Region) > 64 || s.AccessKey == "" || s.SecretKey == "" {
		return errors.New("Enter the S3 bucket, region and credentials")
	}
	if s.Prefix != "" {
		for _, v := range strings.Split(strings.Trim(s.Prefix, "/"), "/") {
			if !objectPart.MatchString(v) {
				return errors.New("Use letters, numbers, hyphens or underscores in the object prefix")
			}
		}
	}
	return nil
}
func (s S3) Key(instance, id string) string {
	prefix := strings.Trim(s.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return prefix + "shipper/" + instance + "/" + id + ".age"
}
func (s S3) request(ctx context.Context, method, key string, body io.Reader, n int64, hash string) (*http.Response, error) {
	if e := s.Validate(); e != nil {
		return nil, e
	}
	u, _ := url.Parse(s.Endpoint)
	u.Path = "/" + s.Bucket + "/" + key
	req, e := http.NewRequestWithContext(ctx, method, u.String(), body)
	if e != nil {
		return nil, e
	}
	req.ContentLength = n
	req.Header.Set("X-Amz-Content-Sha256", hash)
	if e = v4.NewSigner().SignHTTP(ctx, aws.Credentials{AccessKeyID: s.AccessKey, SecretAccessKey: s.SecretKey}, req, hash, "s3", s.Region, time.Now()); e != nil {
		return nil, e
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: 15 * time.Second, ResponseHeaderTimeout: 60 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, e
		}
		for _, v := range ips {
			if !s.Private && (v.IP.IsPrivate() || v.IP.IsLoopback() || v.IP.IsLinkLocalUnicast() || v.IP.IsUnspecified() || v.IP.IsMulticast()) {
				return nil, errors.New("Private S3 addresses require explicit opt-in")
			}
		}
		var last error
		for _, v := range ips {
			conn, e := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(v.IP.String(), port))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}}
	defer transport.CloseIdleConnections()
	c := &http.Client{Transport: transport, Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("S3 redirects are not followed; configure the correct regional endpoint")
	}}
	res, e := c.Do(req)
	if e != nil {
		return nil, e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("S3 %s failed with status %d", method, res.StatusCode)
	}
	return res, nil
}
func (s S3) Put(ctx context.Context, key, path string) error {
	n, h, e := digest(path)
	if e != nil {
		return e
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	res, e := s.request(ctx, "PUT", key, f, n, h)
	if e != nil {
		return e
	}
	return res.Body.Close()
}
func (s S3) Get(ctx context.Context, key, path, checksum string) error {
	empty := sha256.Sum256(nil)
	res, e := s.request(ctx, "GET", key, nil, 0, hex.EncodeToString(empty[:]))
	if e != nil {
		return e
	}
	defer res.Body.Close()
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, MaxBytes+(16<<20)+1))
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if n > MaxBytes+(16<<20) || hex.EncodeToString(h.Sum(nil)) != checksum {
		return errors.New("Off-server backup failed its checksum check")
	}
	return nil
}
func (s S3) Delete(ctx context.Context, key string) error {
	empty := sha256.Sum256(nil)
	res, e := s.request(ctx, "DELETE", key, nil, 0, hex.EncodeToString(empty[:]))
	if e != nil {
		return e
	}
	return res.Body.Close()
}
