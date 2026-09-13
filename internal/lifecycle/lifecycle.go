package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

var (
	ErrRuntime         = errors.New("application runtime failed")
	ErrShutdown        = errors.New("application shutdown failed")
	ErrShutdownTimeout = errors.New("application shutdown timed out")
)

type ShutdownTask struct {
	Name string
	Run  func(context.Context) error
}

// NotifyContext is canceled by the first SIGINT or SIGTERM. Default signal
// handling is restored right after, so a repeated signal terminates the process.
func NotifyContext(ctx context.Context) context.Context {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop)

	return ctx
}

// Run calls run (or waits for ctx when run is nil) until ctx is canceled or run
// returns, then runs tasks in reverse order within one shared timeout.
func Run(ctx context.Context, timeout time.Duration, run func(context.Context) error, tasks ...ShutdownTask) error {
	if run == nil {
		run = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	}

	runDone := make(chan error, 1)
	go func() { runDone <- run(ctx) }()

	var runErr error
	runFinished := false
	select {
	case <-ctx.Done():
	case runErr = <-runDone:
		runFinished = true
	}

	logger := zerolog.Ctx(ctx)
	logger.Info().Dur("timeout", timeout).Msg("shutdown_started")

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	tasksFailed := make(chan bool, 1)
	go func() { tasksFailed <- runShutdownTasks(shutdownCtx, tasks) }()

	var failed bool
	select {
	case failed = <-tasksFailed:
	case <-shutdownCtx.Done():
		logger.Error().Msg("shutdown_timed_out")
		return ErrShutdownTimeout
	}

	if !runFinished {
		select {
		case runErr = <-runDone:
		case <-shutdownCtx.Done():
			logger.Error().Msg("shutdown_timed_out")
			return ErrShutdownTimeout
		}
	}

	if failed {
		return ErrShutdown
	}

	// context.Canceled after a shutdown request is the expected way for run to stop.
	if runErr != nil && (ctx.Err() == nil || !errors.Is(runErr, context.Canceled)) {
		logger.Error().Msg("application_runtime_failed")
		return ErrRuntime
	}

	logger.Info().Msg("shutdown_completed")

	return nil
}

func runShutdownTasks(ctx context.Context, tasks []ShutdownTask) bool {
	logger := zerolog.Ctx(ctx)
	failed := false
	for _, task := range slices.Backward(tasks) {
		logger.Debug().Str("component", task.Name).Msg("component_shutdown_started")
		if err := task.Run(ctx); err != nil {
			logger.Error().Str("component", task.Name).Msg("component_shutdown_failed")
			failed = true

			continue
		}

		logger.Debug().Str("component", task.Name).Msg("component_shutdown_completed")
	}

	return failed
}
