package types

import (
	"net/url"

	"github.com/containous/flaeg"
)

// Remote name.
const (
	RemoteOrigin   = "origin"
	RemoteUpstream = "upstream"
)

// Merge action.
const (
	ActionMerge  = "merge"
	ActionRebase = "rebase"
)

// NoOption empty struct.
type NoOption struct{}

// Configuration task configuration.
type Configuration struct {
	Owner              string        `short:"o" description:"Repository owner. [required]"`
	RepositoryName     string        `long:"repo-name" short:"r" description:"Repository name. [required]"`
	GitHubToken        string        `long:"token" short:"t" description:"GitHub Token. [required]"`
	MinReview          int           `long:"min-review" description:"Minimal number of review."`
	MinLightReview     int           `long:"min-light-review" description:"Minimal number of review (light review)."`
	DryRun             bool          `long:"dry-run" description:"Dry run mode."`
	Debug              bool          `long:"debug" description:"Debug mode."`
	SSH                bool          `description:"Use SSH instead HTTPS."`
	DefaultMergeMethod string        `long:"merge-method" description:"Default merge method. (merge|squash|rebase|ff)"`
	LabelMarkers       *LabelMarkers `long:"marker" description:"GitHub Labels."`
	CheckNeedUpToDate  bool          `long:"check-up-to-date" description:"Use GitHub repository configuration to check the need to be up-to-date."`
	ForceNeedUpToDate  bool          `long:"force-up-to-date" description:"Forcing need up-to-date. (check-up-to-date must be false)"`
	Retry              *Retry        `long:"retry" description:"Merge retry configuration."`
	ServerMode         bool          `long:"server" description:"Server mode."`
	ServerPort         int           `long:"port" description:"Server port."`
	NeedMilestone      bool          `long:"need-milestone" description:"Forcing PR to have a milestone."`
	GitUserEmail       string        `long:"git-email" description:"Git user email."`
	GitUserName        string        `long:"git-name" description:"Git user name."`
	APIURL             string        `long:"github-url" description:"GitHub API URL (GitHub Enterprise) [optional]"`
	GitHubURL          *url.URL      // related to "github-url"
}

// LabelMarkers Labels use to control actions.
type LabelMarkers struct {
	NeedHumanMerge    string `long:"need-human-merge" description:"Label use when the bot cannot perform a merge."`
	NeedMerge         string `long:"need-merge" description:"Label use when you want the bot perform a merge."`
	MergeInProgress   string `long:"merge-in-progress" description:"Label use when the bot update the PR (merge/rebase)."`
	MergeRetryPrefix  string `long:"merge-retry-prefix" description:"Use to manage merge retry."`
	MergeMethodPrefix string `long:"merge-method-prefix" description:"Use to override default merge method for a PR."`
	LightReview       string `long:"light-review" description:"Label use when a pull request need a lower minimal review as default."`
	NoMerge           string `long:"no-merge" description:"Label use when a PR must not be merge."`
}

// Retry configuration.
type Retry struct {
	Number      int            `long:"number" description:"Number of retry before failed."`
	Interval    flaeg.Duration `long:"interval" description:"Time between retry."`
	OnStatuses  bool           `long:"on-statuses" description:"Retry on GitHub checks (aka statuses)."`
	OnMergeable bool           `long:"on-mergeable" description:"Retry on PR mergeable state (GitHub information)."`
}

// GitConfig Git local configuration.
type GitConfig struct {
	GitHubToken string
	SSH         bool
	UserName    string
	Email       string
}

// Review Review criterion.
type Review struct {
	Min      int
	MinLight int
}

// Checks Pull request checks.
type Checks struct {
	CheckNeedUpToDate bool
	ForceNeedUpToDate bool
	NeedMilestone     bool
	Review            Review
}

// RepoID Repository identifier.
type RepoID struct {
	Owner          string
	RepositoryName string
}

// Extra configuration.
type Extra struct {
	DryRun bool
	Debug  bool
}

// Result Merge result.
type Result struct {
	Message string
	Merged  bool
}
