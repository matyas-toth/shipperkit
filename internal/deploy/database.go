package deploy

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"shipper/internal/store"
)

type DatabaseEndpoint struct {
	Engine string `json:"engine"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

// DatabaseEndpointFor uses the serving release, never an unapplied configuration.
// The release alias already exists on the instance's shared Docker network.
func DatabaseEndpointFor(d store.Deployment) *DatabaseEndpoint {
	if ReleaseMethod(d) != "image" || d.ID == "" || d.ContainerID == "" || d.TaskCommand != nil {
		return nil
	}
	engine, port := databaseImage(d.Config.Image)
	if engine == "" {
		return nil
	}
	endpoint := &DatabaseEndpoint{Engine: engine, Host: "site-" + d.ID, Port: port}
	if d.RuntimePort > 0 {
		endpoint.Port = d.RuntimePort
	}
	return endpoint
}

func databaseImage(image string) (string, int) {
	ref, err := ImageReference(image)
	if err != nil {
		return "", 0
	}
	switch ref.Name() {
	case "docker.io/library/postgres":
		return "postgresql", 5432
	case "docker.io/library/mysql":
		return "mysql", 3306
	case "docker.io/library/mariadb":
		return "mariadb", 3306
	case "docker.io/library/mongo":
		return "mongodb", 27017
	case "docker.io/library/redis", "docker.io/valkey/valkey":
		return "redis", 6379
	default:
		return "", 0
	}
}

func databaseCapabilities(d store.Deployment) []string {
	if ReleaseMethod(d) != "image" || d.TaskCommand != nil {
		return nil
	}
	engine, _ := databaseImage(d.Config.Image)
	if engine == "" {
		return nil
	}
	// Official database entrypoints initialize owned data directories and drop
	// from root to their service user. Keep all other capabilities dropped.
	return []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID"}
}

func DatabaseURI(d store.Deployment) (string, error) {
	endpoint := DatabaseEndpointFor(d)
	if endpoint == nil {
		return "", errors.New("This release is not a supported database image")
	}
	if d.Config.StartCommand != "" {
		return "", errors.New("Custom database commands require a manually configured connection string")
	}
	env := map[string]string{}
	for _, v := range d.Config.Variables {
		if v.Scope == "" || v.Scope == "runtime" || v.Scope == "both" {
			env[v.Name] = v.Value
		}
	}
	// File-mounted credentials cannot be inferred from the saved environment.
	for _, key := range []string{"POSTGRES_PASSWORD_FILE", "POSTGRES_USER_FILE", "POSTGRES_DB_FILE", "MYSQL_PASSWORD_FILE", "MYSQL_ROOT_PASSWORD_FILE", "MARIADB_PASSWORD_FILE", "MARIADB_ROOT_PASSWORD_FILE", "MONGO_INITDB_ROOT_USERNAME_FILE", "MONGO_INITDB_ROOT_PASSWORD_FILE"} {
		if env[key] != "" {
			return "", errors.New("File-based database credentials require a manually configured connection string")
		}
	}
	u := &url.URL{Scheme: endpoint.Engine, Host: net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port))}
	user, password, database := "", "", ""
	switch endpoint.Engine {
	case "postgresql":
		user = env["POSTGRES_USER"]
		if user == "" {
			user = "postgres"
		}
		password = env["POSTGRES_PASSWORD"]
		if password == "" && env["POSTGRES_HOST_AUTH_METHOD"] != "trust" {
			return "", errors.New("Set POSTGRES_PASSWORD in the database environment")
		}
		database = env["POSTGRES_DB"]
		if database == "" {
			database = user
		}
	case "mysql", "mariadb":
		value := func(key string) string {
			if endpoint.Engine == "mariadb" && env["MARIADB_"+key] != "" {
				return env["MARIADB_"+key]
			}
			return env["MYSQL_"+key]
		}
		user, password, database = value("USER"), value("PASSWORD"), value("DATABASE")
		if user == "" || user == "root" {
			user, password = "root", value("ROOT_PASSWORD")
			if value("ROOT_HOST") == "localhost" || value("ROOT_HOST") == "127.0.0.1" {
				return "", errors.New("Configure a database user that allows connections from other applications")
			}
		}
		if password == "" {
			return "", errors.New("Set the database user's password in the database environment")
		}
		// MariaDB clients commonly use the MySQL URI scheme.
		u.Scheme = "mysql"
	case "mongodb":
		user, password = env["MONGO_INITDB_ROOT_USERNAME"], env["MONGO_INITDB_ROOT_PASSWORD"]
		database = env["MONGO_INITDB_DATABASE"]
		if (user == "") != (password == "") {
			return "", errors.New("Set both MongoDB root credentials in the database environment")
		}
		if user != "" {
			u.RawQuery = "authSource=admin"
		}
	case "redis":
		// Official Redis/Valkey images do not consume REDIS_PASSWORD or VALKEY_PASSWORD.
		// Auth configured through commands or mounted config must not be guessed.
		if env["REDIS_PASSWORD"] != "" || env["VALKEY_PASSWORD"] != "" {
			return "", errors.New("Redis and Valkey authentication requires a manually configured connection string")
		}
		for _, volume := range d.Config.Volumes {
			if strings.TrimRight(volume.Path, "/") != "/data" {
				return "", errors.New("Redis and Valkey configuration mounts require a manually configured connection string")
			}
		}
		database = "0"
	}
	if user != "" {
		if password != "" {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}
	}
	u.Path = "/" + database
	u.RawPath = "/" + url.PathEscape(database)
	return u.String(), nil
}
