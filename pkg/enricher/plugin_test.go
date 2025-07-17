package enricher

import (
	"testing"
)

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "valid GitHub URL",
			url:  "https://github.com/metraction/pharos-plugins/tree/main/eos",
			want: true,
		},
		{
			name: "valid GitHub URL with commit hash",
			url:  "https://github.com/metraction/pharos-plugins/tree/10e1197c1e2e9aca608fa429a28606254aa8a2bf/eos",
			want: true,
		},
		{
			name: "valid GitLab URL",
			url:  "https://gitlab.com/metraction/pharos-plugins/-/tree/main/eos",
			want: true,
		},
		{
			name: "valid Bitbucket URL",
			url:  "https://bitbucket.org/metraction/pharos-plugins/src/main/eos",
			want: true,
		},
		{
			name: "HTTP URL",
			url:  "http://example.com/repo",
			want: true,
		},
		{
			name: "local path",
			url:  "/path/to/enricher",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGitURL(tt.url); got != tt.want {
				t.Errorf("isGitURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantRepoURL string
		wantRef     string
		wantDir     string
		wantErr     bool
	}{
		{
			name:        "GitHub URL with directory",
			url:         "https://github.com/metraction/pharos-plugins/tree/main/eos",
			wantRepoURL: "https://github.com/metraction/pharos-plugins.git",
			wantRef:     "main",
			wantDir:     "eos",
			wantErr:     false,
		},
		{
			name:        "GitHub URL with commit hash",
			url:         "https://github.com/metraction/pharos-plugins/tree/10e1197c1e2e9aca608fa429a28606254aa8a2bf/eos",
			wantRepoURL: "https://github.com/metraction/pharos-plugins.git",
			wantRef:     "10e1197c1e2e9aca608fa429a28606254aa8a2bf",
			wantDir:     "eos",
			wantErr:     false,
		},
		{
			name:        "GitHub URL with nested directory",
			url:         "https://github.com/metraction/pharos-plugins/tree/main/eos/nested/dir",
			wantRepoURL: "https://github.com/metraction/pharos-plugins.git",
			wantRef:     "main",
			wantDir:     "eos/nested/dir",
			wantErr:     false,
		},
		{
			name:        "GitHub URL without directory",
			url:         "https://github.com/metraction/pharos-plugins/tree/main",
			wantRepoURL: "https://github.com/metraction/pharos-plugins.git",
			wantRef:     "main",
			wantDir:     "",
			wantErr:     false,
		},
		{
			name:        "GitLab URL with directory",
			url:         "https://gitlab.com/metraction/pharos-plugins/-/tree/main/eos",
			wantRepoURL: "https://gitlab.com/metraction/pharos-plugins.git",
			wantRef:     "main",
			wantDir:     "eos",
			wantErr:     false,
		},
		{
			name:        "Bitbucket URL with directory",
			url:         "https://bitbucket.org/metraction/pharos-plugins/src/main/eos",
			wantRepoURL: "https://bitbucket.org/metraction/pharos-plugins.git",
			wantRef:     "main",
			wantDir:     "eos",
			wantErr:     false,
		},
		{
			name:    "invalid URL format",
			url:     "https://github.com/metraction",
			wantErr: true,
		},
		{
			name:    "not a Git URL",
			url:     "http://example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoURL, ref, dir, err := parseGitURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if repoURL != tt.wantRepoURL {
					t.Errorf("parseGitURL() repoURL = %v, want %v", repoURL, tt.wantRepoURL)
				}
				if ref != tt.wantRef {
					t.Errorf("parseGitURL() ref = %v, want %v", ref, tt.wantRef)
				}
				if dir != tt.wantDir {
					t.Errorf("parseGitURL() dir = %v, want %v", dir, tt.wantDir)
				}
			}
		})
	}
}
