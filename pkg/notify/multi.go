package notify

import (
	"context"
	"errors"
)

// Multi dispatches each event to every underlying Notifier and joins any errors.
type Multi []Notifier

// Name implements Notifier.
func (m Multi) Name() string { return "multi" }

// Notify invokes every non-nil notifier in order and combines errors with errors.Join.
func (m Multi) Notify(ctx context.Context, event Event) error {
	var errs []error
	for _, n := range m {
		if n == nil {
			continue
		}
		if err := n.Notify(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
