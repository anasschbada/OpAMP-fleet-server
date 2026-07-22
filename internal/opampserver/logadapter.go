package opampserver

import (
	"context"
	"fmt"
	"log/slog"

	clientTypes "github.com/open-telemetry/opamp-go/client/types"
)

// slogAdapter satisfies opamp-go's client/types.Logger interface (just
// Debugf/Errorf) by forwarding to our structured slog.Logger, so the
// library's own internal logging (malformed frames, connection errors)
// ends up in the same JSON log stream as the rest of the server instead of
// going to a second, differently-formatted output.
type slogAdapter struct{ log *slog.Logger }

// NewLogAdapter wraps a *slog.Logger for use as opamp-go's server logger.
func NewLogAdapter(log *slog.Logger) clientTypes.Logger {
	return slogAdapter{log: log}
}

func (a slogAdapter) Debugf(ctx context.Context, format string, v ...any) {
	a.log.Debug(fmt.Sprintf(format, v...))
}

func (a slogAdapter) Errorf(ctx context.Context, format string, v ...any) {
	a.log.Error(fmt.Sprintf(format, v...))
}
