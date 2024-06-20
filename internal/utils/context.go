package utils

import (
	"context"
)

type key int

const (
	// CtxContext context key for custom context object
	CtxContext key = iota
)

type ProcessData struct {
	Original   string
	Punctuated string
}

type Segments struct {
	Final     bool
	ID          int
	OriginalStr string
	Processed   []*ProcessData
}

type CustomData struct {
	PartialResult string
	Segments      []*Segments
}

func CustomContext(ctx context.Context) (context.Context, *CustomData) {
	res, ok := ctx.Value(CtxContext).(*CustomData)
	if ok {
		return ctx, res
	}
	res = &CustomData{}
	return context.WithValue(ctx, CtxContext, res), res
}
