package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestRunWithoutWork(t *testing.T) {
	ctx, cancel, output := newTestContext(t)
	cancel()

	if err := run(ctx, time.Second, func() {}, nil); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(output.String(), "shutdown_started") || !strings.Contains(output.String(), "shutdown_completed") {
		t.Errorf("lifecycle output = %q", output.String())
	}
}

func TestShutdownTasksRunInReverseOrder(t *testing.T) {
	ctx, cancel, _ := newTestContext(t)
	order := make([]string, 0, 3)
	task := func(name string) ShutdownTask {
		return ShutdownTask{Name: name, Run: func(context.Context) error {
			order = append(order, name)
			return nil
		}}
	}
	cancel()

	err := run(ctx, time.Second, func() {}, nil, task("first"), task("second"), task("third"))
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if want := []string{"third", "second", "first"}; !reflect.DeepEqual(order, want) {
		t.Errorf("hook order = %v, want %v", order, want)
	}
}

func TestRunErrorsAreSafe(t *testing.T) {
	tests := []struct {
		name    string
		runFn   RunFunc
		tasks   []ShutdownTask
		wantErr error
	}{
		{
			name:    "runtime failure",
			runFn:   func(context.Context) error { return errors.New("runtime-secret-do-not-leak") },
			wantErr: ErrRuntime,
		},
		{
			name: "shutdown failure",
			tasks: []ShutdownTask{{Name: "database", Run: func(context.Context) error {
				return errors.New("database-secret-do-not-leak")
			}}},
			wantErr: ErrShutdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, output := newTestContext(t)
			if tt.runFn == nil {
				cancel()
			}

			err := run(ctx, time.Second, func() {}, tt.runFn, tt.tasks...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("run() error = %v, want %v", err, tt.wantErr)
			}
			if strings.Contains(output.String(), "do-not-leak") || strings.Contains(err.Error(), "do-not-leak") {
				t.Fatalf("secret leaked: error=%v output=%q", err, output.String())
			}
		})
	}
}

func TestRunUsesSingleShutdownTimeout(t *testing.T) {
	ctx, cancel, output := newTestContext(t)
	block := make(chan struct{})
	cancel()

	started := time.Now()
	err := run(ctx, 20*time.Millisecond, func() {}, nil, ShutdownTask{
		Name: "blocked",
		Run: func(context.Context) error {
			<-block
			return nil
		},
	})
	if !errors.Is(err, ErrShutdownTimeout) {
		t.Fatalf("run() error = %v, want ErrShutdownTimeout", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Errorf("shutdown took %s, want bounded timeout", elapsed)
	}
	if !strings.Contains(output.String(), "shutdown_timed_out") {
		t.Errorf("timeout output = %q", output.String())
	}
	close(block)
}

func TestRunTimesOutWaitingForRunFunction(t *testing.T) {
	ctx, cancel, _ := newTestContext(t)
	runStarted := make(chan struct{})
	block := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, 20*time.Millisecond, func() {}, func(context.Context) error {
			close(runStarted)
			<-block
			return nil
		})
	}()

	<-runStarted
	cancel()
	if err := <-done; !errors.Is(err, ErrShutdownTimeout) {
		t.Fatalf("run() error = %v, want ErrShutdownTimeout", err)
	}
	close(block)
}

func TestCanceledRunFunctionCompletesNormally(t *testing.T) {
	ctx, cancel, _ := newTestContext(t)
	runStarted := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, time.Second, func() {}, func(ctx context.Context) error {
			close(runStarted)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	<-runStarted
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func newTestContext(t *testing.T) (context.Context, context.CancelFunc, *bytes.Buffer) {
	t.Helper()
	output := &bytes.Buffer{}
	logger := zerolog.New(output).With().Timestamp().Str("service", "test").Logger()
	ctx, cancel := context.WithCancel(logger.WithContext(context.Background()))
	return ctx, cancel, output
}
