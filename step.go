package retryflow

import (
	"context"
	"fmt"
)

// Step defines a single step in the retry sequence.
type Step struct {
	run        func(ctx context.Context, input any) (any, error) // Execution function that takes context, input and returns output and error
	outputPtr  any                                               // Pointer to store the output (*T)
	checkpoint bool
	onFail     func()
}

// Exec creates a step that executes a function without input/output.
func Exec(fn func(context.Context) error) *Step {
	return &Step{
		run: func(ctx context.Context, _ any) (any, error) {
			return nil, fn(ctx)
		},
	}
}

func Chain[In any, Out any](fn func(context.Context, In) (Out, error)) *Step {
	s := &Step{}
	s.run = func(ctx context.Context, input any) (any, error) {
		in, ok := input.(In)
		if !ok && input != nil {
			return nil, fmt.Errorf("expected %T but got %T", *new(In), input)
		}
		return fn(ctx, in)
	}
	return s
}

func (s *Step) Do(ptr any) *Step {
	s.outputPtr = ptr
	return s
}

// Checkpoint marks the step as a checkpoint.
func (s *Step) Checkpoint() *Step {
	s.checkpoint = true
	return s
}

// OnFail sets the failure handler for the step.
func (s *Step) OnFail(fn func()) *Step {
	s.onFail = fn
	return s
}

// Steps is a sequence of steps.
type Steps []*Step

// Seq creates a sequence of steps.
func Seq(steps ...*Step) Steps {
	return steps
}
