package main

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

type curated struct {
	ID          string
	Name        string
	Description string
	Category    string
	Tags        []string
	Docs        string
	Compose     string
	Main        string
	Port        int
	Fields      []field
	Domains     []domain
	Support     string
	SupportNote string
}

// Curated services cover the primitives people reach for most often. They use
// the same manifest contract as converted templates, with deliberate labels
// and credentials instead of generated variable names.
func curatedServices() []service {
	items := []curated{
		{
			ID: "postgresql", Name: "PostgreSQL", Description: "Reliable relational database with JSON support and extensions.",
			Category: "Databases", Tags: []string{"sql", "postgres", "database"}, Docs: "https://www.postgresql.org/docs/",
			Main: "postgres", Port: 5432, Support: "ready",
			Compose: `services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-app}
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - type: volume
        source: postgres-data
        target: /var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 5s
      timeout: 5s
      retries: 12
volumes:
  postgres-data:
`,
			Fields: []field{
				{Key: "POSTGRES_DB", Label: "Database", Kind: "text", Group: "configuration", Default: "app", Required: true},
				{Key: "POSTGRES_USER", Label: "Username", Kind: "text", Group: "access", Default: "postgres", Required: true},
				{Key: "POSTGRES_PASSWORD", Label: "Password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
		},
		{
			ID: "mysql", Name: "MySQL", Description: "Popular relational database for web applications.",
			Category: "Databases", Tags: []string{"sql", "mysql", "database"}, Docs: "https://dev.mysql.com/doc/",
			Main: "mysql", Port: 3306, Support: "ready",
			Compose: `services:
  mysql:
    image: mysql:8.4
    environment:
      MYSQL_DATABASE: ${MYSQL_DATABASE:-app}
      MYSQL_USER: ${MYSQL_USER:-app}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
    volumes:
      - type: volume
        source: mysql-data
        target: /var/lib/mysql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "-p$${MYSQL_ROOT_PASSWORD}"]
      interval: 5s
      timeout: 5s
      retries: 12
volumes:
  mysql-data:
`,
			Fields: []field{
				{Key: "MYSQL_DATABASE", Label: "Database", Kind: "text", Group: "configuration", Default: "app", Required: true},
				{Key: "MYSQL_USER", Label: "Username", Kind: "text", Group: "access", Default: "app", Required: true},
				{Key: "MYSQL_PASSWORD", Label: "Password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "MYSQL_ROOT_PASSWORD", Label: "Root password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
		},
		{
			ID: "mariadb", Name: "MariaDB", Description: "Open source relational database compatible with MySQL.",
			Category: "Databases", Tags: []string{"sql", "mariadb", "database"}, Docs: "https://mariadb.com/kb/en/documentation/",
			Main: "mariadb", Port: 3306, Support: "ready",
			Compose: `services:
  mariadb:
    image: mariadb:11.4
    environment:
      MARIADB_DATABASE: ${MARIADB_DATABASE:-app}
      MARIADB_USER: ${MARIADB_USER:-app}
      MARIADB_PASSWORD: ${MARIADB_PASSWORD}
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD}
    volumes:
      - type: volume
        source: mariadb-data
        target: /var/lib/mysql
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 5s
      timeout: 5s
      retries: 12
volumes:
  mariadb-data:
`,
			Fields: []field{
				{Key: "MARIADB_DATABASE", Label: "Database", Kind: "text", Group: "configuration", Default: "app", Required: true},
				{Key: "MARIADB_USER", Label: "Username", Kind: "text", Group: "access", Default: "app", Required: true},
				{Key: "MARIADB_PASSWORD", Label: "Password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "MARIADB_ROOT_PASSWORD", Label: "Root password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
		},
		{
			ID: "mongodb", Name: "MongoDB", Description: "Document database for flexible application data.",
			Category: "Databases", Tags: []string{"document", "mongodb", "database"}, Docs: "https://www.mongodb.com/docs/",
			Main: "mongodb", Port: 27017, Support: "ready",
			Compose: `services:
  mongodb:
    image: mongo:7
    environment:
      MONGO_INITDB_DATABASE: ${MONGO_DATABASE:-app}
      MONGO_INITDB_ROOT_USERNAME: ${MONGO_USER:-mongo}
      MONGO_INITDB_ROOT_PASSWORD: ${MONGO_PASSWORD}
    volumes:
      - type: volume
        source: mongodb-data
        target: /data/db
    healthcheck:
      test: ["CMD-SHELL", "mongosh --quiet --eval 'db.adminCommand({ping:1}).ok' --username $${MONGO_USER} --password $${MONGO_PASSWORD} --authenticationDatabase admin"]
      interval: 8s
      timeout: 6s
      retries: 12
volumes:
  mongodb-data:
`,
			Fields: []field{
				{Key: "MONGO_DATABASE", Label: "Database", Kind: "text", Group: "configuration", Default: "app", Required: true},
				{Key: "MONGO_USER", Label: "Username", Kind: "text", Group: "access", Default: "mongo", Required: true},
				{Key: "MONGO_PASSWORD", Label: "Password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
		},
		{
			ID: "redis", Name: "Redis", Description: "In-memory cache and queue with optional persistence.",
			Category: "Databases", Tags: []string{"cache", "redis", "queue"}, Docs: "https://redis.io/docs/latest/",
			Main: "redis", Port: 6379, Support: "ready",
			Compose: `services:
  redis:
    image: redis:7-alpine
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "${REDIS_PASSWORD}"]
    environment:
      REDIS_PASSWORD: ${REDIS_PASSWORD}
    volumes:
      - type: volume
        source: redis-data
        target: /data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "$${REDIS_PASSWORD}", "ping"]
      interval: 5s
      timeout: 4s
      retries: 12
volumes:
  redis-data:
`,
			Fields: []field{
				{Key: "REDIS_PASSWORD", Label: "Password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
			SupportNote: "Connect apps over the private network",
		},
		{
			ID: "clickhouse", Name: "ClickHouse", Description: "Column-oriented database for real-time analytics.",
			Category: "Databases", Tags: []string{"analytics", "clickhouse", "sql"}, Docs: "https://clickhouse.com/docs",
			Main: "clickhouse", Port: 8123, Support: "ready",
			Compose: `services:
  clickhouse:
    image: clickhouse/clickhouse-server:24.8-alpine
    environment:
      CLICKHOUSE_DB: ${CLICKHOUSE_DB:-analytics}
      CLICKHOUSE_USER: ${CLICKHOUSE_USER:-default}
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD}
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: "1"
    volumes:
      - type: volume
        source: clickhouse-data
        target: /var/lib/clickhouse
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://127.0.0.1:8123/ping"]
      interval: 8s
      timeout: 5s
      retries: 12
volumes:
  clickhouse-data:
`,
			Fields: []field{
				{Key: "CLICKHOUSE_DB", Label: "Database", Kind: "text", Group: "configuration", Default: "analytics", Required: true},
				{Key: "CLICKHOUSE_USER", Label: "Username", Kind: "text", Group: "access", Default: "default", Required: true},
				{Key: "CLICKHOUSE_PASSWORD", Label: "Password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
			Domains: []domain{{Service: "clickhouse", Port: 8123, Primary: true}},
		},
		{
			ID: "prometheus", Name: "Prometheus", Description: "Metrics collection and alerting for your services.",
			Category: "Monitoring", Tags: []string{"metrics", "monitoring", "prometheus"}, Docs: "https://prometheus.io/docs/",
			Main: "prometheus", Port: 9090, Support: "ready",
			Compose: `services:
  prometheus:
    image: prom/prometheus:v3.1.0
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --storage.tsdb.path=/prometheus
      - --web.enable-lifecycle
    volumes:
      - type: volume
        source: prometheus-data
        target: /prometheus
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://127.0.0.1:9090/-/healthy"]
      interval: 10s
      timeout: 5s
      retries: 10
volumes:
  prometheus-data:
`,
			Fields:  []field{},
			Domains: []domain{{Service: "prometheus", Port: 9090, Primary: true}},
		},
		{
			ID: "loki", Name: "Loki", Description: "Log aggregation with a compact single-binary mode.",
			Category: "Monitoring", Tags: []string{"logs", "monitoring", "loki"}, Docs: "https://grafana.com/docs/loki/latest/",
			Main: "loki", Port: 3100, Support: "ready",
			Compose: `services:
  loki:
    image: grafana/loki:3.3.2
    command: ["-config.file=/etc/loki/local-config.yaml"]
    volumes:
      - type: volume
        source: loki-data
        target: /loki
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://127.0.0.1:3100/ready"]
      interval: 10s
      timeout: 5s
      retries: 10
volumes:
  loki-data:
`,
			Fields:  []field{},
			Domains: []domain{{Service: "loki", Port: 3100, Primary: true}},
		},
		{
			ID: "minio", Name: "MinIO", Description: "S3-compatible object storage for backups and media.",
			Category: "Storage", Tags: []string{"s3", "storage", "objects"}, Docs: "https://min.io/docs/minio/container/index.html",
			Main: "minio", Port: 9001, Support: "ready",
			Compose: `services:
  minio:
    image: minio/minio:RELEASE.2025-02-07T23-21-09Z
    command: ["server", "/data", "--console-address", ":9001"]
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    volumes:
      - type: volume
        source: minio-data
        target: /data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 10
volumes:
  minio-data:
`,
			Fields: []field{
				{Key: "MINIO_ROOT_USER", Label: "Access key", Kind: "text", Group: "access", Default: "minioadmin", Required: true},
				{Key: "MINIO_ROOT_PASSWORD", Label: "Secret key", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
			Domains: []domain{{Service: "minio", Port: 9001, Primary: true}},
		},
		{
			ID: "outline", Name: "Outline", Description: "Team wiki and knowledge base with a clean editor.",
			Category: "Productivity", Tags: []string{"wiki", "docs", "knowledge"}, Docs: "https://docs.getoutline.com/s/hosting",
			Main: "outline", Port: 3000, Support: "advanced",
			SupportNote: "Connect an OIDC provider after deploy",
			Compose: `services:
  outline:
    image: outline/outline:0.81.0
    environment:
      NODE_ENV: production
      DATABASE_URL: postgres://outline:${POSTGRES_PASSWORD}@postgres:5432/outline
      PGSSLMODE: disable
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379
      SECRET_KEY: ${OUTLINE_SECRET_KEY}
      UTILS_SECRET: ${OUTLINE_UTILS_SECRET}
      URL: ${SERVICE_FQDN_OUTLINE}
      PORT: "3000"
      FILE_STORAGE: local
      OIDC_CLIENT_ID: ${OIDC_CLIENT_ID}
      OIDC_CLIENT_SECRET: ${OIDC_CLIENT_SECRET}
      OIDC_AUTH_URI: ${OIDC_AUTH_URI}
      OIDC_TOKEN_URI: ${OIDC_TOKEN_URI}
      OIDC_USERINFO_URI: ${OIDC_USERINFO_URI}
      OIDC_USERNAME_CLAIM: preferred_username
      OIDC_DISPLAY_NAME: SSO
    volumes:
      - type: volume
        source: outline-data
        target: /var/lib/outline/data
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: outline
      POSTGRES_USER: outline
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - type: volume
        source: outline-postgres
        target: /var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U outline -d outline"]
      interval: 5s
      timeout: 5s
      retries: 12
  redis:
    image: redis:7-alpine
    command: ["redis-server", "--requirepass", "${REDIS_PASSWORD}"]
    environment:
      REDIS_PASSWORD: ${REDIS_PASSWORD}
    volumes:
      - type: volume
        source: outline-redis
        target: /data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "$${REDIS_PASSWORD}", "ping"]
      interval: 5s
      timeout: 4s
      retries: 12
volumes:
  outline-data:
  outline-postgres:
  outline-redis:
`,
			Fields: []field{
				{Key: "OUTLINE_SECRET_KEY", Label: "Secret key", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "OUTLINE_UTILS_SECRET", Label: "Utilities secret", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "POSTGRES_PASSWORD", Label: "Database password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "REDIS_PASSWORD", Label: "Cache password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "OIDC_CLIENT_ID", Label: "OIDC client ID", Kind: "text", Group: "configuration", Required: true},
				{Key: "OIDC_CLIENT_SECRET", Label: "OIDC client secret", Kind: "password", Group: "configuration", Secret: true, Required: true},
				{Key: "OIDC_AUTH_URI", Label: "Authorization URL", Kind: "url", Group: "configuration", Placeholder: "https://id.example.com/authorize", Required: true},
				{Key: "OIDC_TOKEN_URI", Label: "Token URL", Kind: "url", Group: "configuration", Placeholder: "https://id.example.com/token", Required: true},
				{Key: "OIDC_USERINFO_URI", Label: "User info URL", Kind: "url", Group: "configuration", Placeholder: "https://id.example.com/userinfo", Required: true},
			},
			Domains: []domain{{Key: "SERVICE_FQDN_OUTLINE", Service: "outline", Port: 3000, Primary: true}},
		},
		{
			ID: "calcom", Name: "Cal.com", Description: "Scheduling infrastructure for teams and products.",
			Category: "Productivity", Tags: []string{"scheduling", "calendar", "meetings"}, Docs: "https://cal.com/docs",
			Main: "calcom", Port: 3000, Support: "ready",
			Compose: `services:
  calcom:
    image: calcom/cal.com:5.6.4
    environment:
      NODE_ENV: production
      NEXT_PUBLIC_WEBAPP_URL: ${SERVICE_FQDN_CALCOM}
      NEXTAUTH_URL: ${SERVICE_FQDN_CALCOM}/api/auth
      DATABASE_URL: postgres://calcom:${POSTGRES_PASSWORD}@postgres:5432/calcom
      DATABASE_DIRECT_URL: postgres://calcom:${POSTGRES_PASSWORD}@postgres:5432/calcom
      NEXTAUTH_SECRET: ${NEXTAUTH_SECRET}
      CALENDSO_ENCRYPTION_KEY: ${CALENDSO_ENCRYPTION_KEY}
      NEXT_PUBLIC_LICENSE_CONSENT: "agree"
      CALCOM_TELEMETRY_DISABLED: "1"
    depends_on:
      postgres:
        condition: service_healthy
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: calcom
      POSTGRES_USER: calcom
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - type: volume
        source: calcom-postgres
        target: /var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U calcom -d calcom"]
      interval: 5s
      timeout: 5s
      retries: 12
volumes:
  calcom-postgres:
`,
			Fields: []field{
				{Key: "NEXTAUTH_SECRET", Label: "Auth secret", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "CALENDSO_ENCRYPTION_KEY", Label: "Encryption key", Kind: "password", Group: "access", Secret: true, Generate: "hex", Required: true},
				{Key: "POSTGRES_PASSWORD", Label: "Database password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
			Domains: []domain{{Key: "SERVICE_FQDN_CALCOM", Service: "calcom", Port: 3000, Primary: true}},
		},
		{
			ID: "wordpress", Name: "WordPress", Description: "Publishing platform with a familiar editor and plugin ecosystem.",
			Category: "CMS", Tags: []string{"cms", "blog", "wordpress"}, Docs: "https://wordpress.org/documentation/",
			Main: "wordpress", Port: 80, Support: "ready",
			Compose: `services:
  wordpress:
    image: wordpress:6.7-php8.2-apache
    environment:
      WORDPRESS_DB_HOST: mysql:3306
      WORDPRESS_DB_NAME: wordpress
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: ${MYSQL_PASSWORD}
      WORDPRESS_CONFIG_EXTRA: |
        define('WP_HOME', '${SERVICE_FQDN_WORDPRESS}');
        define('WP_SITEURL', '${SERVICE_FQDN_WORDPRESS}');
    volumes:
      - type: volume
        source: wordpress-data
        target: /var/www/html
    depends_on:
      mysql:
        condition: service_healthy
  mysql:
    image: mysql:8.4
    environment:
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
    volumes:
      - type: volume
        source: wordpress-mysql
        target: /var/lib/mysql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "-p$${MYSQL_ROOT_PASSWORD}"]
      interval: 5s
      timeout: 5s
      retries: 12
volumes:
  wordpress-data:
  wordpress-mysql:
`,
			Fields: []field{
				{Key: "MYSQL_PASSWORD", Label: "Database password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
				{Key: "MYSQL_ROOT_PASSWORD", Label: "Root password", Kind: "password", Group: "access", Secret: true, Generate: "password", Required: true},
			},
			Domains: []domain{{Key: "SERVICE_FQDN_WORDPRESS", Service: "wordpress", Port: 80, Primary: true}},
		},
	}
	out := make([]service, 0, len(items))
	for _, item := range items {
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(item.Compose), &doc); err != nil {
			panic(fmt.Sprintf("curated %s: %v", item.ID, err))
		}
		name := displayName(item.ID)
		out = append(out, service{
			ID: item.ID, Name: item.Name, Description: item.Description, Category: item.Category,
			Tags: item.Tags, Logomark: logomark(item.Name), Logo: "/service-logos/" + item.ID + ".svg", Documentation: item.Docs,
			Source: "shipper", Featured: true, MainService: item.Main, Port: item.Port, HealthPath: "/",
			Support: item.Support, SupportNote: item.SupportNote, Compose: strings.TrimSpace(item.Compose),
			Services: serviceInfos(doc, item.Main), Fields: item.Fields, Domains: item.Domains,
		})
		_ = name
	}
	return out
}
