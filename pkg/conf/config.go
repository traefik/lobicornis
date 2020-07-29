package conf

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Configuration the global configuration.
type Configuration struct {
	Github       Github                 `yaml:"github"`
	Git          Git                    `yaml:"git"`
	Server       Server                 `yaml:"server"`
	Markers      Markers                `yaml:"markers"`
	Retry        Retry                  `yaml:"retry"`
	Default      RepoConfig             `yaml:"default"`
	Extra        Extra                  `yaml:"extra"`
	Repositories map[string]*RepoConfig `yaml:"repositories,omitempty"`
}

// Github the GitHub configuration.
type Github struct {
	User  string `yaml:"user,omitempty"`
	Token string `yaml:"token,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

// Git the Git configuration.
type Git struct {
	Email    string `yaml:"email,omitempty"`
	UserName string `yaml:"userName,omitempty"`
	SSH      bool   `yaml:"ssh,omitempty"`
}

// Server the server configuration.
type Server struct {
	Port int `yaml:"port"`
}

// Markers the markers configuration.
type Markers struct {
	LightReview       string `yaml:"lightReview,omitempty"`
	NeedMerge         string `yaml:"needMerge,omitempty"`
	MergeInProgress   string `yaml:"mergeInProgress,omitempty"`
	MergeMethodPrefix string `yaml:"mergeMethodPrefix,omitempty"`
	MergeRetryPrefix  string `yaml:"mergeRetryPrefix,omitempty"`
	NeedHumanMerge    string `yaml:"needHumanMerge,omitempty"`
	NoMerge           string `yaml:"noMerge,omitempty"`
}

// Retry the retry configuration.
type Retry struct {
	Interval    time.Duration `yaml:"interval,omitempty"`
	Number      int           `yaml:"number,omitempty"`
	OnMergeable bool          `yaml:"onMergeable,omitempty"`
	OnStatuses  bool          `yaml:"onStatuses,omitempty"`
}

// RepoConfig the repo configuration.
type RepoConfig struct {
	MergeMethod       *string `yaml:"mergeMethod,omitempty"`
	MinLightReview    *int    `yaml:"minLightReview,omitempty"`
	MinReview         *int    `yaml:"minReview,omitempty"`
	NeedMilestone     *bool   `yaml:"needMilestone,omitempty"`
	CheckNeedUpToDate *bool   `yaml:"checkNeedUpToDate,omitempty"`
	ForceNeedUpToDate *bool   `yaml:"forceNeedUpToDate,omitempty"`
	AddErrorInComment *bool   `yaml:"addErrorInComment,omitempty"`
}

// GetMergeMethod gets merge method.
func (r *RepoConfig) GetMergeMethod() string {
	if r.MergeMethod != nil {
		return *r.MergeMethod
	}

	return ""
}

// GetMinLightReview gets MinLightReview.
func (r *RepoConfig) GetMinLightReview() int {
	if r.MinLightReview != nil {
		return *r.MinLightReview
	}

	return -1
}

// GetMinReview gets MinReview.
func (r *RepoConfig) GetMinReview() int {
	if r.MinReview != nil {
		return *r.MinReview
	}

	return -1
}

// GetNeedMilestone gets NeedMilestone.
func (r *RepoConfig) GetNeedMilestone() bool {
	if r.NeedMilestone != nil {
		return *r.NeedMilestone
	}

	return false
}

// GetCheckNeedUpToDate gets CheckNeedUpToDate.
func (r *RepoConfig) GetCheckNeedUpToDate() bool {
	if r.CheckNeedUpToDate != nil {
		return *r.CheckNeedUpToDate
	}

	return false
}

// GetForceNeedUpToDate gets ForceNeedUpToDate.
func (r *RepoConfig) GetForceNeedUpToDate() bool {
	if r.ForceNeedUpToDate != nil {
		return *r.ForceNeedUpToDate
	}

	return false
}

// GetAddErrorInComment gets AddErrorInComment.
func (r *RepoConfig) GetAddErrorInComment() bool {
	if r.AddErrorInComment != nil {
		return *r.AddErrorInComment
	}

	return false
}

// Extra the extra configuration.
type Extra struct {
	Debug  bool `yaml:"debug,omitempty"`
	DryRun bool `yaml:"dryRun,omitempty"`
}

// Load loads the configuration.
func Load(filename string) (Configuration, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Configuration{}, err
	}

	cfg := Configuration{
		Github: Github{
			Token: os.Getenv("GITHUB_TOKEN"),
		},
		Server: Server{
			Port: 80,
		},
		Markers: Markers{
			NeedHumanMerge:    "bot/need-human-merge",
			NeedMerge:         "status/3-needs-merge",
			MergeInProgress:   "status/4-merge-in-progress",
			MergeMethodPrefix: "bot/merge-method-",
			MergeRetryPrefix:  "bot/merge-retry-",
			LightReview:       "bot/light-review",
			NoMerge:           "bot/no-merge",
		},
		Retry: Retry{
			Interval: 1 * time.Minute,
		},
		Default: RepoConfig{
			MergeMethod:       String("squash"),
			MinLightReview:    Int(0),
			MinReview:         Int(1),
			NeedMilestone:     Bool(true),
			CheckNeedUpToDate: Bool(false),
			ForceNeedUpToDate: Bool(true),
			AddErrorInComment: Bool(false),
		},
		Extra: Extra{
			Debug:  false,
			DryRun: true,
		},
		Repositories: map[string]*RepoConfig{},
	}
	err = yaml.NewDecoder(file).Decode(&cfg)
	if err != nil {
		return Configuration{}, err
	}

	for _, config := range cfg.Repositories {
		if config == nil {
			continue
		}

		applyDefault(config, cfg)
	}

	err = validate(cfg)
	if err != nil {
		return Configuration{}, err
	}

	return cfg, nil
}

func applyDefault(config *RepoConfig, cfg Configuration) {
	if config.CheckNeedUpToDate == nil {
		config.CheckNeedUpToDate = cfg.Default.CheckNeedUpToDate
	}

	if config.ForceNeedUpToDate == nil {
		config.ForceNeedUpToDate = cfg.Default.ForceNeedUpToDate
	}

	if config.MergeMethod == nil {
		config.MergeMethod = cfg.Default.MergeMethod
	}

	if config.MinLightReview == nil {
		config.MinLightReview = cfg.Default.MinLightReview
	}

	if config.MinReview == nil {
		config.MinReview = cfg.Default.MinReview
	}

	if config.NeedMilestone == nil {
		config.NeedMilestone = cfg.Default.NeedMilestone
	}

	if config.AddErrorInComment == nil {
		config.AddErrorInComment = cfg.Default.AddErrorInComment
	}
}

func validate(cfg Configuration) error {
	fields := map[string]string{
		"github.user":               cfg.Github.User,
		"git.email":                 cfg.Git.Email,
		"git.userName":              cfg.Git.UserName,
		"markers.needMerge":         cfg.Markers.NeedMerge,
		"markers.mergeInProgress":   cfg.Markers.MergeInProgress,
		"markers.lightReview":       cfg.Markers.LightReview,
		"markers.mergeMethodPrefix": cfg.Markers.MergeMethodPrefix,
		"markers.needHumanMerge":    cfg.Markers.NeedHumanMerge,
		"markers.noMerge":           cfg.Markers.NoMerge,
	}

	for field, value := range fields {
		if value == "" {
			return fmt.Errorf("%s is required", field)
		}
	}

	if cfg.Default.GetMinReview() < 0 {
		return errors.New("default.minReview is invalid")
	}

	if cfg.Default.GetMinLightReview() < 0 {
		return errors.New("default.minLightReview is invalid")
	}

	if cfg.Default.GetMergeMethod() == "" {
		return errors.New("default.mergeMethod is required")
	}

	return nil
}

// String convert a string to a string pointer.
func String(v string) *string { return &v }

// Int convert a int to a int pointer.
func Int(v int) *int { return &v }

// Bool convert a bool to a bool pointer.
func Bool(v bool) *bool { return &v }
