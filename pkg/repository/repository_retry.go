package repository

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
)

func (r Repository) cleanRetryLabel(ctx context.Context, pr *github.PullRequest) {
	if !r.retry.OnMergeable && !r.retry.OnStatuses {
		return
	}

	currentRetryLabel := findLabelNameWithPrefix(pr.Labels, r.markers.MergeRetryPrefix)
	if len(currentRetryLabel) > 0 {
		err := r.removeLabel(ctx, pr, currentRetryLabel)
		ignoreError(err)
	}
}

func (r Repository) manageRetryLabel(ctx context.Context, pr *github.PullRequest, retry bool, rootErr error) error {
	if !retry || r.retry.Number <= 0 {
		return rootErr
	}

	currentRetryLabel := findLabelNameWithPrefix(pr.Labels, r.markers.MergeRetryPrefix)
	if len(currentRetryLabel) == 0 {
		// first retry
		newRetryLabel := r.markers.MergeRetryPrefix + strconv.Itoa(1)

		err := r.addLabels(ctx, pr, newRetryLabel)
		ignoreError(err)

		err = r.addLabels(ctx, pr, r.markers.MergeInProgress)
		ignoreError(err)

		return nil
	}

	err := r.removeLabel(ctx, pr, currentRetryLabel)
	ignoreError(err)

	number := extractRetryNumber(currentRetryLabel, r.markers.MergeRetryPrefix)

	if number >= r.retry.Number {
		return fmt.Errorf("too many retry [%d/%d]: %v", number, r.retry.Number, rootErr)
	}

	// retry
	newRetryLabel := r.markers.MergeRetryPrefix + strconv.Itoa(number+1)

	err = r.addLabels(ctx, pr, newRetryLabel)
	ignoreError(err)

	return nil
}

func extractRetryNumber(label, prefix string) int {
	raw := strings.TrimPrefix(label, prefix)

	number, err := strconv.Atoi(raw)
	if err != nil {
		log.Println(err)
		return 0
	}

	return number
}
