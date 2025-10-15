package recipes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devplaninc/adcp/clients/go/adcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func TestRecipe_Materialize_NilRecipe(t *testing.T) {
	r := &Recipe{}
	_, err := r.Materialize(context.Background(), nil)
	assert.Error(t, err, "expected error for nil recipe")
}

func TestRecipe_Materialize_EmptyRecipe(t *testing.T) {
	r := &Recipe{}
	recipe := adcp.Recipe_builder{}.Build()
	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.GetEntries())
}

func TestRecipe_Materialize_ContextOnly(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Context: adcp.Context_builder{
			Entries: []*adcp.ContextEntry{
				adcp.ContextEntry_builder{
					Path: "README.md",
					From: adcp.ContextFrom_builder{
						Text: strPtr("# Project README"),
					}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.GetEntries(), 1)

	entry := result.GetEntries()[0]
	assert.Equal(t, "README.md", entry.GetFile().GetPath())
	assert.Equal(t, "# Project README", entry.GetFile().GetContent())
}

func TestRecipe_Materialize_IdeOnly(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Ide: adcp.Ide_builder{
			Commands: adcp.Commands_builder{
				Entries: []*adcp.Command{
					adcp.Command_builder{
						Name: "test",
						From: adcp.CommandFrom_builder{
							Text: strPtr("Run all tests"),
						}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.GetEntries(), 1)

	entry := result.GetEntries()[0]
	assert.Equal(t, ".claude/commands/test.md", entry.GetFile().GetPath())
	assert.Equal(t, "Run all tests", entry.GetFile().GetContent())
}

func TestRecipe_Materialize_ContextAndIde(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Context: adcp.Context_builder{
			Entries: []*adcp.ContextEntry{
				adcp.ContextEntry_builder{
					Path: "docs/arch.md",
					From: adcp.ContextFrom_builder{
						Text: strPtr("# Architecture"),
					}.Build(),
				}.Build(),
			},
		}.Build(),
		Ide: adcp.Ide_builder{
			Commands: adcp.Commands_builder{
				Entries: []*adcp.Command{
					adcp.Command_builder{
						Name: "deploy",
						From: adcp.CommandFrom_builder{
							Text: strPtr("Deploy to production"),
						}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.GetEntries(), 2)

	// Build map for easier verification
	entries := make(map[string]string)
	for _, e := range result.GetEntries() {
		entries[e.GetFile().GetPath()] = e.GetFile().GetContent()
	}

	assert.Equal(t, "# Architecture", entries["docs/arch.md"])
	assert.Equal(t, "Deploy to production", entries[".claude/commands/deploy.md"])
}

func TestRecipe_Materialize_ComplexRecipe(t *testing.T) {
	r := &Recipe{}

	// Mock HTTP server for GitHub content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GitHub content"))
	}))
	defer server.Close()

	recipe := adcp.Recipe_builder{
		Context: adcp.Context_builder{
			Entries: []*adcp.ContextEntry{
				adcp.ContextEntry_builder{
					Path: "context/intro.md",
					From: adcp.ContextFrom_builder{
						Text: strPtr("# Introduction"),
					}.Build(),
				}.Build(),
				adcp.ContextEntry_builder{
					Path: "context/from-cmd.txt",
					From: adcp.ContextFrom_builder{
						Cmd: strPtr("echo 'command output'"),
					}.Build(),
				}.Build(),
				adcp.ContextEntry_builder{
					Path: "context/from-github.md",
					From: adcp.ContextFrom_builder{
						Github: adcp.GitReference_builder{
							Path: server.URL,
						}.Build(),
					}.Build(),
				}.Build(),
			},
		}.Build(),
		Ide: adcp.Ide_builder{
			Commands: adcp.Commands_builder{
				Entries: []*adcp.Command{
					adcp.Command_builder{
						Name: "lint",
						From: adcp.CommandFrom_builder{
							Text: strPtr("Run linting"),
						}.Build(),
					}.Build(),
					adcp.Command_builder{
						Name: "format",
						From: adcp.CommandFrom_builder{
							Cmd: strPtr("echo 'format code'"),
						}.Build(),
					}.Build(),
				},
			}.Build(),
			Permissions: adcp.Permissions_builder{
				Allow: []*adcp.OperationPermission{
					adcp.OperationPermission_builder{
						Bash: strPtr("go test:*"),
					}.Build(),
				},
				Deny: []*adcp.OperationPermission{
					adcp.OperationPermission_builder{
						Write: strPtr("**/secrets/**"),
					}.Build(),
				},
			}.Build(),
			Mcp: adcp.Mcp_builder{
				Servers: map[string]*adcp.McpServer{
					"test-server": adcp.McpServer_builder{
						Stdio: adcp.StdioMcpServer_builder{
							Command: "test-mcp-server",
						}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Build map for easier verification
	entries := make(map[string]string)
	for _, e := range result.GetEntries() {
		entries[e.GetFile().GetPath()] = e.GetFile().GetContent()
	}

	// Verify context entries
	assert.Equal(t, "# Introduction", entries["context/intro.md"])
	assert.Equal(t, "command output\n", entries["context/from-cmd.txt"])
	assert.Equal(t, "GitHub content", entries["context/from-github.md"])

	// Verify command entries
	assert.Equal(t, "Run linting", entries[".claude/commands/lint.md"])
	assert.Equal(t, "format code\n", entries[".claude/commands/format.md"])

	// Verify permissions
	settingsContent := entries[".claude/settings.local.json"]
	require.NotEmpty(t, settingsContent)

	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &settings))
	assert.Contains(t, settings.Permissions.Allow, "Bash(go test:*)")
	assert.Contains(t, settings.Permissions.Deny, "Write(**/secrets/**)")

	// Verify MCP
	mcpContent := entries[".claude/mcp.local.json"]
	require.NotEmpty(t, mcpContent)

	var mcp struct {
		McpServers map[string]map[string]string `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcpContent), &mcp))
	assert.Contains(t, mcp.McpServers, "test-server")
	assert.Equal(t, "test-mcp-server", mcp.McpServers["test-server"]["command"])
}

func TestRecipe_Materialize_InvalidContext(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Context: adcp.Context_builder{
			Entries: []*adcp.ContextEntry{
				adcp.ContextEntry_builder{
					Path: "test.md",
					// Missing From source
				}.Build(),
			},
		}.Build(),
	}.Build()

	_, err := r.Materialize(context.Background(), recipe)
	assert.Error(t, err, "expected error for invalid context entry")
	assert.Contains(t, err.Error(), "failed to materialize context")
}

func TestRecipe_Materialize_InvalidIde(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Ide: adcp.Ide_builder{
			Commands: adcp.Commands_builder{
				Entries: []*adcp.Command{
					adcp.Command_builder{
						Name: "invalid",
						// Missing From source
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	_, err := r.Materialize(context.Background(), recipe)
	assert.Error(t, err, "expected error for invalid IDE command")
	assert.Contains(t, err.Error(), "failed to materialize IDE configuration")
}

func TestRecipe_Materialize_ContextWithCombinedSource(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Context: adcp.Context_builder{
			Entries: []*adcp.ContextEntry{
				adcp.ContextEntry_builder{
					Path: "combined.md",
					From: adcp.ContextFrom_builder{
						Combined: adcp.CombinedContextSource_builder{
							Items: []*adcp.CombinedContextSource_Item{
								adcp.CombinedContextSource_Item_builder{
									Text: strPtr("# Header\n"),
								}.Build(),
								adcp.CombinedContextSource_Item_builder{
									Cmd: strPtr("echo 'Body content'"),
								}.Build(),
								adcp.CombinedContextSource_Item_builder{
									Text: strPtr("\n# Footer"),
								}.Build(),
							},
						}.Build(),
					}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.GetEntries(), 1)

	entry := result.GetEntries()[0]
	assert.Equal(t, "combined.md", entry.GetFile().GetPath())
	assert.Equal(t, "# Header\nBody content\n\n# Footer", entry.GetFile().GetContent())
}

func TestRecipe_Materialize_MultiplePermissions(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Ide: adcp.Ide_builder{
			Permissions: adcp.Permissions_builder{
				Allow: []*adcp.OperationPermission{
					adcp.OperationPermission_builder{Bash: strPtr("make:*")}.Build(),
					adcp.OperationPermission_builder{Read: strPtr("**/*.go")}.Build(),
					adcp.OperationPermission_builder{Write: strPtr("**/*.md")}.Build(),
				},
				Deny: []*adcp.OperationPermission{
					adcp.OperationPermission_builder{Bash: strPtr("rm -rf:*")}.Build(),
					adcp.OperationPermission_builder{Write: strPtr("/etc/**")}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.GetEntries(), 1)

	var settingsContent string
	for _, e := range result.GetEntries() {
		if e.GetFile().GetPath() == ".claude/settings.local.json" {
			settingsContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, settingsContent)

	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &settings))
	assert.Len(t, settings.Permissions.Allow, 3)
	assert.Len(t, settings.Permissions.Deny, 2)
	assert.Contains(t, settings.Permissions.Allow, "Bash(make:*)")
	assert.Contains(t, settings.Permissions.Allow, "Read(**/*.go)")
	assert.Contains(t, settings.Permissions.Allow, "Write(**/*.md)")
	assert.Contains(t, settings.Permissions.Deny, "Bash(rm -rf:*)")
	assert.Contains(t, settings.Permissions.Deny, "Write(/etc/**)")
}

func TestRecipe_Materialize_MultipleMcpServers(t *testing.T) {
	r := &Recipe{}

	recipe := adcp.Recipe_builder{
		Ide: adcp.Ide_builder{
			Mcp: adcp.Mcp_builder{
				Servers: map[string]*adcp.McpServer{
					"http-server": adcp.McpServer_builder{
						Http: adcp.HttpMcpServer_builder{
							Url: "https://example.com/mcp",
						}.Build(),
					}.Build(),
					"stdio-server": adcp.McpServer_builder{
						Stdio: adcp.StdioMcpServer_builder{
							Command: "stdio-mcp",
						}.Build(),
					}.Build(),
					"another-stdio": adcp.McpServer_builder{
						Stdio: adcp.StdioMcpServer_builder{
							Command: "another-mcp-server",
						}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	result, err := r.Materialize(context.Background(), recipe)
	require.NoError(t, err)
	require.NotNil(t, result)

	var mcpContent string
	for _, e := range result.GetEntries() {
		if e.GetFile().GetPath() == ".claude/mcp.local.json" {
			mcpContent = e.GetFile().GetContent()
			break
		}
	}
	require.NotEmpty(t, mcpContent)

	var mcp struct {
		McpServers map[string]map[string]string `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcpContent), &mcp))
	assert.Len(t, mcp.McpServers, 3)
	assert.Equal(t, "https://example.com/mcp", mcp.McpServers["http-server"]["url"])
	assert.Equal(t, "stdio-mcp", mcp.McpServers["stdio-server"]["command"])
	assert.Equal(t, "another-mcp-server", mcp.McpServers["another-stdio"]["command"])
}
