package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestRun(t *testing.T) {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })

	tests := []struct {
		name string

		giveTimeout time.Duration
		giveRun     func(context.Context) error
		giveTasks   []ShutdownTask

		wantErr    error
		wantOutput string
	}{
		{
			name:        "no run and no tasks, context canceled -> clean shutdown",
			giveTimeout: time.Second,
			wantOutput:  "shutdown_completed",
		},
		{
			name:        "run exits with context.Canceled after shutdown request -> not an error",
			giveTimeout: time.Second,
			giveRun: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			wantOutput: "shutdown_completed",
		},
		{
			name:        "run fails with a secret in its error -> ErrRuntime without the secret",
			giveTimeout: time.Second,
			giveRun:     func(context.Context) error { return errors.New("runtime-secret-do-not-leak") },
			wantErr:     ErrRuntime,
			wantOutput:  "application_runtime_failed",
		},
		{
			name:        "task fails with a secret in its error -> ErrShutdown without the secret",
			giveTimeout: time.Second,
			giveTasks: []ShutdownTask{{Name: "database", Run: func(context.Context) error {
				return errors.New("database-secret-do-not-leak")
			}}},
			wantErr:    ErrShutdown,
			wantOutput: "component_shutdown_failed",
		},
		{
			name:        "task blocks past the deadline -> ErrShutdownTimeout",
			giveTimeout: 20 * time.Millisecond,
			giveTasks: []ShutdownTask{{Name: "blocked", Run: func(context.Context) error {
				<-block
				return nil
			}}},
			wantErr:    ErrShutdownTimeout,
			wantOutput: "shutdown_timed_out",
		},
		{
			name:        "run ignores cancellation past the deadline -> ErrShutdownTimeout",
			giveTimeout: 20 * time.Millisecond,
			giveRun: func(context.Context) error {
				<-block
				return nil
			},
			wantErr:    ErrShutdownTimeout,
			wantOutput: "shutdown_timed_out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, output := newTestContext(t)
			cancel()

			started := time.Now()
			err := Run(ctx, tt.giveTimeout, tt.giveRun, tt.giveTasks...)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
			}
			if elapsed := time.Since(started); elapsed > tt.giveTimeout+250*time.Millisecond {
				t.Errorf("Run() took %s, want it bounded by the %s timeout", elapsed, tt.giveTimeout)
			}
			if !strings.Contains(output.String(), tt.wantOutput) {
				t.Errorf("Run() output = %q, want %q", output.String(), tt.wantOutput)
			}
			if strings.Contains(output.String(), "do-not-leak") || (err != nil && strings.Contains(err.Error(), "do-not-leak")) {
				t.Errorf("secret leaked: error=%v output=%q", err, output.String())
			}
		})
	}
}

func TestRunTasksInReverseOrder(t *testing.T) {
	ctx, cancel, _ := newTestContext(t)
	cancel()

	order := make([]string, 0, 3)
	task := func(name string) ShutdownTask {
		return ShutdownTask{Name: name, Run: func(context.Context) error {
			order = append(order, name)
			return nil
		}}
	}

	if err := Run(ctx, time.Second, nil, task("first"), task("second"), task("third")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if want := []string{"third", "second", "first"}; !reflect.DeepEqual(order, want) {
		t.Errorf("task order = %v, want %v", order, want)
	}
}

func TestRunStopsWhenRunReturns(t *testing.T) {
	ctx, cancel, output := newTestContext(t)
	defer cancel()

	if err := Run(ctx, time.Second, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), "shutdown_completed") {
		t.Errorf("Run() output = %q", output.String())
	}
}

func TestNotifyContextCanceledBySignal(t *testing.T) {
	ctx := NotifyContext(context.Background())

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("NotifyContext() was not canceled by SIGTERM")
	}
}

func newTestContext(t *testing.T) (context.Context, context.CancelFunc, *bytes.Buffer) {
	t.Helper()

	output := &bytes.Buffer{}
	// Shutdown tasks log from their own goroutine, and bytes.Buffer is not safe for concurrent writes.
	logger := zerolog.New(zerolog.SyncWriter(output)).With().Timestamp().Str("service", "test").Logger()
	ctx, cancel := context.WithCancel(logger.WithContext(context.Background()))

	return ctx, cancel, output
}
