package repository

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v32/github"
)

// removeLabels remove some labels on an issue (PR).
func (r *Repository) removeLabels(ctx context.Context, pr numbered, labelsToRemove []string) error {
	freshIssue, _, err := r.client.Issues.Get(ctx, r.owner, r.name, pr.GetNumber())
	if err != nil {
		return err
	}

	var newLabels []string
	for _, lbl := range freshIssue.Labels {
		if !contains(labelsToRemove, lbl.GetName()) {
			newLabels = append(newLabels, lbl.GetName())
		}
	}

	if len(freshIssue.Labels) == len(newLabels) {
		return nil
	}

	if newLabels == nil {
		// Due to go-github/GitHub API constraint
		newLabels = []string{}
	}

	_, _, err = r.client.Issues.ReplaceLabelsForIssue(ctx, r.owner, r.name, pr.GetNumber(), newLabels)
	return err
}

// removeLabel remove a label on an issue (PR).
func (r Repository) removeLabel(ctx context.Context, pr *github.PullRequest, label string) error {
	if !hasLabel(pr, label) {
		return nil
	}

	if r.dryRun || r.debug {
		log.Printf("Remove label: %s. Dry run: %v", label, r.dryRun)
	}

	if r.dryRun {
		return nil
	}

	resp, err := r.client.Issues.RemoveLabelForIssue(ctx, r.owner, r.name, pr.GetNumber(), label)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove label %s. Status code: %d", label, resp.StatusCode)
	}

	return nil
}

// addLabels add some labels on an issue (PR).
func (r *Repository) addLabels(ctx context.Context, pr numbered, labels ...string) error {
	if r.dryRun || r.debug {
		log.Printf("Add labels: %s. Dry run: %v", labels, r.dryRun)
	}

	if r.dryRun {
		return nil
	}

	_, resp, err := r.client.Issues.AddLabelsToIssue(ctx, r.owner, r.name, pr.GetNumber(), labels)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add labels %v. Status code: %d", labels, resp.StatusCode)
	}

	return nil
}

// hasLabel checks if an issue has a specific label.
func hasLabel(pr *github.PullRequest, label string) bool {
	for _, lbl := range pr.Labels {
		if lbl.GetName() == label {
			return true
		}
	}

	return false
}

// findLabelNameWithPrefix Find an issue with a specific label prefix.
func findLabelNameWithPrefix(labels []*github.Label, prefix string) string {
	for _, lbl := range labels {
		if strings.HasPrefix(lbl.GetName(), prefix) {
			return lbl.GetName()
		}
	}

	return ""
}

func contains(values []string, value string) bool {
	for _, val := range values {
		if value == val {
			return true
		}
	}
	return false
}
