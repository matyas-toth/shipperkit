package deploy

import (
	"context"

	"shipper/internal/store"
)

// reusePortGuess starts from the port the serving release answered on when
// nobody chose one, so a detected port is not rediscovered on every deploy.
// A wrong hint costs nothing: the health check detects the listener again.
func (w *Worker) reusePortGuess(ctx context.Context, d *store.Deployment) {
	if d.Operation == "rollback" || d.PreviousID == "" || !PortGuessed(*d) || d.RuntimePort != 3000 {
		return
	}
	previous, e := w.DB.Deployment(ctx, d.PreviousID)
	if e != nil || previous.State != "succeeded" || !PortGuessed(previous) || previous.RuntimePort <= 0 || previous.RuntimePort == d.RuntimePort {
		return
	}
	d.RuntimePort = previous.RuntimePort
}

// addImageVolumes keeps the paths an image declares with VOLUME on persistent
// volumes and records them on the application, where they show up like any
// other volume.
func (w *Worker) addImageVolumes(ctx context.Context, d *store.Deployment) error {
	method := ReleaseMethod(*d)
	if d.TaskCommand != nil || d.ImageID == "" || method == "static" || method == "compose" || d.Config.Database != nil {
		return nil
	}
	runtime, ok := w.Runtime.(interface {
		ImageVolumes(context.Context, store.Deployment, func(string)) ([]store.Volume, error)
	})
	if !ok {
		return nil
	}
	volumes, e := runtime.ImageVolumes(ctx, *d, func(line string) { w.log(*d, line) })
	if e != nil || len(volumes) == 0 {
		return e
	}
	if e = w.DB.AddVolumes(ctx, d.ApplicationID, d.ID, volumes); e != nil {
		return e
	}
	for _, v := range volumes {
		d.Config.Volumes = append(d.Config.Volumes, v)
		w.log(*d, "Keeping "+v.Path+" on the persistent volume "+v.Name)
	}
	return nil
}
