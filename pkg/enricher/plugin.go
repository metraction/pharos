package enricher

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
)

var logger = logging.NewLogger("info", "component", "plugin")

func FetchEnricherFromGit(enricherUri string, destinationDir string) (enricherDir string, err error) {
	// Check if enricherPath is a Git repository URL
	if !isGitURL(enricherUri) {
		return "", fmt.Errorf("Enricher path %s is not supported a Git repository URL", enricherUri)
	}

	// Parse Git URL
	repoURL, ref, dir, err := parseGitURL(enricherUri)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse Git repository URL")
	}

	// Clone the repository
	cloneDir, err := cloneGitRepo(destinationDir, repoURL, ref)
	if err != nil {
		logger.Fatal().Str("url", repoURL).Err(err).Msg("Failed to clone Git repository")
	}

	// Use the cloned repository directory + subdirectory as the enricher path
	enricherDir = filepath.Join(cloneDir, dir)
	logger.Debug().Str("path", enricherDir).Msg("Using cloned repository directory")
	return enricherDir, nil
}

func LoadEnricher(enricherPath string, name string, source streams.Source) streams.Flow {
	logger.Debug().Str("path", enricherPath).Msg("Loading Enricher " + name)

	mapperConfig, err := mappers.LoadMappersConfig("results", filepath.Join(enricherPath, "enricher.yaml"))
	if err != nil {
		logger.Fatal().Err(err).Str("path", enricherPath).Msg("Failed to load mappers config")
	}
	enricherConfig := model.EnricherConfig{
		BasePath: enricherPath,
		Configs:  mapperConfig,
	}
	return mappers.NewResultEnricherStream(source, name, enricherConfig)
}

// isGitURL checks if the path is a Git repository URL
func isGitURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// parseGitURL extracts repository URL, reference, and directory path from a URL
// Supports formats like:
// - https://github.com/owner/repo/tree/branch/dir
// - https://username:token@github.com/owner/repo/tree/branch/dir
// - https://gitlab.com/owner/repo/-/tree/branch/dir
// - https://bitbucket.org/owner/repo/src/branch/dir
// - Any other URL with a reference and directory path pattern
func parseGitURL(url string) (repoURL, ref, dir string, err error) {
	// Extract credentials if present
	var credentials string
	credentialsPattern := regexp.MustCompile(`^(https?://)([^@]+@)(.+)$`)
	if matches := credentialsPattern.FindStringSubmatch(url); len(matches) == 4 {
		credentials = matches[2] // username:password@
		// Remove credentials from URL for pattern matching
		url = matches[1] + matches[3] // https:// + rest of URL
	}

	// Try GitHub format first
	ghPattern := regexp.MustCompile(`^(https?://github\.com/[^/]+/[^/]+)/tree/([^/]+)/?(.*)$`)
	if matches := ghPattern.FindStringSubmatch(url); len(matches) == 4 {
		// Reinsert credentials if they existed
		if credentials != "" {
			return matches[1][:8] + credentials + matches[1][8:] + ".git", matches[2], matches[3], nil
		}
		return matches[1] + ".git", matches[2], matches[3], nil
	}

	// Try GitLab format
	glPattern := regexp.MustCompile(`^(https?://gitlab\.com/[^/]+/[^/]+)/-/tree/([^/]+)/?(.*)$`)
	if matches := glPattern.FindStringSubmatch(url); len(matches) == 4 {
		// Reinsert credentials if they existed
		if credentials != "" {
			return matches[1][:8] + credentials + matches[1][8:] + ".git", matches[2], matches[3], nil
		}
		return matches[1] + ".git", matches[2], matches[3], nil
	}

	// Try Bitbucket format
	bbPattern := regexp.MustCompile(`^(https?://bitbucket\.org/[^/]+/[^/]+)/src/([^/]+)/?(.*)$`)
	if matches := bbPattern.FindStringSubmatch(url); len(matches) == 4 {
		// Reinsert credentials if they existed
		if credentials != "" {
			return matches[1][:8] + credentials + matches[1][8:] + ".git", matches[2], matches[3], nil
		}
		return matches[1] + ".git", matches[2], matches[3], nil
	}

	// Generic pattern for other Git hosting services
	// This is a fallback and might need adjustment for specific services
	genericPattern := regexp.MustCompile(`^(https?://[^/]+/[^/]+/[^/]+)/([^/]+)/([^/]+)/?(.*)$`)
	if matches := genericPattern.FindStringSubmatch(url); len(matches) == 5 {
		// Reinsert credentials if they existed
		if credentials != "" {
			return matches[1][:8] + credentials + matches[1][8:] + ".git", matches[3], matches[4], nil
		}
		return matches[1] + ".git", matches[3], matches[4], nil
	}

	return "", "", "", fmt.Errorf("unable to parse Git URL format: %s", url)
}

// cloneGitRepo clones a Git repository to a temporary directory using go-git
func cloneGitRepo(destinationDir, repoURL, ref string) (string, error) {
	// Clone options
	options := &git.CloneOptions{
		URL:               repoURL,
		SingleBranch:      true,
		Depth:             1,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}

	// If ref is provided, set the reference to checkout
	if ref != "" {
		options.ReferenceName = plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", ref))
	}

	// Clone the repository
	_, err := git.PlainClone(destinationDir, false, options)
	if err != nil {
		// If branch checkout fails, try with commit hash
		if ref != "" && err.Error() == fmt.Sprintf("reference not found: refs/heads/%s", ref) {
			// Try cloning without branch specification
			options.ReferenceName = ""

			// Clone without branch specification
			r, err := git.PlainClone(destinationDir, false, options)
			if err != nil {
				return "", fmt.Errorf("git clone failed: %w", err)
			}

			// Checkout the commit hash
			w, err := r.Worktree()
			if err != nil {
				return "", fmt.Errorf("failed to get worktree: %w", err)
			}

			// Checkout options with commit hash
			checkoutOptions := &git.CheckoutOptions{
				Hash: plumbing.NewHash(ref),
			}

			// Checkout the commit
			err = w.Checkout(checkoutOptions)
		}
	}
	logger.Debug().Str("dir", destinationDir).Msg("Cloned Git repository to directory")
	return destinationDir, nil
}
