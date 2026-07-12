package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

const (
	OutputLimit = 8 * 1024 * 1024
	waitDelay   = time.Second
)

var (
	ErrCanceled            = errors.New("external process canceled")
	ErrOutputLimitExceeded = errors.New("external process output limit exceeded")
	ErrTimedOut            = errors.New("external process timed out")
	errConfiguredTimeout   = errors.New("configured process timeout elapsed")
)

type OutputLimitError struct {
	Limit  int
	Stream string
}

func (e *OutputLimitError) Error() string {
	return fmt.Sprintf("%s exceeded %d-byte output limit", e.Stream, e.Limit)
}

func (e *OutputLimitError) Is(target error) bool {
	return target == ErrOutputLimitExceeded
}

type RunError struct {
	Cause        error
	ContextCause error
	Detail       error
	Kind         error
}

func (e *RunError) Error() string {
	if e.Detail != nil {
		return e.Detail.Error()
	}
	return e.Kind.Error()
}

func (e *RunError) Unwrap() []error {
	errs := []error{e.Kind}
	if e.ContextCause != nil {
		errs = append(errs, e.ContextCause)
	}
	if e.Detail != nil {
		errs = append(errs, e.Detail)
	}
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	return errs
}

type Result struct {
	Stderr []byte
	Stdout []byte
}

type Spec struct {
	Args []string
	Dir  string
	Env  []string
	Name string
}

type limitedBuffer struct {
	data       []byte
	limit      int
	onOverflow func()
	once       sync.Once
}

func (b *limitedBuffer) Bytes() []byte {
	return b.data
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - len(b.data)
	retain := min(remaining, len(p))
	if retain > 0 {
		needed := len(b.data) + retain
		if cap(b.data) < needed {
			newCap := max(needed, cap(b.data)*2)
			newCap = min(newCap, b.limit)
			data := make([]byte, len(b.data), newCap)
			copy(data, b.data)
			b.data = data
		}
		b.data = append(b.data, p[:retain]...)
	}
	if retain < len(p) {
		b.once.Do(b.onOverflow)
	}
	return len(p), nil
}

func Run(parent context.Context, spec Spec, timeout time.Duration) (Result, error) {
	if parent == nil {
		parent = context.Background()
	}

	deadlineCtx := parent
	deadlineCancel := func() {}
	if timeout > 0 {
		deadlineCtx, deadlineCancel = context.WithTimeoutCause(parent, timeout, errConfiguredTimeout)
	}
	defer deadlineCancel()

	runCtx, cancel := context.WithCancelCause(deadlineCtx)
	defer cancel(nil)

	setOverflow := func(stream string) func() {
		return func() {
			cancel(&OutputLimitError{Limit: OutputLimit, Stream: stream})
		}
	}

	stdout := &limitedBuffer{limit: OutputLimit, onOverflow: setOverflow("stdout")}
	stderr := &limitedBuffer{limit: OutputLimit, onOverflow: setOverflow("stderr")}

	cmd := exec.CommandContext(runCtx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = waitDelay
	configureProcessGroup(cmd)
	cmd.Cancel = func() error {
		return terminateProcessGroup(cmd)
	}

	err := cmd.Run()
	result := Result{
		Stderr: stderr.Bytes(),
		Stdout: stdout.Bytes(),
	}

	var limitErr *OutputLimitError
	if errors.As(context.Cause(runCtx), &limitErr) {
		_ = terminateProcessGroup(cmd)
		return result, &RunError{
			Cause:  err,
			Detail: limitErr,
			Kind:   ErrOutputLimitExceeded,
		}
	}
	if err == nil {
		return result, nil
	}
	if errors.Is(context.Cause(deadlineCtx), errConfiguredTimeout) {
		return result, &RunError{
			Cause:        err,
			ContextCause: context.DeadlineExceeded,
			Kind:         ErrTimedOut,
		}
	}
	if parent.Err() != nil {
		return result, &RunError{
			Cause:        err,
			ContextCause: context.Cause(parent),
			Kind:         ErrCanceled,
		}
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		_ = terminateProcessGroup(cmd)
	}
	return result, err
}
