package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	utils2 "github.com/devplaninc/adcp-core/adcp/core/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

// IDE is responsible for materializing Claude Code specific IDE configuration files.
type IDE struct{}

// Materialize converts an Ide configuration into a set of materialized files for Claude Code.
// It produces:
// - .claude/commands/<name>.md files for each command
// - .claude/settings.local.json for permissions (allow/deny) including MCP server permissions
// - .mcp.json for MCP server definitions
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

	// Extract MCP server names for permissions
	var mcpServerNames []string
	if ide.HasMcp() {
		for name := range ide.GetMcp().GetServers() {
			mcpServerNames = append(mcpServerNames, name)
		}
	}

	// Permissions -> .claude/settings.local.json (including MCP server permissions)
	if ide.HasPermissions() || len(mcpServerNames) > 0 {
		perms := ide.GetPermissions()
		permEntries, err := g.materializePermissions(perms, mcpServerNames)
		if err != nil {
			return nil, err
		}
		entries = append(entries, permEntries...)
	}

	// MCP servers -> .mcp.json
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

func (g *IDE) materializePermissions(perms *adcp.Permissions, mcpServerNames []string) ([]*adcp.MaterializedResult_Entry, error) {
	var entries []*adcp.MaterializedResult_Entry

	// Read existing file content if it exists
	existingContent := ""
	settingsPath := ".claude/settings.local.json"
	if data, err := os.ReadFile(settingsPath); err == nil {
		existingContent = string(data)
	}

	settingsContent, err := buildClaudeSettingsJSON(perms, mcpServerNames, existingContent)
	if err != nil {
		return nil, err
	}
	entries = append(entries, adcp.MaterializedResult_Entry_builder{
		File: adcp.FullFileContent_builder{Path: settingsPath, Content: settingsContent}.Build(),
	}.Build())
	return entries, nil
}

func (g *IDE) materializeMcp(mcp *adcp.Mcp) ([]*adcp.MaterializedResult_Entry, error) {
	var entries []*adcp.MaterializedResult_Entry
	if mcp == nil {
		return entries, nil
	}

	// Read existing file content if it exists
	existingContent := ""
	mcpPath := ".mcp.json"
	if data, err := os.ReadFile(mcpPath); err == nil {
		existingContent = string(data)
	}

	mcpContent, err := buildClaudeMcpJSON(mcp, existingContent)
	if err != nil {
		return nil, err
	}
	entries = append(entries, adcp.MaterializedResult_Entry_builder{
		File: adcp.FullFileContent_builder{Path: mcpPath, Content: mcpContent}.Build(),
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
		return utils2.ExecuteCommand(ctx, from.GetCmd())
	case adcp.CommandFrom_Github_case:
		return utils2.FetchGithub(ctx, from.GetGithub())
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
	EnabledMcpjsonServers []string `json:"enabledMcpjsonServers,omitempty"`
}

type claudeMcp struct {
	McpServers map[string]map[string]string `json:"mcpServers"`
}

func buildClaudeSettingsJSON(perms *adcp.Permissions, mcpServerNames []string, existingContent string) (string, error) {
	var s claudeSettings

	// Parse existing content if provided
	if existingContent != "" {
		if err := json.Unmarshal([]byte(existingContent), &s); err != nil {
			// If parsing fails, start fresh but log the error
			s = claudeSettings{}
		}
	}

	// Ensure non-nil slices
	if s.Permissions.Allow == nil {
		s.Permissions.Allow = []string{}
	}
	if s.Permissions.Deny == nil {
		s.Permissions.Deny = []string{}
	}
	if s.Permissions.Ask == nil {
		s.Permissions.Ask = []string{}
	}
	if s.EnabledMcpjsonServers == nil {
		s.EnabledMcpjsonServers = []string{}
	}

	// Build new permissions from input
	var newAllow []string
	if perms != nil {
		for _, p := range perms.GetAllow() {
			if !p.HasType() {
				continue
			}
			newAllow = append(newAllow, formatPermission(p))
		}
	}

	var newDeny []string
	if perms != nil {
		for _, p := range perms.GetDeny() {
			if !p.HasType() {
				continue
			}
			newDeny = append(newDeny, formatPermission(p))
		}
	}

	// Add MCP servers to allow list as mcp__<name>
	var mcpAllowPermissions []string
	for _, serverName := range mcpServerNames {
		mcpAllowPermissions = append(mcpAllowPermissions, fmt.Sprintf("mcp__%s", serverName))
	}
	newAllow = append(newAllow, mcpAllowPermissions...)

	// Merge with existing permissions (deduplicate)
	s.Permissions.Allow = mergeUniqueStrings(s.Permissions.Allow, newAllow)
	s.Permissions.Deny = mergeUniqueStrings(s.Permissions.Deny, newDeny)

	// Add MCP server names to enabledMcpjsonServers
	s.EnabledMcpjsonServers = mergeUniqueStrings(s.EnabledMcpjsonServers, mcpServerNames)

	b, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal settings json: %w", err)
	}
	return string(b), nil
}

// mergeUniqueStrings merges two string slices, removing duplicates
func mergeUniqueStrings(existing, new []string) []string {
	seen := make(map[string]bool)
	var result []string

	// Add existing items first
	for _, s := range existing {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	// Add new items that aren't duplicates
	for _, s := range new {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

func buildClaudeMcpJSON(mcp *adcp.Mcp, existingContent string) (string, error) {
	if mcp == nil {
		return "", fmt.Errorf("mcp cannot be nil")
	}

	var cm claudeMcp

	// Parse existing content if provided
	if existingContent != "" {
		if err := json.Unmarshal([]byte(existingContent), &cm); err != nil {
			// If parsing fails, start fresh
			cm = claudeMcp{}
		}
	}

	// Ensure the map is initialized
	if cm.McpServers == nil {
		cm.McpServers = map[string]map[string]string{}
	}

	// Add or update servers from the new configuration
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
