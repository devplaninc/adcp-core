//go:build integration
// +build integration

package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/devplaninc/adcp/clients/go/adcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration Tests for Merge Functionality

func TestIDE_Materialize_Permissions_MergeWithExisting(t *testing.T) {
	// Setup: Create a temporary directory and existing settings file
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing settings file with some permissions
	existingSettings := `{
  "permissions": {
    "allow": [
      "Bash(git status:*)",
      "Read(/etc/hosts)"
    ],
    "deny": [
      "Write(/etc/**)"
    ],
    "ask": [
      "Bash(rm:*)"
    ]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(existingSettings), 0644))

	// Define new permissions to merge
	g := &IDE{}
	allowBash := adcp.OperationPermission_builder{Bash: strPtr("go test:*")}.Build()
	allowRead := adcp.OperationPermission_builder{Read: strPtr("~/.zshrc")}.Build()
	denyWrite := adcp.OperationPermission_builder{Write: strPtr("**/secrets/**")}.Build()

	ide := adcp.Ide_builder{
		Permissions: adcp.Permissions_builder{
			Allow: []*adcp.OperationPermission{allowBash, allowRead},
			Deny:  []*adcp.OperationPermission{denyWrite},
		}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify the merged result
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Verify existing permissions are preserved
	assert.Contains(t, parsed.Permissions.Allow, "Bash(git status:*)", "existing allow should be preserved")
	assert.Contains(t, parsed.Permissions.Allow, "Read(/etc/hosts)", "existing allow should be preserved")
	assert.Contains(t, parsed.Permissions.Deny, "Write(/etc/**)", "existing deny should be preserved")
	assert.Contains(t, parsed.Permissions.Ask, "Bash(rm:*)", "existing ask should be preserved")

	// Verify new permissions are added
	assert.Contains(t, parsed.Permissions.Allow, "Bash(go test:*)", "new allow should be added")
	assert.Contains(t, parsed.Permissions.Allow, "Read(~/.zshrc)", "new allow should be added")
	assert.Contains(t, parsed.Permissions.Deny, "Write(**/secrets/**)", "new deny should be added")

	// Verify total counts
	assert.Len(t, parsed.Permissions.Allow, 4, "should have 2 existing + 2 new allows")
	assert.Len(t, parsed.Permissions.Deny, 2, "should have 1 existing + 1 new deny")
	assert.Len(t, parsed.Permissions.Ask, 1, "should have 1 existing ask")
}

func TestIDE_Materialize_Permissions_Deduplication(t *testing.T) {
	// Setup: Create a temporary directory and existing settings file
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing settings file with duplicate permission
	existingSettings := `{
  "permissions": {
    "allow": [
      "Bash(go test:*)",
      "Read(/etc/hosts)"
    ],
    "deny": [],
    "ask": []
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(existingSettings), 0644))

	// Define new permissions including a duplicate
	g := &IDE{}
	allowBash := adcp.OperationPermission_builder{Bash: strPtr("go test:*")}.Build() // Duplicate!
	allowRead := adcp.OperationPermission_builder{Read: strPtr("~/.zshrc")}.Build()

	ide := adcp.Ide_builder{
		Permissions: adcp.Permissions_builder{
			Allow: []*adcp.OperationPermission{allowBash, allowRead},
		}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify the merged result
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Verify no duplicates
	assert.Len(t, parsed.Permissions.Allow, 3, "should have 3 unique allows (2 existing + 1 new, with 1 duplicate removed)")
	assert.Contains(t, parsed.Permissions.Allow, "Bash(go test:*)")
	assert.Contains(t, parsed.Permissions.Allow, "Read(/etc/hosts)")
	assert.Contains(t, parsed.Permissions.Allow, "Read(~/.zshrc)")

	// Count occurrences to ensure no duplicates
	count := 0
	for _, p := range parsed.Permissions.Allow {
		if p == "Bash(go test:*)" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate permission should appear only once")
}

func TestIDE_Materialize_Permissions_InvalidExistingJSON(t *testing.T) {
	// Setup: Create a temporary directory with invalid JSON
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing settings file with invalid JSON
	invalidJSON := `{ "permissions": { "allow": ["test" }`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(invalidJSON), 0644))

	// Define new permissions
	g := &IDE{}
	allowBash := adcp.OperationPermission_builder{Bash: strPtr("go test:*")}.Build()

	ide := adcp.Ide_builder{
		Permissions: adcp.Permissions_builder{
			Allow: []*adcp.OperationPermission{allowBash},
		}.Build(),
	}.Build()

	// Execute - should not error, just start fresh
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify the result contains only new permissions (old invalid JSON was ignored)
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Should only have new permission
	assert.Len(t, parsed.Permissions.Allow, 1)
	assert.Contains(t, parsed.Permissions.Allow, "Bash(go test:*)")
}

