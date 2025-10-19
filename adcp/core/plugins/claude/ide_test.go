package claude

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

func TestIDE_Materialize_NilIde(t *testing.T) {
	g := &IDE{}
	_, err := g.Materialize(context.Background(), nil)
	assert.Error(t, err)
}

func TestIDE_Materialize_Commands_TextAndCmd(t *testing.T) {
	g := &IDE{}

	cmdOutput := "hello from cmd\n"
	ide := adcp.Ide_builder{
		Commands: adcp.Commands_builder{
			Entries: []*adcp.Command{
				adcp.Command_builder{
					Name: "refine",
					From: adcp.CommandFrom_builder{Text: strPtr("some text content")}.Build(),
				}.Build(),
				adcp.Command_builder{
					Name: "run",
					From: adcp.CommandFrom_builder{Cmd: strPtr("echo 'hello from cmd'")}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()

	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify contents
	m := map[string]string{}
	for _, e := range res.GetEntries() {
		m[e.GetFile().GetPath()] = e.GetFile().GetContent()
	}
	assert.Equal(t, "some text content", m[".claude/commands/refine.md"])
	assert.Equal(t, cmdOutput, m[".claude/commands/run.md"])
}

func TestIDE_Materialize_Command_Github(t *testing.T) {
	g := &IDE{}

	// Mock HTTP server to serve content
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("from github"))
	}))
	defer ts.Close()

	ide := adcp.Ide_builder{
		Commands: adcp.Commands_builder{
			Entries: []*adcp.Command{
				adcp.Command_builder{
					Name: "gh",
					From: adcp.CommandFrom_builder{Github: adcp.GitReference_builder{Path: ts.URL}.Build()}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()

	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)
	// Find the command entry for gh
	var foundPath string
	var foundContent string
	for _, e := range res.GetEntries() {
		if e.GetFile().GetPath() == ".claude/commands/gh.md" {
			foundPath = e.GetFile().GetPath()
			foundContent = e.GetFile().GetContent()
			break
		}
	}
	require.Equal(t, ".claude/commands/gh.md", foundPath)
	assert.Equal(t, "from github", foundContent)
}

func TestIDE_Materialize_Permissions(t *testing.T) {
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

	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Find settings file
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
	assert.Contains(t, parsed.Permissions.Allow, "Bash(go test:*)")
	assert.Contains(t, parsed.Permissions.Allow, "Read(~/.zshrc)")
	assert.Contains(t, parsed.Permissions.Deny, "Write(**/secrets/**)")
}

func TestIDE_Materialize_Mcp(t *testing.T) {
	g := &IDE{}

	ide := adcp.Ide_builder{
		Mcp: adcp.Mcp_builder{Servers: map[string]*adcp.McpServer{
			"github":  adcp.McpServer_builder{Http: adcp.HttpMcpServer_builder{Url: "https://api.githubcopilot.com/mcp/"}.Build()}.Build(),
			"devplan": adcp.McpServer_builder{Stdio: adcp.StdioMcpServer_builder{Command: "devplan mcp"}.Build()}.Build(),
		}}.Build(),
	}.Build()

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
		McpServers map[string]struct {
			Type    string            `json:"type"`
			Command string            `json:"command,omitempty"`
			Args    []string          `json:"args,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
			Url     string            `json:"url,omitempty"`
		} `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcpContent), &parsed))

	require.Contains(t, parsed.McpServers, "github")
	require.Contains(t, parsed.McpServers, "devplan")
	assert.Equal(t, "http", parsed.McpServers["github"].Type)
	assert.Equal(t, "https://api.githubcopilot.com/mcp/", parsed.McpServers["github"].Url)
	assert.Equal(t, "stdio", parsed.McpServers["devplan"].Type)
	assert.Equal(t, "devplan", parsed.McpServers["devplan"].Command)
	assert.Equal(t, []string{"mcp"}, parsed.McpServers["devplan"].Args)
}

func TestIDE_Materialize_Commands_AddedToAllow(t *testing.T) {
	g := &IDE{}

	ide := adcp.Ide_builder{
		Commands: adcp.Commands_builder{Entries: []*adcp.Command{
			adcp.Command_builder{Name: "refine", From: adcp.CommandFrom_builder{Text: strPtr("content1")}.Build()}.Build(),
			adcp.Command_builder{Name: "run", From: adcp.CommandFrom_builder{Text: strPtr("content2")}.Build()}.Build(),
		}}.Build(),
	}.Build()

	res, err := g.Materialize(context.Background(), ide)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Find settings file
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
		} `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal([]byte(settingsContent), &parsed))
	assert.Contains(t, parsed.Permissions.Allow, "SlashCommand(/refine)")
	assert.Contains(t, parsed.Permissions.Allow, "SlashCommand(/run)")
}

func strPtr(s string) *string {
	return &s
}
