package dispatch

import "log/slog"

func logFailure(route string, err error) {
	slog.Default().Error("an operation failed and the caller was told internal error", "route", route, "error", err.Error())
}
