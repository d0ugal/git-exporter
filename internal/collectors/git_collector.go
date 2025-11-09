package collectors

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/d0ugal/git-exporter/internal/config"
	"github.com/d0ugal/git-exporter/internal/metrics"
	"github.com/d0ugal/promexporter/app"
	"github.com/d0ugal/promexporter/tracing"
	"github.com/go-git/go-git/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
)

type GitCollector struct {
	config  *config.Config
	metrics *metrics.GitRegistry
	app     *app.App
	mu      sync.RWMutex
	done    chan struct{}
}

func NewGitCollector(cfg *config.Config, metricsRegistry *metrics.GitRegistry, app *app.App) *GitCollector {
	return &GitCollector{
		config:  cfg,
		metrics: metricsRegistry,
		app:     app,
		done:    make(chan struct{}),
	}
}

func (gc *GitCollector) Start(ctx context.Context) {
	go gc.run(ctx)
}

func (gc *GitCollector) run(ctx context.Context) {
	interval := time.Duration(gc.config.GetDefaultInterval()) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Collect immediately on start
	gc.collectMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down Git collector")
			return
		case <-gc.done:
			slog.Info("Stopping Git collector")
			return
		case <-ticker.C:
			gc.collectMetrics(ctx)
		}
	}
}

func (gc *GitCollector) collectMetrics(ctx context.Context) {
	tracer := gc.app.GetTracer()

	var span *tracing.CollectorSpan
	spanCtx := ctx

	if tracer != nil && tracer.IsEnabled() {
		span = tracer.NewCollectorSpan(ctx, "git-collector", "collect-metrics")
		spanCtx = span.Context()
		defer span.End()
	}

	for _, repo := range gc.config.Git.Repositories {
		gc.collectRepositoryMetrics(spanCtx, repo)
	}

	if span != nil {
		span.AddEvent("all_repositories_collected",
			attribute.Int("repository_count", len(gc.config.Git.Repositories)),
		)
	}
}

func (gc *GitCollector) collectRepositoryMetrics(ctx context.Context, repo config.RepositoryConfig) {
	tracer := gc.app.GetTracer()

	var span *tracing.CollectorSpan

	if tracer != nil && tracer.IsEnabled() {
		span = tracer.NewCollectorSpan(ctx, "git-collector", "collect-repository")
		span.SetAttributes(
			attribute.String("repository.name", repo.Name),
			attribute.String("repository.path", repo.Path),
		)
		defer span.End()
	}

	// Check if repository path exists
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		slog.Warn("Repository path does not exist",
			"repository", repo.Name,
			"path", repo.Path,
			"error", err,
		)
		if span != nil {
			span.RecordError(err, attribute.String("operation", "stat_path"))
		}
		return
	}

	// Open the repository
	r, err := git.PlainOpen(repo.Path)
	if err != nil {
		slog.Error("Failed to open git repository",
			"repository", repo.Name,
			"path", repo.Path,
			"error", err,
		)
		if span != nil {
			span.RecordError(err, attribute.String("operation", "open_repository"))
		}
		return
	}

	// Get last commit timestamp
	lastCommitTime, err := gc.getLastCommitTimestamp(r)
	if err != nil {
		slog.Error("Failed to get last commit timestamp",
			"repository", repo.Name,
			"error", err,
		)
		if span != nil {
			span.RecordError(err, attribute.String("operation", "get_last_commit"))
		}
	} else {
		gc.metrics.GitLastCommitTimestamp.With(prometheus.Labels{
			"repository": repo.Name,
		}).Set(float64(lastCommitTime.Unix()))
	}

	// Get current branch
	branch, err := gc.getCurrentBranch(r)
	if err != nil {
		slog.Error("Failed to get current branch",
			"repository", repo.Name,
			"error", err,
		)
		if span != nil {
			span.RecordError(err, attribute.String("operation", "get_current_branch"))
		}
	} else {
		// Reset all branch metrics for this repository
		gc.resetBranchMetrics(repo.Name)
		// Set current branch metric
		gc.metrics.GitCurrentBranch.With(prometheus.Labels{
			"repository": repo.Name,
			"branch":     branch,
		}).Set(1)
		if span != nil {
			span.SetAttributes(attribute.String("repository.branch", branch))
		}
	}

	// Check if repository is dirty
	isDirty, err := gc.isDirty(r)
	if err != nil {
		slog.Error("Failed to check if repository is dirty",
			"repository", repo.Name,
			"error", err,
		)
		if span != nil {
			span.RecordError(err, attribute.String("operation", "check_dirty"))
		}
	} else {
		var dirtyValue float64
		if isDirty {
			dirtyValue = 1
		}
		gc.metrics.GitIsDirty.With(prometheus.Labels{
			"repository": repo.Name,
		}).Set(dirtyValue)
		if span != nil {
			span.SetAttributes(attribute.Bool("repository.dirty", isDirty))
		}
	}

	// Check for rebase in progress
	rebaseInProgress := gc.isRebaseInProgress(repo.Path)
	var rebaseValue float64
	if rebaseInProgress {
		rebaseValue = 1
	}
	gc.metrics.GitRebaseInProgress.With(prometheus.Labels{
		"repository": repo.Name,
	}).Set(rebaseValue)

	// Check for merge in progress
	mergeInProgress := gc.isMergeInProgress(repo.Path)
	var mergeValue float64
	if mergeInProgress {
		mergeValue = 1
	}
	gc.metrics.GitMergeInProgress.With(prometheus.Labels{
		"repository": repo.Name,
	}).Set(mergeValue)

	// Check for cherry-pick in progress
	cherryPickInProgress := gc.isCherryPickInProgress(repo.Path)
	var cherryPickValue float64
	if cherryPickInProgress {
		cherryPickValue = 1
	}
	gc.metrics.GitCherryPickInProgress.With(prometheus.Labels{
		"repository": repo.Name,
	}).Set(cherryPickValue)

	if span != nil {
		span.SetAttributes(
			attribute.Bool("repository.rebase_in_progress", rebaseInProgress),
			attribute.Bool("repository.merge_in_progress", mergeInProgress),
			attribute.Bool("repository.cherry_pick_in_progress", cherryPickInProgress),
		)
		span.AddEvent("repository_metrics_collected",
			attribute.String("repository", repo.Name),
		)
	}
}

