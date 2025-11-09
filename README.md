# Git Exporter

A Prometheus exporter for Git repository monitoring that exposes metrics about repository state, including last commit timestamp, current branch, dirty status, and ongoing operations (rebase, merge, cherry-pick).

## Metrics

### Git Repository Metrics
- `git_last_commit_timestamp` - Unix timestamp of the last commit in the repository
- `git_current_branch` - Current branch name (value is always 1, branch name is in the label)
- `git_is_dirty` - Whether the repository has uncommitted changes (1 = dirty, 0 = clean)
- `git_rebase_in_progress` - Whether a rebase operation is in progress (1 = in progress, 0 = not in progress)
- `git_merge_in_progress` - Whether a merge operation is in progress (1 = in progress, 0 = not in progress)
- `git_cherry_pick_in_progress` - Whether a cherry-pick operation is in progress (1 = in progress, 0 = not in progress)

### Endpoints
- `GET /`: Service information
- `GET /health`: Health check endpoint
- `GET /metrics`: Prometheus metrics endpoint

## Quick Start

### Configuration

Create a `config.yaml` file to configure Git repositories to monitor. You can either specify repositories explicitly or use discovery patterns to automatically find all Git repositories in a directory.

### Explicit Repository Configuration

Specify repositories individually with a name and path:

```yaml
git:
  repositories:
    - name: "my-repo"
      path: "/path/to/repository"
```

### Discovery Patterns

Use the `discover` field to automatically find all Git repositories in a directory. The exporter will recursively scan the specified directories and discover all Git repositories:

```yaml
git:
  discover:
    - "/home/user/Code"  # Discover all repos in the Code directory
```

You can combine both explicit repositories and discovery patterns:

```yaml
git:
  repositories:
    - name: "special-repo"
      path: "/path/to/special/repository"
  discover:
    - "/home/user/Code"  # Also discover all repos in Code directory
```

When using discovery, repository names are automatically generated from the relative path from the discovery root. For example, if `/home/user/Code` contains `project1` and `project2`, they will be named `project1` and `project2` respectively.

### Full Configuration Example

```yaml
server:
  host: "0.0.0.0"
  port: 8080

logging:
  level: "info"
  format: "json"

metrics:
  collection:
    default_interval: "30s"

git:
  # Explicit repositories (optional)
  repositories:
    - name: "my-repo"
      path: "/path/to/repository"
    - name: "another-repo"
      path: "/path/to/another/repository"
  
  # Discovery patterns - automatically find all Git repositories in these directories
  # Each directory will be scanned recursively for Git repositories
  discover:
    - "/home/user/Code"  # Discover all repos in the Code directory
    # - "/path/to/other/directory"  # Add more directories as needed
```

### Docker Compose

```yaml
version: '3.8'
services:
  git-exporter:
    image: ghcr.io/d0ugal/git-exporter:latest
    ports:
      - "8080:8080"
    volumes:
      - /path/to/repos:/repos:ro
      - ./config.yaml:/app/config.yaml:ro
    environment:
      - CONFIG_PATH=/app/config.yaml
    restart: unless-stopped
```

## Building

```bash
make build
```

## Testing

```bash
make test
```

## Linting

```bash
make lint
```

## License

See LICENSE file for details.

