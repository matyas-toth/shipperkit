package deploy

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"

	"shipper/internal/store"
)

// DatabasePasswordKey names the variable holding the password applications
// connect with. SQLite has none.
func DatabasePasswordKey(engine string) string {
	return map[string]string{
		"postgresql": "POSTGRES_PASSWORD",
		"mysql":      "MYSQL_PASSWORD",
		"mariadb":    "MARIADB_PASSWORD",
		"mongodb":    "MONGO_INITDB_ROOT_PASSWORD",
		"clickhouse": "CLICKHOUSE_PASSWORD",
		"redis":      "REDIS_PASSWORD",
	}[engine]
}

var databasePassword = regexp.MustCompile(`^[A-Za-z0-9]{24,128}$`)

// WithDatabasePassword returns the configuration carrying a new password.
func WithDatabasePassword(c store.Config, password string) (store.Config, error) {
	if c.Database == nil {
		return c, errors.New("Choose a managed database")
	}
	key := DatabasePasswordKey(c.Database.Engine)
	if key == "" {
		return c, errors.New("This database has no password to rotate")
	}
	c.Variables = append([]store.Variable(nil), c.Variables...)
	for i := range c.Variables {
		if c.Variables[i].Name == key {
			c.Variables[i].Value = password
			return c, nil
		}
	}
	return c, errors.New("The database password variable is missing")
}

// SetDatabasePassword changes the stored password of the application user in a
// running database. ClickHouse and Redis read theirs from the environment and
// command on every start, so the redeploy that follows applies them.
func (r *DockerRuntime) SetDatabasePassword(ctx context.Context, d store.Deployment, password string) error {
	if d.Config.Database == nil {
		return errors.New("Choose a managed database")
	}
	engine := d.Config.Database.Engine
	if engine == "clickhouse" || engine == "redis" {
		return nil
	}
	if !databasePassword.MatchString(password) {
		return errors.New("Generate a new database password")
	}
	env := databaseEnvironment(d.Config)
	userKey, _ := DatabaseIdentityKeys(engine)
	user := env[userKey]
	if engine == "postgresql" && user == "" {
		user = "postgres"
	}
	if !databaseIdentifier.MatchString(user) {
		return errors.New("Rotate this database user's password manually")
	}
	id, e := r.databaseContainer(ctx, d, store.BackupSpec{})
	if e != nil {
		return e
	}
	switch engine {
	case "postgresql":
		// The local socket trusts the container's own user; the password never
		// reaches a command line.
		script := "\\getenv shipper_password SHIPPER_PASSWORD\nALTER ROLE CURRENT_USER PASSWORD :'shipper_password';\n"
		return r.protectionExec(ctx, id, []string{"sh", "-c", `exec psql -X -q -v ON_ERROR_STOP=1 --username="$POSTGRES_USER" --dbname="$POSTGRES_DB"`}, []string{"SHIPPER_PASSWORD=" + password}, strings.NewReader(script), io.Discard)
	case "mysql", "mariadb":
		tool, root := "mysql", env["MYSQL_ROOT_PASSWORD"]
		if engine == "mariadb" {
			tool, root = "mariadb", env["MARIADB_ROOT_PASSWORD"]
		}
		// Both values are checked identifiers, so quoting cannot be escaped.
		script := "ALTER USER '" + user + "'@'%' IDENTIFIED BY '" + password + "';\nFLUSH PRIVILEGES;\n"
		return r.protectionExec(ctx, id, []string{tool, "--user=root"}, []string{"MYSQL_PWD=" + root}, strings.NewReader(script), io.Discard)
	case "mongodb":
		c := store.BackupSpec{Host: "127.0.0.1", User: user, Password: env["MONGO_INITDB_ROOT_PASSWORD"]}
		return r.protectionExec(ctx, id, []string{"sh", "-c", `exec mongosh "$MONGO_URI" --quiet --norc --eval 'db.changeUserPassword(process.env.SHIPPER_USER, process.env.SHIPPER_PASSWORD)'`}, []string{"MONGO_URI=" + mongoConnection(c, "admin"), "SHIPPER_USER=" + user, "SHIPPER_PASSWORD=" + password}, nil, io.Discard)
	}
	return errors.New("This database has no password to rotate")
}
