package enricher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/mappers"
	"github.com/metraction/pharos/pkg/model"
)

var logger = logging.NewLogger("info", "component", "plugin")

func LoadEnrichersConfig(enrichersPath string) (*model.EnrichersConfig, error) {
	var enrichers *model.EnrichersConfig
	var err error
	// Check if args[0] points to enrichers.yaml file
	if filepath.Base(enrichersPath) == "enrichers.yaml" {
		logger.Info().Msgf("Loading Enrichers from file: %s\n", enrichersPath)
		enrichers, err = model.LoadEnrichersFromFile(enrichersPath)
		if err != nil {
			logger.Error().Msgf("Error loading Enrichers from %s: %v\n", enrichersPath, err)
			return nil, err
		}
		logger.Info().Msgf("Successfully loaded Enrichers with %d order items and %d sources\n",
			len(enrichers.Order), len(enrichers.Sources))
	} else {
		logger.Info().Msgf("Loading Enricher from directory: %s\n", enrichersPath)
		enrichers = &model.EnrichersConfig{
			Order: []string{"result"},
			Sources: []model.EnricherSource{
				{
					Name: "results",
					Path: enrichersPath,
				},
			},
		}
	}
	return enrichers, nil
}

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

func LoadEnricherConfig(enricherPath string, name string) model.EnricherConfig {
	var enricherDir, enricherFile string
	if filepath.Ext(enricherPath) == ".yaml" {
		enricherDir = filepath.Dir(enricherPath)
		enricherFile = filepath.Base(enricherPath)
	} else {
		enricherDir = enricherPath
		enricherFile = "enricher.yaml"
	}

	logger.Debug().Str("path", enricherDir).Str("file", enricherFile).Msg("Loading Enricher " + name)

	// Read the file
	data, err := os.ReadFile(filepath.Join(enricherDir, enricherFile))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to read config file")
	}
	mapperConfig, err := mappers.LoadMappersConfig(data)
	configs := mapperConfig[name]
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load mappers config")
	}
	enricherConfig := model.EnricherConfig{
		BasePath: enricherDir,
		Configs:  configs,
	}
	logger.Debug().Interface("config", enricherConfig).Msg("Loaded Enricher config")
	return enricherConfig
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

	// Try to clone the repository with the specified reference
	_, err := git.PlainClone(destinationDir, false, options)
	if err == nil {
		// Clone successful
		logger.Debug().Str("dir", destinationDir).Msg("Cloned Git repository to directory")
		return destinationDir, nil
	}

	// If there's no reference specified or the error is not about missing reference, return the error
	if ref == "" || err.Error() != fmt.Sprintf("reference not found: refs/heads/%s", ref) {
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	// Try cloning without branch specification (to try with commit hash)
	options.ReferenceName = ""
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
	if err != nil {
		return "", fmt.Errorf("failed to checkout commit %s: %w", ref, err)
	}

	logger.Debug().Str("dir", destinationDir).Msg("Cloned Git repository to directory")
	return destinationDir, nil
}
