package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

var (
	ErrRuntime         = errors.New("application runtime failed")
	ErrShutdown        = errors.New("application shutdown failed")
	ErrShutdownTimeout = errors.New("application shutdown timed out")
)

type (
	RunFunc      func(context.Context) error
	ShutdownFunc func(context.Context) error
)

type ShutdownTask struct {
	Name string
	Run  ShutdownFunc
}

type triggerResult struct {
	runErr            error
	runCompleted      bool
	shutdownRequested bool
}

func Run(ctx context.Context, timeout time.Duration, runFn RunFunc, shutdownTasks ...ShutdownTask) error {
	signalContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	return run(signalContext, timeout, stop, runFn, shutdownTasks...)
}

func run(ctx context.Context, timeout time.Duration, stop context.CancelFunc, runFn RunFunc, shutdownTasks ...ShutdownTask) error {
	if stop == nil {
		stop = func() {}
	}

	runDone := startApplication(ctx, runFn)
	trigger := waitForShutdown(ctx, runDone)
	stop()

	return shutdownApplication(ctx, timeout, runDone, trigger, shutdownTasks)
}

func startApplication(ctx context.Context, runFn RunFunc) <-chan error {
	if runFn == nil {
		return nil
	}

	runDone := make(chan error, 1)
	go func() {
		runDone <- runFn(ctx)
	}()

	return runDone
}

func waitForShutdown(ctx context.Context, runDone <-chan error) triggerResult {
	if runDone == nil {
		<-ctx.Done()
		return triggerResult{shutdownRequested: true}
	}

	select {
	case <-ctx.Done():
		return triggerResult{shutdownRequested: true}
	case err := <-runDone:
		return triggerResult{runErr: err, runCompleted: true}
	}
}

func shutdownApplication(ctx context.Context, timeout time.Duration, runDone <-chan error, trigger triggerResult, shutdownTasks []ShutdownTask) error {
	logger := zerolog.Ctx(ctx)
	logger.Info().Dur("timeout", timeout).Msg("shutdown_started")
	if timeout <= 0 {
		logger.Error().Msg("shutdown_timed_out")
		return ErrShutdownTimeout
	}

	base := context.WithoutCancel(ctx)
	shutdownContext, cancel := context.WithTimeout(base, timeout)
	defer cancel()

	shutdownFailed, err := runShutdownTasks(shutdownContext, shutdownTasks)
	if err != nil {
		return err
	}

	runErr := trigger.runErr
	if shouldWaitForApplication(runDone, trigger) {
		var waitErr error
		runErr, waitErr = waitForApplication(shutdownContext, runDone)
		if waitErr != nil {
			return waitErr
		}
	}

	if shutdownFailed {
		return ErrShutdown
	}

	if applicationRunFailed(runDone, trigger, runErr) {
		logger.Error().Msg("application_runtime_failed")
		return ErrRuntime
	}

	logger.Info().Msg("shutdown_completed")
	return nil
}

func shouldWaitForApplication(runDone <-chan error, trigger triggerResult) bool {
	return runDone != nil && !trigger.runCompleted
}

func applicationRunFailed(runDone <-chan error, trigger triggerResult, runErr error) bool {
	if runDone == nil || runErr == nil {
		return false
	}

	return !trigger.shutdownRequested || !errors.Is(runErr, context.Canceled)
}

func runShutdownTasks(ctx context.Context, shutdownTasks []ShutdownTask) (bool, error) {
	failed := false
	for _, task := range slices.Backward(shutdownTasks) {
		if task.Run == nil {
			continue
		}

		if err := runShutdownTask(ctx, task); err != nil {
			if errors.Is(err, ErrShutdownTimeout) {
				return failed, err
			}
			failed = true
		}
	}

	return failed, nil
}

func runShutdownTask(ctx context.Context, task ShutdownTask) error {
	name := strings.TrimSpace(task.Name)
	if name == "" {
		name = "unnamed"
	}

	logger := zerolog.Ctx(ctx)
	logger.Debug().Str("component", name).Msg("component_shutdown_started")

	done := make(chan error, 1)
	go func() {
		done <- task.Run(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Error().Str("component", name).Msg("component_shutdown_failed")
			return ErrShutdown
		}

		logger.Debug().Str("component", name).Msg("component_shutdown_completed")
		return nil

	case <-ctx.Done():
		logger.Error().Str("component", name).Msg("shutdown_timed_out")
		return ErrShutdownTimeout
	}
}

func waitForApplication(ctx context.Context, runDone <-chan error) (error, error) {
	select {
	case err := <-runDone:
		return err, nil

	case <-ctx.Done():
		zerolog.Ctx(ctx).Error().Msg("shutdown_timed_out")
		return nil, ErrShutdownTimeout
	}
}
