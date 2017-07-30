package mjolnir

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/google/go-github/github"
)

var (
	globalFixesIssueRE = regexp.MustCompile(`(?i)(?:close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)((?:[\s]+#[\d]+)(?:[\s,]+#[\d]+)*(?:[\n\r\s,]|$))`)
	fixesIssueRE       = regexp.MustCompile(`[\s,]+#`)
	cleanNumberRE      = regexp.MustCompile(`[\n\r\s,]`)
)

// CloseRelatedIssues Closes issues listed in the PR description.
func CloseRelatedIssues(ctx context.Context, client *github.Client, owner string, repositoryName string, pr *github.PullRequest) {

	issueNumbers := parseIssueFixes(pr.GetBody())

	for _, issueNumber := range issueNumbers {
		issueRequest := &github.IssueRequest{
			Milestone: pr.Milestone.Number,
			State:     github.String("closed"),
		}
		_, _, err := client.Issues.Edit(ctx, owner, repositoryName, issueNumber, issueRequest)
		if err != nil {
			log.Fatal(err)
		}

		if pr.Base.GetRef() != "master" {
			issueComment := &github.IssueComment{
				Body: github.String(fmt.Sprintf("Closed by #%d.", pr.GetNumber())),
			}
			client.Issues.CreateComment(ctx, owner, repositoryName, issueNumber, issueComment)
		}
	}
}

func parseIssueFixes(text string) []int {
	var issueNumbers []int

	submatch := globalFixesIssueRE.FindStringSubmatch(text)

	if len(submatch) != 0 {
		issuesRaw := fixesIssueRE.Split(submatch[1], -1)

		for _, issueRaw := range issuesRaw {
			cleanIssueRaw := cleanNumberRE.ReplaceAllString(issueRaw, "")
			if len(cleanIssueRaw) != 0 {
				numb, err := strconv.ParseInt(cleanIssueRaw, 10, 16)
				if err != nil {
					log.Println(err)
				}
				issueNumbers = append(issueNumbers, int(numb))
			}
		}
	}
	return issueNumbers
}
