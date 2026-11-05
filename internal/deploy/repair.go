package deploy

import "context"

// RepairProxy runs only through the leased maintenance worker. Existing runtime
// checks refuse foreign resources and incompatible static configuration.
func (w *Worker) RepairProxy(ctx context.Context) error {
	if e := w.Runtime.Ensure(ctx); e != nil {
		return e
	}
	return w.routes(ctx, "", nil)
}
