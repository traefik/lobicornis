package conf

// RepoConfig the repo configuration.
type RepoConfig struct {
	MergeMethod       *string `yaml:"mergeMethod,omitempty"`
	MinLightReview    *int    `yaml:"minLightReview,omitempty"`
	MinReview         *int    `yaml:"minReview,omitempty"`
	NeedMilestone     *bool   `yaml:"needMilestone,omitempty"`
	CheckNeedUpToDate *bool   `yaml:"checkNeedUpToDate,omitempty"`
	ForceNeedUpToDate *bool   `yaml:"forceNeedUpToDate,omitempty"`
	AddErrorInComment *bool   `yaml:"addErrorInComment,omitempty"`
	CommitMessage     *string `yaml:"commitMessage,omitempty"`
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

// GetCommitMessage gets commit message strategy.
func (r *RepoConfig) GetCommitMessage() string {
	if r.CommitMessage != nil {
		return *r.CommitMessage
	}

	return ""
}
