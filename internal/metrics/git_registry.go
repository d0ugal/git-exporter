package metrics

import (
	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// GitRegistry wraps the promexporter registry with Git-specific metrics
type GitRegistry struct {
	*promexporter_metrics.Registry

	// Git repository metrics
	GitLastCommitTimestamp *prometheus.GaugeVec
	GitCurrentBranch       *prometheus.GaugeVec
	GitIsDirty             *prometheus.GaugeVec
	GitRebaseInProgress    *prometheus.GaugeVec
	GitMergeInProgress     *prometheus.GaugeVec
	GitCherryPickInProgress *prometheus.GaugeVec
}

// NewGitRegistry creates a new Git metrics registry
func NewGitRegistry(baseRegistry *promexporter_metrics.Registry) *GitRegistry {
	// Get the underlying Prometheus registry
	promRegistry := baseRegistry.GetRegistry()
	factory := promauto.With(promRegistry)

	git := &GitRegistry{
		Registry: baseRegistry,
	}

	// Git last commit timestamp
	git.GitLastCommitTimestamp = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "git_last_commit_timestamp",
			Help: "Unix timestamp of the last commit in the repository",
		},
		[]string{"repository"},
	)

	baseRegistry.AddMetricInfo("git_last_commit_timestamp", "Unix timestamp of the last commit in the repository", []string{"repository"})

	// Git current branch (as a gauge with value 1 if branch exists)
	git.GitCurrentBranch = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "git_current_branch",
			Help: "Current branch name (value is always 1, branch name is in the label)",
		},
		[]string{"repository", "branch"},
	)

	baseRegistry.AddMetricInfo("git_current_branch", "Current branch name (value is always 1, branch name is in the label)", []string{"repository", "branch"})

	// Git is dirty (1 = dirty, 0 = clean)
	git.GitIsDirty = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "git_is_dirty",
			Help: "Whether the repository has uncommitted changes (1 = dirty, 0 = clean)",
		},
		[]string{"repository"},
	)

	baseRegistry.AddMetricInfo("git_is_dirty", "Whether the repository has uncommitted changes (1 = dirty, 0 = clean)", []string{"repository"})

	// Git rebase in progress (1 = in progress, 0 = not in progress)
	git.GitRebaseInProgress = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "git_rebase_in_progress",
			Help: "Whether a rebase operation is in progress (1 = in progress, 0 = not in progress)",
		},
		[]string{"repository"},
	)

	baseRegistry.AddMetricInfo("git_rebase_in_progress", "Whether a rebase operation is in progress (1 = in progress, 0 = not in progress)", []string{"repository"})

	// Git merge in progress (1 = in progress, 0 = not in progress)
	git.GitMergeInProgress = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "git_merge_in_progress",
			Help: "Whether a merge operation is in progress (1 = in progress, 0 = not in progress)",
		},
		[]string{"repository"},
	)

	baseRegistry.AddMetricInfo("git_merge_in_progress", "Whether a merge operation is in progress (1 = in progress, 0 = not in progress)", []string{"repository"})

	// Git cherry-pick in progress (1 = in progress, 0 = not in progress)
	git.GitCherryPickInProgress = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "git_cherry_pick_in_progress",
			Help: "Whether a cherry-pick operation is in progress (1 = in progress, 0 = not in progress)",
		},
		[]string{"repository"},
	)

	baseRegistry.AddMetricInfo("git_cherry_pick_in_progress", "Whether a cherry-pick operation is in progress (1 = in progress, 0 = not in progress)", []string{"repository"})

	return git
}

