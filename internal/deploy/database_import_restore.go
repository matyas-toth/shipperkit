package deploy

import (
	"context"
	"io"
	"os"
	"shipper/internal/auth"
	"shipper/internal/store"
)

// Untrusted native SQL executes as a temporary principal scoped to the new copy.
// The application database remains outside that principal's privileges.
func (r *DockerRuntime) restoreNativeDatabase(ctx context.Context, id string, c store.BackupSpec, file string) error {
	adminArgs, adminEnv := dbArgs(c)
	role := "shipper_import_" + auth.Token()[:16]
	password := auth.Token()
	run := func(args, env []string, in io.Reader) error {
		return r.protectionExec(ctx, id, args, env, in, io.Discard)
	}
	limited := c
	limited.User = role
	limited.Password = password
	args, env := dbArgs(limited)
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	if c.Engine == "postgres" {
		sql := func(db, statement string) error {
			return run(append([]string{"psql", "--no-psqlrc", "--set=ON_ERROR_STOP=1", "--dbname=" + db, "--command=" + statement}, adminArgs...), adminEnv, nil)
		}
		if e = sql(c.Database, "CREATE ROLE "+role+" LOGIN PASSWORD '"+password+"' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION"); e != nil {
			return e
		}
		defer func() { _ = sql(c.Database, "ALTER ROLE "+role+" NOLOGIN"); _ = sql(c.Database, "DROP ROLE "+role) }()
		if e = run(append([]string{"createdb", "--owner=" + role}, append(adminArgs, "--", c.Target)...), adminEnv, nil); e != nil {
			return e
		}
		if e = run(append([]string{"pg_restore", "--exit-on-error", "--single-transaction", "--no-owner", "--no-acl", "--dbname=" + c.Target}, args...), env, f); e != nil {
			return e
		}
		// Transfer imported objects to the database owner before removing the login.
		if e = sql(c.Target, "REASSIGN OWNED BY "+role+" TO CURRENT_USER; ALTER DATABASE \""+c.Target+"\" OWNER TO CURRENT_USER"); e != nil {
			return e
		}
		return sql(c.Target, "SELECT count(*) FROM information_schema.tables")
	}
	tool := "mysql"
	if c.Engine == "mariadb" {
		tool = "mariadb"
	}
	sql := func(statement string) error {
		return run(append([]string{tool}, append(adminArgs, "--execute="+statement)...), adminEnv, nil)
	}
	if e = sql("CREATE DATABASE `" + c.Target + "`"); e != nil {
		return e
	}
	if e = sql("CREATE USER '" + role + "'@'127.0.0.1' IDENTIFIED BY '" + password + "'; GRANT ALL PRIVILEGES ON `" + c.Target + "`.* TO '" + role + "'@'127.0.0.1'"); e != nil {
		return e
	}
	defer func() { _ = sql("DROP USER '" + role + "'@'127.0.0.1'") }()
	if e = run(append([]string{tool, "--binary-mode=1", "--local-infile=0"}, append(args, c.Target)...), env, f); e != nil {
		return e
	}
	return sql("SELECT count(*) FROM information_schema.tables WHERE table_schema='" + c.Target + "'")
}