func TestIDE_Materialize_Mcp_MergeWithExisting(t *testing.T) {
	// Setup: Create a temporary directory and existing MCP file
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing MCP file
	existingMcp := `{
  "mcpServers": {
    "filesystem": {
      "command": "npx @modelcontextprotocol/server-filesystem"
    },
    "github": {
      "url": "https://old-api.github.com/mcp/"
    }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".mcp.json"), []byte(existingMcp), 0644))

	// Define new MCP servers to merge
	g := &IDE{}
	ide := adcp.Ide_builder{
		Mcp: adcp.Mcp_builder{Servers: map[string]*adcp.McpServer{
			"github":  adcp.McpServer_builder{Http: adcp.HttpMcpServer_builder{Url: "https://api.githubcopilot.com/mcp/"}.Build()}.Build(), // Update existing
			"devplan": adcp.McpServer_builder{Stdio: adcp.StdioMcpServer_builder{Command: "devplan mcp"}.Build()}.Build(),                  // Add new
		}}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)

	var mcpContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".mcp.json" {
			mcpContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, mcpContent)

	var parsed struct {
		McpServers map[string]map[string]string `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcpContent), &parsed))

	// Verify existing server not in new config is preserved
	require.Contains(t, parsed.McpServers, "filesystem", "existing server not in new config should be preserved")
	assert.Equal(t, "npx @modelcontextprotocol/server-filesystem", parsed.McpServers["filesystem"]["command"])

	// Verify existing server in new config is updated
	require.Contains(t, parsed.McpServers, "github", "existing server should be updated")
	assert.Equal(t, "https://api.githubcopilot.com/mcp/", parsed.McpServers["github"]["url"], "github server should be updated")

	// Verify new server is added
	require.Contains(t, parsed.McpServers, "devplan", "new server should be added")
	assert.Equal(t, "devplan mcp", parsed.McpServers["devplan"]["command"])

	// Verify total count
	assert.Len(t, parsed.McpServers, 3, "should have 3 servers total")
}

