package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devplaninc/adcp-core/adcp/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

// IDE is responsible for materializing Claude Code specific IDE configuration files.
type IDE struct{}

// Materialize converts an Ide configuration into a set of materialized files for Claude Code.
// It produces:
// - .claude/commands/<name>.md files for each command
// - .claude/settings.local.json for permissions (allow/deny)
// - .claude/mcp.local.json for MCP server definitions
func (g *IDE) Materialize(ctx context.Context, ide *adcp.Ide) (*adcp.MaterializedResult, error) {
	if ide == nil {
		return nil, fmt.Errorf("ide cannot be nil")
	}

	var entries []*adcp.MaterializedResult_Entry

	// Commands -> .claude/commands/<name>.md
	if ide.HasCommands() {
		cmdEntries, err := g.materializeCommands(ctx, ide.GetCommands())
		if err != nil {
			return nil, err
		}
		entries = append(entries, cmdEntries...)
	}

	// Permissions -> .claude/settings.local.json
	if ide.HasPermissions() {
		permEntries, err := g.materializePermissions(ide.GetPermissions())
		if err != nil {
			return nil, err
		}
		entries = append(entries, permEntries...)
	}

	// MCP servers -> .claude/mcp.local.json
	if ide.HasMcp() {
		mcpEntries, err := g.materializeMcp(ide.GetMcp())
		if err != nil {
			return nil, err
		}
		entries = append(entries, mcpEntries...)
	}

	return adcp.MaterializedResult_builder{Entries: entries}.Build(), nil
}

func (g *IDE) materializeCommands(ctx context.Context, commands *adcp.Commands) ([]*adcp.MaterializedResult_Entry, error) {
	var entries []*adcp.MaterializedResult_Entry
	if commands == nil {
		return entries, nil
	}
	cmds := commands.GetEntries()
	for _, c := range cmds {
		name := c.GetName()
		if name == "" {
			return nil, fmt.Errorf("command name cannot be empty")
		}
		if !c.HasFrom() {
			return nil, fmt.Errorf("command %s must have a 'from' source", name)
		}

		content, err := g.fetchCommandContent(ctx, c.GetFrom())
		if err != nil {
			return nil, fmt.Errorf("failed to materialize command %s: %w", name, err)
		}

		path := fmt.Sprintf(".claude/commands/%s.md", name)
		entries = append(entries, adcp.MaterializedResult_Entry_builder{
			File: adcp.FullFileContent_builder{Path: path, Content: content}.Build(),
		}.Build())
	}
	return entries, nil
}

func (g *IDE) materializePermissions(perms *adcp.Permissions) ([]*adcp.MaterializedResult_Entry, error) {
	var entries []*adcp.MaterializedResult_Entry
	if perms == nil {
		return entries, nil
	}

	settingsContent, err := buildClaudeSettingsJSON(perms)
	if err != nil {
		return nil, err
	}
	entries = append(entries, adcp.MaterializedResult_Entry_builder{
		File: adcp.FullFileContent_builder{Path: ".claude/settings.local.json", Content: settingsContent}.Build(),
	}.Build())
	return entries, nil
}

func (g *IDE) materializeMcp(mcp *adcp.Mcp) ([]*adcp.MaterializedResult_Entry, error) {
	var entries []*adcp.MaterializedResult_Entry
	if mcp == nil {
		return entries, nil
	}

	mcpContent, err := buildClaudeMcpJSON(mcp)
	if err != nil {
		return nil, err
	}
	entries = append(entries, adcp.MaterializedResult_Entry_builder{
		File: adcp.FullFileContent_builder{Path: ".claude/mcp.local.json", Content: mcpContent}.Build(),
	}.Build())
	return entries, nil
}

func (g *IDE) fetchCommandContent(ctx context.Context, from *adcp.CommandFrom) (string, error) {
	if from == nil || !from.HasType() {
		return "", fmt.Errorf("command 'from' source cannot be nil")
	}

	switch from.WhichType() {
	case adcp.CommandFrom_Text_case:
		return from.GetText(), nil
	case adcp.CommandFrom_Cmd_case:
		return utils.ExecuteCommand(ctx, from.GetCmd())
	case adcp.CommandFrom_Github_case:
		return utils.FetchGithub(ctx, from.GetGithub())
	default:
		return "", fmt.Errorf("unknown or unset command source type")
	}
}

// JSON models for Claude configuration files

type claudeSettings struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
		Ask   []string `json:"ask"`
	} `json:"permissions"`
}

type claudeMcp struct {
	McpServers map[string]map[string]string `json:"mcpServers"`
}

func buildClaudeSettingsJSON(perms *adcp.Permissions) (string, error) {
	if perms == nil {
		return "", fmt.Errorf("permissions cannot be nil")
	}

	var s claudeSettings
	// ensure non-nil slices
	s.Permissions.Allow = []string{}
	s.Permissions.Deny = []string{}
	s.Permissions.Ask = []string{}

	for _, p := range perms.GetAllow() {
		if p == nil || !p.HasType() {
			continue
		}
		s.Permissions.Allow = append(s.Permissions.Allow, formatPermission(p))
	}
	for _, p := range perms.GetDeny() {
		if p == nil || !p.HasType() {
			continue
		}
		s.Permissions.Deny = append(s.Permissions.Deny, formatPermission(p))
	}

	b, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal settings json: %w", err)
	}
	return string(b), nil
}

func buildClaudeMcpJSON(mcp *adcp.Mcp) (string, error) {
	if mcp == nil {
		return "", fmt.Errorf("mcp cannot be nil")
	}

	cm := claudeMcp{McpServers: map[string]map[string]string{}}
	for name, s := range mcp.GetServers() {
		if s == nil || !s.HasType() {
			continue
		}
		srv := map[string]string{}
		switch s.WhichType() {
		case adcp.McpServer_Http_case:
			if s.GetHttp() != nil {
				srv["url"] = s.GetHttp().GetUrl()
			}
		case adcp.McpServer_Stdio_case:
			if s.GetStdio() != nil {
				srv["command"] = s.GetStdio().GetCommand()
			}
		}
		if len(srv) > 0 {
			cm.McpServers[name] = srv
		}
	}

	b, err := json.MarshalIndent(&cm, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal mcp json: %w", err)
	}
	return string(b), nil
}

func formatPermission(p *adcp.OperationPermission) string {
	switch p.WhichType() {
	case adcp.OperationPermission_Bash_case:
		return fmt.Sprintf("Bash(%s)", p.GetBash())
	case adcp.OperationPermission_Read_case:
		return fmt.Sprintf("Read(%s)", p.GetRead())
	case adcp.OperationPermission_Write_case:
		return fmt.Sprintf("Write(%s)", p.GetWrite())
	default:
		return ""
	}
}
