package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"shipper/internal/protection"
	"shipper/internal/store"
)

var unsafeDatabaseRune = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// recoveryName derives a valid database name from the original and a suffix,
// shortening the original so the result stays within identifier limits.
func recoveryName(database, suffix string) string {
	base := unsafeDatabaseRune.ReplaceAllString(database, "_")
	if base == "" || base[0] >= '0' && base[0] <= '9' {
		base = "db_" + base
	}
	if limit := 63 - len(suffix); len(base) > limit {
		base = base[:limit]
	}
	return base + suffix
}

// ReplaceSupported reports the engines that can restore over live data.
func ReplaceSupported(engine string) bool { return engine == "postgres" || engine == "mongodb" }

// ReplaceDatabase restores a backup over the live database. The backup is
// first restored and verified in a staging database, so a broken archive never
// touches live data; the swap then keeps the previous data under a dated name.
func (r *DockerRuntime) ReplaceDatabase(ctx context.Context, d store.Deployment, c store.BackupSpec, m protection.Manifest, file, run string) (string, error) {
	if !ReplaceSupported(c.Engine) {
		return "", errors.New("Restore a copy for this database engine")
	}
	staged := recoveryName(c.Database, "_restore_"+run[max(0, len(run)-8):])
	kept := recoveryName(c.Database, "_before_"+time.Now().UTC().Format("20060102_150405"))
	c.Target = staged
	id, e := r.databaseContainer(ctx, d, c)
	if e != nil {
		return "", e
	}
	if e = r.RestoreDatabase(ctx, d, c, m, file); e != nil {
		r.dropDatabase(context.WithoutCancel(ctx), id, c, staged)
		return "", e
	}
	if c.Engine == "postgres" {
		e = r.swapPostgres(ctx, id, c, staged, kept)
	} else {
		e = r.swapMongo(ctx, id, c, staged, kept)
	}
	if e != nil {
		r.dropDatabase(context.WithoutCancel(ctx), id, c, staged)
		return "", e
	}
	return "Replaced " + c.Database + "; previous data kept in " + kept, nil
}

func quoteIdentifier(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
func quoteLiteral(value string) string   { return `'` + strings.ReplaceAll(value, `'`, `''`) + `'` }

// swapPostgres renames the live database aside and the staged copy into its
// place. Connections are refused and closed first, since PostgreSQL cannot
// rename a database in use; applications reconnect to the restored data.
func (r *DockerRuntime) swapPostgres(ctx context.Context, id string, c store.BackupSpec, staged, kept string) error {
	args, env := dbArgs(c)
	sql := func(statement string) error {
		return r.protectionExec(ctx, id, append([]string{"psql", "--no-psqlrc", "--set=ON_ERROR_STOP=1", "--dbname=postgres", "--command=" + statement}, args...), env, nil, io.Discard)
	}
	live := quoteIdentifier(c.Database)
	if e := sql("ALTER DATABASE " + live + " WITH ALLOW_CONNECTIONS false"); e != nil {
		return e
	}
	terminate := "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname IN (" + quoteLiteral(c.Database) + "," + quoteLiteral(staged) + ") AND pid <> pg_backend_pid()"
	rename := func(from, to string) error {
		var e error
		// Terminated sessions take a moment to exit; retry until the name is free.
		for attempt := 0; attempt < 40; attempt++ {
			if e = sql(terminate); e != nil {
				return e
			}
			if e = sql("ALTER DATABASE " + quoteIdentifier(from) + " RENAME TO " + quoteIdentifier(to)); e == nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		return e
	}
	if e := rename(c.Database, kept); e != nil {
		_ = sql("ALTER DATABASE " + live + " WITH ALLOW_CONNECTIONS true")
		return fmt.Errorf("The live database could not be swapped: %w", e)
	}
	if e := rename(staged, c.Database); e != nil {
		// Put the original back so the application keeps its data.
		_ = rename(kept, c.Database)
		_ = sql("ALTER DATABASE " + live + " WITH ALLOW_CONNECTIONS true")
		return fmt.Errorf("The restored database could not be swapped in: %w", e)
	}
	// The kept copy stays inspectable; the restored one now serves the application.
	_ = sql("ALTER DATABASE " + quoteIdentifier(kept) + " WITH ALLOW_CONNECTIONS true")
	return sql("ALTER DATABASE " + live + " WITH ALLOW_CONNECTIONS true")
}

// swapMongo moves the live collections aside and the restored ones into the
// live database. MongoDB cannot rename a database, so each collection moves.
func (r *DockerRuntime) swapMongo(ctx context.Context, id string, c store.BackupSpec, staged, kept string) error {
	script := `const live = process.env.SHIPPER_LIVE, staged = process.env.SHIPPER_STAGED, kept = process.env.SHIPPER_KEPT;
const names = name => db.getSiblingDB(name).getCollectionInfos({ type: "collection" }).map(c => c.name).filter(n => !n.startsWith("system."));
const move = (from, to, name) => { const r = db.adminCommand({ renameCollection: from + "." + name, to: to + "." + name }); if (!r.ok) throw new Error(r.errmsg); };
const restored = names(staged);
for (const name of names(live)) move(live, kept, name);
for (const name of restored) move(staged, live, name);
db.getSiblingDB(staged).dropDatabase();`
	env := []string{"MONGO_URI=" + mongoConnection(c, "admin"), "SHIPPER_LIVE=" + c.Database, "SHIPPER_STAGED=" + staged, "SHIPPER_KEPT=" + kept, "SHIPPER_SCRIPT=" + script}
	return r.protectionExec(ctx, id, []string{"sh", "-c", `exec mongosh "$MONGO_URI" --quiet --eval "$SHIPPER_SCRIPT"`}, env, nil, io.Discard)
}

func (r *DockerRuntime) dropDatabase(ctx context.Context, id string, c store.BackupSpec, name string) {
	if c.Engine == "postgres" {
		args, env := dbArgs(c)
		_ = r.protectionExec(ctx, id, append([]string{"dropdb", "--if-exists", "--force"}, append(args, "--", name)...), env, nil, io.Discard)
		return
	}
	env := []string{"MONGO_URI=" + mongoConnection(c, name)}
	_ = r.protectionExec(ctx, id, []string{"sh", "-c", `exec mongosh "$MONGO_URI" --quiet --eval "db.dropDatabase()"`}, env, nil, io.Discard)
}
