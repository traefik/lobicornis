package repository

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v41/github"
	"github.com/rs/zerolog/log"
)

// Mjolnir the hammer of Thor.
type Mjolnir struct {
	client *github.Client

	globalFixesIssueRE *regexp.Regexp
	fixesIssueRE       *regexp.Regexp
	cleanNumberRE      *regexp.Regexp

	dryRun bool

	owner string
	name  string
}

func newMjolnir(client *github.Client, owner, name string, dryRun bool) Mjolnir {
	return Mjolnir{
		client: client,

		globalFixesIssueRE: regexp.MustCompile(`(?i)(?:close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)((?:[\s]+#[\d]+)(?:[\s,]+#[\d]+)*(?:[\n\r\s,]|$))`),
		fixesIssueRE:       regexp.MustCompile(`[\s,]+#`),
		cleanNumberRE:      regexp.MustCompile(`[\n\r\s,]`),

		dryRun: dryRun,

		owner: owner,
		name:  name,
	}
}

// CloseRelatedIssues Closes issues listed in the PR description.
func (m Mjolnir) CloseRelatedIssues(ctx context.Context, pr *github.PullRequest) error {
	logger := log.Ctx(ctx)

	issueNumbers := m.parseIssueFixes(ctx, pr.GetBody())

	for _, issueNumber := range issueNumbers {
		logger.Info().Msgf("closes issue #%d, add milestones %s", issueNumber, pr.Milestone.GetTitle())

		if !m.dryRun {
			err := m.closeIssue(ctx, pr, issueNumber)
			if err != nil {
				return fmt.Errorf("unable to close issue #%d: %w", issueNumber, err)
			}
		}

		// Add comment if needed

		if pr.Base.GetRef() == mainBranch {
			return nil
		}

		message := fmt.Sprintf("Closed by #%d.", pr.GetNumber())

		logger.Debug().Msgf("issue #%d, add comment: %s", issueNumber, message)

		if !m.dryRun {
			err := m.addComment(ctx, issueNumber, message)
			if err != nil {
				return fmt.Errorf("unable to add comment on issue #%d: %w", issueNumber, err)
			}
		}
	}

	return nil
}

func (m Mjolnir) closeIssue(ctx context.Context, pr *github.PullRequest, issueNumber int) error {
	var milestone *int
	if pr.Milestone != nil {
		milestone = pr.Milestone.Number
	}

	issueRequest := &github.IssueRequest{
		Milestone: milestone,
		State:     github.String("closed"),
	}

	_, _, err := m.client.Issues.Edit(ctx, m.owner, m.name, issueNumber, issueRequest)
	return err
}

func (m Mjolnir) addComment(ctx context.Context, issueNumber int, message string) error {
	issueComment := &github.IssueComment{
		Body: github.String(message),
	}

	_, _, err := m.client.Issues.CreateComment(ctx, m.owner, m.name, issueNumber, issueComment)
	return err
}

func (m Mjolnir) parseIssueFixes(ctx context.Context, text string) []int {
	submatch := m.globalFixesIssueRE.FindStringSubmatch(strings.ReplaceAll(text, ":", ""))

	if len(submatch) == 0 {
		return nil
	}

	issuesRaw := m.fixesIssueRE.Split(submatch[1], -1)

	var issueNumbers []int
	for _, issueRaw := range issuesRaw {
		cleanIssueRaw := m.cleanNumberRE.ReplaceAllString(issueRaw, "")
		if len(cleanIssueRaw) != 0 {
			numb, err := strconv.ParseInt(cleanIssueRaw, 10, 16)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Str("cleanIssueRaw", cleanIssueRaw).Msg("unable to parse int")
			}

			issueNumbers = append(issueNumbers, int(numb))
		}
	}
	return issueNumbers
}
