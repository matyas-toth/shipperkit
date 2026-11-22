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
		w.log(*d, "The database no longer answers on "+previous)
		return nil
	}
	port := strconv.Itoa(store.DatabaseDomainPorts[d.Config.Database.Engine])
	w.log(*d, "The database answers on "+next+":"+port)
	return nil
}
