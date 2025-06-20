package internal

import (
	"context"
)

type Analyzer struct {
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) AnalyzePodSchedulability(ctx context.Context) error {
	return nil
}

func (a *Analyzer) EvaluateResourceConstraints(ctx context.Context) error {
	return nil
}
