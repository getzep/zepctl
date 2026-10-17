package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	zep "github.com/getzep/zep-go/v4"
)

func TestWaitForTaskWith(t *testing.T) {
	tests := []struct {
		name      string
		tasks     []*zep.Task
		timeout   time.Duration
		wantErr   string
		wantCalls int
	}{
		{
			name: "nil status then succeeded",
			tasks: []*zep.Task{
				{},
				{Status: taskStatus(zep.TaskStatusSucceeded)},
			},
			timeout:   time.Second * 3,
			wantCalls: 2,
		},
		{
			name:      "failed",
			tasks:     []*zep.Task{{Status: taskStatus(zep.TaskStatusFailed), Error: &zep.ErrorBody{Message: zep.String("failed")}}},
			timeout:   time.Second * 3,
			wantErr:   "task task-uuid failed: failed",
			wantCalls: 1,
		},
		{
			name:      "partial",
			tasks:     []*zep.Task{{Status: taskStatus(zep.TaskStatusPartial)}},
			timeout:   time.Second * 3,
			wantErr:   "task task-uuid partial",
			wantCalls: 1,
		},
		{
			name:      "timeout",
			timeout:   time.Second,
			wantErr:   "timeout waiting for task task-uuid",
			wantCalls: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(0, 0)
			calls := 0
			getTask := func(context.Context, string) (*zep.Task, error) {
				calls++
				return test.tasks[calls-1], nil
			}
			sleep := func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				return nil
			}

			err := waitForTaskWith("task-uuid", test.timeout, time.Second, getTask, func() time.Time {
				return now
			}, sleep)

			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("waitForTaskWith() error = %v, want nil", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("waitForTaskWith() error = %v, want substring %q", err, test.wantErr)
			}
			if calls != test.wantCalls {
				t.Fatalf("getTask called %d times, want %d", calls, test.wantCalls)
			}
		})
	}
}

func taskStatus(status zep.TaskStatus) *zep.TaskStatus {
	return &status
}
