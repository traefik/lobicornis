package repository

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
)

func (r Repository) cleanRetryLabel(ctx context.Context, pr *github.PullRequest, retry bool) {
	if !retry {
		return
	}

	currentRetryLabel := findLabelNameWithPrefix(pr.Labels, r.markers.MergeRetryPrefix)
	if len(currentRetryLabel) > 0 {
		err := r.removeLabel(ctx, pr, currentRetryLabel)
		if err != nil {
			log.Println(err)
		}
	}
}

func (r Repository) manageRetryLabel(ctx context.Context, pr *github.PullRequest, retry bool) {
	if !retry || r.retry.Number <= 0 {
		// Need Human
		errLbl := r.addLabels(ctx, pr, r.markers.NeedHumanMerge)
		if errLbl != nil {
			log.Println(errLbl)
		}

		errLbl = r.removeLabel(ctx, pr, r.markers.MergeInProgress)
		if errLbl != nil {
			log.Println(errLbl)
		}

		return
	}

	currentRetryLabel := findLabelNameWithPrefix(pr.Labels, r.markers.MergeRetryPrefix)
	if len(currentRetryLabel) == 0 {
		// first retry
		newRetryLabel := r.markers.MergeRetryPrefix + strconv.Itoa(1)

		errLbl := r.addLabels(ctx, pr, newRetryLabel)
		if errLbl != nil {
			log.Println(errLbl)
		}

		errLbl = r.addLabels(ctx, pr, r.markers.MergeInProgress)
		if errLbl != nil {
			log.Println(errLbl)
		}

		return
	}

	err := r.removeLabel(ctx, pr, currentRetryLabel)
	if err != nil {
		log.Println(err)
	}

	number := extractRetryNumber(currentRetryLabel, r.markers.MergeRetryPrefix)

	if number >= r.retry.Number {
		r.callHuman(ctx, pr, fmt.Sprintf("too many retry: %d/%d", number, r.retry.Number))

		return
	}

	// retry
	newRetryLabel := r.markers.MergeRetryPrefix + strconv.Itoa(number+1)
	errLabel := r.addLabels(ctx, pr, newRetryLabel)
	if errLabel != nil {
		log.Println(errLabel)
	}
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