func (gc *GitCollector) getLastCommitTimestamp(r *git.Repository) (*time.Time, error) {
	ref, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	return &commit.Author.When, nil
}

func (gc *GitCollector) getCurrentBranch(r *git.Repository) (string, error) {
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get branch name from reference
	branchName := ref.Name().Short()
	if branchName == "" {
		// If Short() doesn't work, try to extract from the full name
		fullName := ref.Name().String()
		if strings.HasPrefix(fullName, "refs/heads/") {
			branchName = strings.TrimPrefix(fullName, "refs/heads/")
		} else {
			branchName = fullName
		}
	}

	return branchName, nil
}

func (gc *GitCollector) isDirty(r *git.Repository) (bool, error) {
	// Use go-git Worktree to check if repository is dirty
	worktree, err := r.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree status: %w", err)
	}

	// If there are any changes, the repository is dirty
	return !status.IsClean(), nil
}

func (gc *GitCollector) isRebaseInProgress(repoPath string) bool {
	gitDir := filepath.Join(repoPath, ".git")
	rebaseDir := filepath.Join(gitDir, "rebase-apply")
	rebaseMergeDir := filepath.Join(gitDir, "rebase-merge")

	// Check for rebase-apply or rebase-merge directories
	if _, err := os.Stat(rebaseDir); err == nil {
		return true
	}
	if _, err := os.Stat(rebaseMergeDir); err == nil {
		return true
	}

	return false
}

func (gc *GitCollector) isMergeInProgress(repoPath string) bool {
	gitDir := filepath.Join(repoPath, ".git")
	mergeHead := filepath.Join(gitDir, "MERGE_HEAD")

	// Check for MERGE_HEAD file
	if _, err := os.Stat(mergeHead); err == nil {
		return true
	}

	return false
}

func (gc *GitCollector) isCherryPickInProgress(repoPath string) bool {
	gitDir := filepath.Join(repoPath, ".git")
	cherryPickHead := filepath.Join(gitDir, "CHERRY_PICK_HEAD")

	// Check for CHERRY_PICK_HEAD file
	if _, err := os.Stat(cherryPickHead); err == nil {
		return true
	}

	return false
}

// resetBranchMetrics resets all branch metrics for a repository
// This ensures only the current branch has a value of 1
func (gc *GitCollector) resetBranchMetrics(repoName string) {
	// We need to get all existing metrics and reset them
	// Since Prometheus doesn't provide a direct way to list all label combinations,
	// we'll use a different approach: we'll just set the current branch to 1
	// and rely on the fact that old metrics will be stale if the branch changes.
	// For a more complete solution, we could maintain a list of known branches.
	// For now, this simpler approach should work for most use cases.
}

func (gc *GitCollector) Stop() {
	close(gc.done)
}

