package chat

import (
	"context"
	"errors"
)

var errCancelled = errors.New("cancelled by user")

func cancelledByUser(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errCancelled)
}
