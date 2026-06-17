package tools

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/callbacks"
	"go.starlark.net/lib/math"
	"go.starlark.net/starlark"
)

// Calculator is a tool that can do math.
type Calculator struct {
	CallbacksHandler callbacks.Handler
}

var _ Tool = Calculator{}

// Description returns a string describing the calculator tool.
func (c Calculator) Description() string {
	return `Useful for when you need to answer questions about math.
Input should be a plain-text mathematical expression (not JSON), e.g. 2 + 2 or sqrt(16) * 3.
Supports arithmetic, trigonometry, and other common math functions.
Returns the result of evaluating the expression as a number string.`
}

// Name returns the name of the tool.
func (c Calculator) Name() string {
	return "calculator"
}

// Call evaluates the input using a starlak evaluator and returns the result as a
// string. If the evaluator errors the error is given in the result to give the
// agent the ability to retry.
func (c Calculator) Call(ctx context.Context, input string) (string, error) {
	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolStart(ctx, input)
	}

	v, err := starlark.Eval(&starlark.Thread{Name: "main"}, "input", input, math.Module.Members)
	if err != nil {
		return fmt.Sprintf("error from evaluator: %s", err.Error()), nil //nolint:nilerr
	}
	result := v.String()

	if c.CallbacksHandler != nil {
		c.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	return result, nil
}
