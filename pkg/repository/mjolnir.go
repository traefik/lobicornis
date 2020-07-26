package repository

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
)

// Mjolnir the hammer of Thor.
type Mjolnir struct {
	client *github.Client

	globalFixesIssueRE *regexp.Regexp
	fixesIssueRE       *regexp.Regexp
	cleanNumberRE      *regexp.Regexp

	debug  bool
	dryRun bool

	owner string
	name  string
}

func newMjolnir(client *github.Client, owner, name string, debug, dryRun bool) Mjolnir {
	return Mjolnir{
		client: client,

		globalFixesIssueRE: regexp.MustCompile(`(?i)(?:close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)((?:[\s]+#[\d]+)(?:[\s,]+#[\d]+)*(?:[\n\r\s,]|$))`),
		fixesIssueRE:       regexp.MustCompile(`[\s,]+#`),
		cleanNumberRE:      regexp.MustCompile(`[\n\r\s,]`),

		debug:  debug,
		dryRun: dryRun,

		owner: owner,
		name:  name,
	}
}

// CloseRelatedIssues Closes issues listed in the PR description.
func (m Mjolnir) CloseRelatedIssues(ctx context.Context, pr *github.PullRequest) error {
	issueNumbers := m.parseIssueFixes(pr.GetBody())

	for _, issueNumber := range issueNumbers {
		log.Printf("closes issue #%d, add milestones %s", issueNumber, pr.Milestone.GetTitle())

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

		log.Printf("issue #%d, add comment: %s", issueNumber, message)

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

func (m Mjolnir) parseIssueFixes(text string) []int {
	submatch := m.globalFixesIssueRE.FindStringSubmatch(strings.Replace(text, ":", "", -1))

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
				log.Println(err)
			}

			issueNumbers = append(issueNumbers, int(numb))
		}
	}
	return issueNumbers
}
