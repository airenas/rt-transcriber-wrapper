package handlers

import (
	"context"
)

type InEvents struct {
}

// NewInEvents creates a in events middleware
func NewInEvents() *InEvents {
	res := InEvents{}
	return &res
}

func (sp *InEvents) Process(ctx context.Context, data string) (string, error) {
	return data, nil
}
