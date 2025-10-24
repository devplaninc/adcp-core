package shared

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devplaninc/adcp/clients/go/adcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getIDE() *IDE {
	return &IDE{
		MCPServersJSONPath: ".mcp.json",
		CommandsFolder:     ".claude/commands/",
	}
}

func TestIDE_Materialize_NilIde(t *testing.T) {
	g := getIDE()
	_, err := g.Materialize(context.Background(), nil)
	assert.Error(t, err)
}

func TestIDE_Materialize_Mcp(t *testing.T) {
	g := getIDE()

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
