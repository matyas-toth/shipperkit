package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"shipper/internal/store"
	"strconv"
	"time"
)

func databaseSerial() *big.Int {
	n, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		panic(e)
	}
	return n
}
func (r *DockerRuntime) databaseTLSRoot(app string) (string, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`).MatchString(app) || r.DataDir == "" {
		return "", errors.New("Database TLS requires a private application data directory")
	}
	return filepath.Join(r.DataDir, "database-tls", app), nil
}
func (r *DockerRuntime) databaseCertificate(app string) (map[string][]byte, error) {
	root, e := r.databaseTLSRoot(app)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	certPath, keyPath := filepath.Join(root, "ca.crt"), filepath.Join(root, "ca.key")
	certPEM, ce := os.ReadFile(certPath)
	keyPEM, ke := os.ReadFile(keyPath)
	var ca *x509.Certificate
	var key *ecdsa.PrivateKey
	if ce == nil && ke == nil {
		cb, _ := pem.Decode(certPEM)
		kb, _ := pem.Decode(keyPEM)
		if cb == nil || kb == nil {
			return nil, errors.New("Database CA is invalid")
		}
		ca, e = x509.ParseCertificate(cb.Bytes)
		if e != nil {
			return nil, e
		}
		key, e = x509.ParseECPrivateKey(kb.Bytes)
		if e != nil {
			return nil, e
		}
	} else if os.IsNotExist(ce) && os.IsNotExist(ke) {
		key, e = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return nil, e
		}
		ca = &x509.Certificate{SerialNumber: databaseSerial(), Subject: pkix.Name{CommonName: "Shipper database " + app}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
		der, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
		if e != nil {
			return nil, e
		}
		certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		der, e = x509.MarshalECPrivateKey(key)
		if e != nil {
			return nil, e
		}
		if e = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0600); e != nil {
			return nil, e
		}
		if e = os.WriteFile(certPath, certPEM, 0600); e != nil {
			return nil, e
		}
	} else {
		return nil, errors.New("Restore both database CA certificate and private key before starting this database")
	}
	serverKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return nil, e
	}
	server := &x509.Certificate{SerialNumber: databaseSerial(), Subject: pkix.Name{CommonName: "db-" + app}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0), DNSNames: []string{"db-" + app, "app-" + app, "localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, server, ca, &serverKey.PublicKey, key)
	if e != nil {
		return nil, e
	}
	keyDER, e := x509.MarshalECPrivateKey(serverKey)
	if e != nil {
		return nil, e
	}
	return map[string][]byte{"ca.crt": certPEM, "server.crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), "server.key": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})}, nil
}
// DatabaseTransportLabel marks managed database containers whose clients
// connect over the private network without TLS. Containers created before it
// existed still require TLS until their next deployment.
const DatabaseTransportLabel = "io.shipper.database.transport"

// configureDatabaseTLS starts managed databases so private clients connect
// with a plain connection string. PostgreSQL, MySQL and MariaDB still offer
// TLS to clients that ask for it, so older strings with sslmode=require keep
// working. Public traffic is encrypted by the proxy, never by the database.
func (r *DockerRuntime) configureDatabaseTLS(ctx context.Context, d store.Deployment, c *container.Config) error {
	if d.Config.Database == nil || d.TaskCommand != nil {
		return nil
	}
	if e := ValidateDatabaseResource(d.Config); e != nil {
		return e
	}
	health := ""
	switch d.Config.Database.Engine {
	case "postgresql":
		health = `PGPASSWORD="$POSTGRES_PASSWORD" psql --host=127.0.0.1 --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -qAtc 'SELECT 1' >/dev/null`
		c.Cmd = []string{"postgres", "-c", "ssl=on", "-c", "ssl_cert_file=/shipper-tls/server.crt", "-c", "ssl_key_file=/shipper-tls/server.key", "-c", "ssl_ca_file=/shipper-tls/ca.crt", "-c", "hba_file=/shipper-tls/pg_hba.conf"}
	case "mysql", "mariadb":
		health = `MYSQL_PWD="$MYSQL_PASSWORD" mysql --host=127.0.0.1 --user="$MYSQL_USER" --execute='SELECT 1' "$MYSQL_DATABASE" >/dev/null`
		binary := "mysqld"
		if d.Config.Database.Engine == "mariadb" {
			health = `MYSQL_PWD="$MARIADB_PASSWORD" mariadb --host=127.0.0.1 --user="$MARIADB_USER" --skip-ssl --execute='SELECT 1' "$MARIADB_DATABASE" >/dev/null`
			binary = "mariadbd"
		}
		c.Cmd = []string{binary, "--ssl-ca=/shipper-tls/ca.crt", "--ssl-cert=/shipper-tls/server.crt", "--ssl-key=/shipper-tls/server.key"}
	case "redis":
		health = `REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli -h 127.0.0.1 PING | grep -qx PONG`
		c.Cmd = append([]string{"redis-server", "--appendonly", "yes", "--requirepass", databaseEnvironment(d.Config)["REDIS_PASSWORD"]}, redisCacheArgs(d.Config)...)
	default:
		c.Labels[DatabaseTransportLabel] = "plain"
		return nil
	}
	c.Labels[DatabaseTransportLabel] = "plain"
	c.Entrypoint = nil // Keep the official entrypoint's initialization and privilege drop.
	c.Healthcheck = &container.HealthConfig{Test: []string{"CMD-SHELL", health}, Interval: 2 * time.Second, Timeout: 5 * time.Second, Retries: 3, StartPeriod: 240 * time.Second}
	return nil
}

// DatabaseLegacyTLS reports a managed database container that still requires
// TLS on the private network because it was created before plain connections.
func (r *DockerRuntime) DatabaseLegacyTLS(ctx context.Context, d store.Deployment) bool {
	if d.Config.Database == nil || d.ContainerID == "" || (d.Config.Database.Engine != "postgresql" && d.Config.Database.Engine != "redis") {
		return false
	}
	info, e := r.Client.ContainerInspect(ctx, d.ContainerID, client.ContainerInspectOptions{})
	if e != nil || info.Container.Config == nil {
		return false
	}
	return info.Container.Config.Labels[DatabaseTransportLabel] == ""
}
func (r *DockerRuntime) seedDatabaseTLS(ctx context.Context, d store.Deployment, id string) error {
	if d.Config.Database == nil || d.TaskCommand != nil || !databaseTLSCapable(d.Config.Database.Engine) {
		return nil
	}
	files, e := r.databaseCertificate(d.ApplicationID)
	if e != nil {
		return e
	}
	if d.Config.Database.Engine == "postgresql" {
		// "host" matches TLS and plain connections alike.
		files["pg_hba.conf"] = []byte("local all all trust\nhost all all all scram-sha-256\n")
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if e = tw.WriteHeader(&tar.Header{Name: "shipper-tls", Typeflag: tar.TypeDir, Mode: 0755, Uid: 999, Gid: 999}); e != nil {
		return e
	}
	for name, data := range files {
		mode := int64(0644)
		if name == "server.key" {
			mode = 0600
		}
		if e = tw.WriteHeader(&tar.Header{Name: "shipper-tls/" + name, Mode: mode, Size: int64(len(data)), Uid: 999, Gid: 999}); e != nil {
			return e
		}
		if _, e = tw.Write(data); e != nil {
			return e
		}
	}
	if e = tw.Close(); e != nil {
		return e
	}
	_, e = r.Client.CopyToContainer(ctx, id, client.CopyToContainerOptions{DestinationPath: "/", Content: &buf, CopyUIDGID: true})
	return e
}

// redisCacheArgs leave a quarter of the memory limit for Redis itself, so
// eviction starts well before the kernel kills the container.
func redisCacheArgs(c store.Config) []string {
	if c.Database == nil || !c.Database.Cache {
		return nil
	}
	memory := c.MemoryMB
	if memory <= 0 {
		memory = 512
	}
	return []string{"--maxmemory", strconv.FormatInt(memory*3/4, 10) + "mb", "--maxmemory-policy", "allkeys-lru"}
}