func TestIDE_Materialize_Mcp_InvalidExistingJSON(t *testing.T) {
	// Setup: Create a temporary directory with invalid JSON
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing MCP file with invalid JSON
	invalidJSON := `{ "mcpServers": { "test": }`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".mcp.json"), []byte(invalidJSON), 0644))

	// Define new MCP servers
	g := &IDE{}
	ide := adcp.Ide_builder{
		Mcp: adcp.Mcp_builder{Servers: map[string]*adcp.McpServer{
			"devplan": adcp.McpServer_builder{Stdio: adcp.StdioMcpServer_builder{Command: "devplan mcp"}.Build()}.Build(),
		}}.Build(),
	}.Build()

	// Execute - should not error, just start fresh
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)

	var mcpContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".mcp.json" {
			mcpContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, mcpContent)

	var parsed struct {
		McpServers map[string]map[string]string `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcpContent), &parsed))

	// Should only have new server
	assert.Len(t, parsed.McpServers, 1)
	require.Contains(t, parsed.McpServers, "devplan")
	assert.Equal(t, "devplan mcp", parsed.McpServers["devplan"]["command"])
}

func TestIDE_Materialize_Permissions_NoExistingFile(t *testing.T) {
	// Setup: Create a temporary directory without existing settings
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".claude"), 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Define new permissions
	g := &IDE{}
	allowBash := adcp.OperationPermission_builder{Bash: strPtr("go test:*")}.Build()

	ide := adcp.Ide_builder{
		Permissions: adcp.Permissions_builder{
			Allow: []*adcp.OperationPermission{allowBash},
		}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify the result contains only new permissions
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Should only have new permission
	assert.Len(t, parsed.Permissions.Allow, 1)
	assert.Contains(t, parsed.Permissions.Allow, "Bash(go test:*)")
	assert.Empty(t, parsed.Permissions.Deny)
	assert.Empty(t, parsed.Permissions.Ask)
}

func TestIDE_Materialize_McpServers_AutoAddPermissions(t *testing.T) {
	// Setup: Create a temporary directory
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Define MCP servers without explicit permissions
	g := &IDE{}
	ide := adcp.Ide_builder{
		Mcp: adcp.Mcp_builder{Servers: map[string]*adcp.McpServer{
			"github":     adcp.McpServer_builder{Http: adcp.HttpMcpServer_builder{Url: "https://api.github.com/mcp/"}.Build()}.Build(),
			"devplan":    adcp.McpServer_builder{Stdio: adcp.StdioMcpServer_builder{Command: "devplan mcp"}.Build()}.Build(),
			"filesystem": adcp.McpServer_builder{Stdio: adcp.StdioMcpServer_builder{Command: "npx @mcp/server-filesystem"}.Build()}.Build(),
		}}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify settings.local.json was created with MCP permissions
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent, "settings.local.json should be created even without explicit permissions")

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
		EnabledMcpjsonServers []string `json:"enabledMcpjsonServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Verify MCP server names were automatically added to enabledMcpjsonServers
	assert.Len(t, parsed.EnabledMcpjsonServers, 3, "should have all 3 MCP servers enabled")
	assert.Contains(t, parsed.EnabledMcpjsonServers, "github", "github MCP server should be enabled")
	assert.Contains(t, parsed.EnabledMcpjsonServers, "devplan", "devplan MCP server should be enabled")
	assert.Contains(t, parsed.EnabledMcpjsonServers, "filesystem", "filesystem MCP server should be enabled")

	// Verify MCP server permissions were also added to allow list with mcp__ prefix
	assert.Len(t, parsed.Permissions.Allow, 3, "should have mcp__ permissions for all 3 MCP servers")
	assert.Contains(t, parsed.Permissions.Allow, "mcp__github", "mcp__github permission should be in allow list")
	assert.Contains(t, parsed.Permissions.Allow, "mcp__devplan", "mcp__devplan permission should be in allow list")
	assert.Contains(t, parsed.Permissions.Allow, "mcp__filesystem", "mcp__filesystem permission should be in allow list")
}

