package steward

import (
	"context"
	"fmt"
)

type ExecutionResult struct {
	Status       JobStatus
	Decision     string
	Metadata     map[string]any
	Retryable    bool
	SideEffects  []map[string]any
	Output       map[string]any
	ErrorClass   string
	ErrorMessage string
}

type Executor interface {
	Execute(ctx context.Context, job Job) (*ExecutionResult, error)
}

type ExecutorFunc func(ctx context.Context, job Job) (*ExecutionResult, error)

func (f ExecutorFunc) Execute(ctx context.Context, job Job) (*ExecutionResult, error) {
	return f(ctx, job)
}

type Registry struct {
	executors map[string]Executor
}

func NewRegistry() *Registry {
	r := &Registry{executors: map[string]Executor{}}
	r.Register("noop", ExecutorFunc(func(ctx context.Context, job Job) (*ExecutionResult, error) {
		return &ExecutionResult{
			Status:   JobSucceeded,
			Decision: "noop",
			Metadata: map[string]any{"job_type": job.JobType},
			Output:   map[string]any{"status": "noop_success"},
		}, nil
	}))
	return r
}

func (r *Registry) Register(jobType string, ex Executor) {
	if jobType == "" || ex == nil {
		return
	}
	r.executors[jobType] = ex
}

func (r *Registry) ExecutorFor(jobType string) (Executor, error) {
	if ex, ok := r.executors[jobType]; ok {
		return ex, nil
	}
	if ex, ok := r.executors["noop"]; ok {
		return ex, nil
	}
	return nil, fmt.Errorf("no executor registered for job type %q", jobType)
}
