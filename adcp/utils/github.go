package utils

import (
	"fmt"
	"strings"

	"github.com/devplaninc/adcp/clients/go/adcp"
)

// ConvertToRawURL converts a github.com URL to raw.githubusercontent.com format.
// It handles various GitHub URL formats including /blob/ and /tree/ patterns.
// If a version is provided, it will be used; otherwise defaults to "main" branch.
func ConvertToRawURL(githubPath string, version *adcp.GitVersion) (string, error) {
	// If it's already a raw.githubusercontent.com URL or doesn't contain github.com, return as-is
	if strings.Contains(githubPath, "raw.githubusercontent.com") || !strings.Contains(githubPath, "github.com") {
		return githubPath, nil
	}

	// Convert github.com URL to raw.githubusercontent.com
	// Example: https://github.com/myorg/repo/blob/main/README.MD
	// To: https://raw.githubusercontent.com/myorg/repo/main/README.MD

	githubPath = strings.TrimPrefix(githubPath, "https://")
	githubPath = strings.TrimPrefix(githubPath, "http://")
	githubPath = strings.TrimPrefix(githubPath, "github.com/")

	// Handle both formats:
	// 1. owner/repo/file.md (no ref specified)
	// 2. owner/repo/blob/ref/file.md or owner/repo/tree/ref/file.md

	parts := strings.SplitN(githubPath, "/", 5)

	var owner, repo, ref, filePath string

	if len(parts) >= 4 && (parts[2] == "blob" || parts[2] == "tree") {
		// Format: owner/repo/blob|tree/ref/file.md
		if len(parts) < 5 {
			return "", fmt.Errorf("invalid github path format: %s", githubPath)
		}
		owner = parts[0]
		repo = parts[1]
		ref = parts[3]
		filePath = parts[4]
	} else if len(parts) >= 3 {
		// Format: owner/repo/file.md
		owner = parts[0]
		repo = parts[1]
		filePath = strings.Join(parts[2:], "/")

		// Use version from parameter if provided
		ref = "main"
		if version != nil && version.HasType() {
			switch version.WhichType() {
			case adcp.GitVersion_Tag_case:
				ref = version.GetTag()
			case adcp.GitVersion_Commit_case:
				ref = version.GetCommit()
			}
		}
	} else {
		return "", fmt.Errorf("invalid github path format: %s", githubPath)
	}

	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, filePath), nil
}