func TestIDE_Materialize_McpServers_MergeWithExistingPermissions(t *testing.T) {
	// Setup: Create a temporary directory with existing permissions
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing settings with some permissions
	existingSettings := `{
  "permissions": {
    "allow": [
      "Bash(git status:*)",
      "Read(/etc/hosts)"
    ],
    "deny": [],
    "ask": []
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(existingSettings), 0644))

	// Define MCP servers and additional permissions
	g := &IDE{}
	allowBash := adcp.OperationPermission_builder{Bash: strPtr("go test:*")}.Build()

	ide := adcp.Ide_builder{
		Permissions: adcp.Permissions_builder{
			Allow: []*adcp.OperationPermission{allowBash},
		}.Build(),
		Mcp: adcp.Mcp_builder{Servers: map[string]*adcp.McpServer{
			"github":  adcp.McpServer_builder{Http: adcp.HttpMcpServer_builder{Url: "https://api.github.com/mcp/"}.Build()}.Build(),
			"devplan": adcp.McpServer_builder{Stdio: adcp.StdioMcpServer_builder{Command: "devplan mcp"}.Build()}.Build(),
		}}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify settings were merged
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
		EnabledMcpjsonServers []string `json:"enabledMcpjsonServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Verify existing permissions are preserved
	assert.Contains(t, parsed.Permissions.Allow, "Bash(git status:*)", "existing permission should be preserved")
	assert.Contains(t, parsed.Permissions.Allow, "Read(/etc/hosts)", "existing permission should be preserved")

	// Verify new explicit permission was added
	assert.Contains(t, parsed.Permissions.Allow, "Bash(go test:*)", "new permission should be added")

	// Verify MCP permissions were added to allow list
	assert.Contains(t, parsed.Permissions.Allow, "mcp__github", "mcp__github permission should be in allow list")
	assert.Contains(t, parsed.Permissions.Allow, "mcp__devplan", "mcp__devplan permission should be in allow list")

	// Verify total permission count (existing + new + MCP)
	assert.Len(t, parsed.Permissions.Allow, 5, "should have 2 existing + 1 new + 2 MCP permissions")

	// Verify MCP servers were added to enabledMcpjsonServers
	assert.Len(t, parsed.EnabledMcpjsonServers, 2, "should have 2 MCP servers enabled")
	assert.Contains(t, parsed.EnabledMcpjsonServers, "github", "github MCP server should be enabled")
	assert.Contains(t, parsed.EnabledMcpjsonServers, "devplan", "devplan MCP server should be enabled")
}

func TestIDE_Materialize_McpServers_PreserveExistingMcpPermissions(t *testing.T) {
	// Setup: Create a temporary directory with existing MCP permissions
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Change to temp directory for the test
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing settings with MCP server already enabled and in allow list
	existingSettings := `{
  "permissions": {
    "allow": [
      "Bash(git status:*)",
      "mcp__github"
    ],
    "deny": [],
    "ask": []
  },
  "enabledMcpjsonServers": ["github"]
}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(existingSettings), 0644))

	// Define the same MCP server again (should not duplicate)
	g := &IDE{}
	ide := adcp.Ide_builder{
		Mcp: adcp.Mcp_builder{Servers: map[string]*adcp.McpServer{
			"github": adcp.McpServer_builder{Http: adcp.HttpMcpServer_builder{Url: "https://api.github.com/mcp/"}.Build()}.Build(),
		}}.Build(),
	}.Build()

	// Execute
	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify no duplicate MCP server
	var settingsContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var parsed struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
		EnabledMcpjsonServers []string `json:"enabledMcpjsonServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))

	// Verify permissions remain with no duplicates
	assert.Len(t, parsed.Permissions.Allow, 2, "should have 2 permissions (no duplicate mcp__github)")
	assert.Contains(t, parsed.Permissions.Allow, "Bash(git status:*)")
	assert.Contains(t, parsed.Permissions.Allow, "mcp__github")

	// Count occurrences in allow list to ensure no duplicates
	allowCount := 0
	for _, p := range parsed.Permissions.Allow {
		if p == "mcp__github" {
			allowCount++
		}
	}
	assert.Equal(t, 1, allowCount, "mcp__github permission should appear only once in allow list")

	// Verify no duplicate MCP server in enabledMcpjsonServers
	assert.Len(t, parsed.EnabledMcpjsonServers, 1, "should have 1 enabled MCP server (no duplicate)")
	assert.Contains(t, parsed.EnabledMcpjsonServers, "github")

	// Count occurrences in enabledMcpjsonServers to ensure no duplicates
	enabledCount := 0
	for _, s := range parsed.EnabledMcpjsonServers {
		if s == "github" {
			enabledCount++
		}
	}
	assert.Equal(t, 1, enabledCount, "github server should appear only once in enabledMcpjsonServers")
}
