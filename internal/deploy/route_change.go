package deploy

import (
	"context"
	"errors"
	"strconv"
	"time"

	"shipper/internal/store"
)

// route applies a change that lives entirely in the proxy. The serving release
// keeps its containers; only its recorded public domain and the routes change.
func (w *Worker) route(ctx context.Context, d *store.Deployment) error {
	if e := w.stage(ctx, d, "routing"); e != nil {
		return e
	}
	release, e := w.DB.Deployment(ctx, d.PreviousID)
	if e != nil {
		return e
	}
	if release.Config.Database == nil || d.Config.Database == nil {
		return errors.New("Database settings are unavailable; redeploy the database")
	}
	previous, next := release.Config.Database.Domain, d.Config.Database.Domain
	w.log(*d, "Updating the public domain without restarting the database")
	setDomain := func(ctx context.Context, domain string) error {
		return w.DB.UpdateDeploymentConfig(ctx, release.ID, func(c *store.Config) {
			if c.Database != nil {
				c.Database.Domain = domain
			}
		})
	}
	if e = setDomain(ctx, next); e != nil {
		return e
	}
	if e = w.routes(ctx, "", nil); e != nil {
		rollback, stop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer stop()
		if restore := setDomain(rollback, previous); restore == nil {
			w.routes(rollback, "", nil)
		}
		return e
	}
	if next == "" {
		w.log(*d, "The database no longer answers publicly on "+previous)
		return nil
	}
	port := strconv.Itoa(store.DatabaseDomainPorts[d.Config.Database.Engine])
	w.log(*d, "The database answers publicly on "+next+":"+port)
	return nil
}

// checkingMessage names the check a release is about to pass. Databases and
// workers only have to keep running; everything else has to answer HTTP.
func checkingMessage(d store.Deployment) string {
	if d.Config.StartupPolicy == "process" {
		return "Checking that the new release keeps running"
	}
	return "Checking the candidate HTTP response"
}

// databaseAddresses lists where a database release answers: always on the
// private network, and on its public domain when it has one.
func databaseAddresses(d store.Deployment) []string {
	port := d.Config.Port
	if d.RuntimePort > 0 {
		port = d.RuntimePort
	}
	if port == 0 {
		return []string{"The database is running"}
	}
	lines := []string{"The database answers privately on db-" + d.ApplicationID + ":" + strconv.Itoa(port)}
	if d.Config.Database.Domain != "" && store.DatabaseDomainPorts[d.Config.Database.Engine] != 0 {
		lines = append(lines, "The database answers publicly on "+d.Config.Database.Domain+":"+strconv.Itoa(store.DatabaseDomainPorts[d.Config.Database.Engine]))
	}
	return lines
}
