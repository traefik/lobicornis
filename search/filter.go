package search

import "fmt"

// Parameter search parameter
type Parameter func() string

// NoOp No operation parameter
func NoOp() string {
	return ""
}

// Cond apply conditionally some parameters
func Cond(apply bool, parameters ...Parameter) Parameter {
	if apply {
		return func() string {
			var filter string
			for _, param := range parameters {
				if param != nil {
					filter += param()
				}
			}
			return filter
		}
	}
	return NoOp
}

// WithReviewApproved add a search filter by review approved.
func WithReviewApproved() string {
	return " review:approved "
}

// WithLabels add a search filter by labels.
func WithLabels(labels ...string) Parameter {
	var labelsFilter string
	for _, lbl := range labels {
		labelsFilter += fmt.Sprintf("label:%s ", lbl)
	}
	return func() string {
		return " " + labelsFilter
	}
}

// WithExcludedLabels add a search filter by unwanted labels.
func WithExcludedLabels(labels ...string) Parameter {
	var labelsFilter string
	for _, lbl := range labels {
		labelsFilter += fmt.Sprintf("-label:%s ", lbl)
	}
	return func() string {
		return " " + labelsFilter
	}
}
