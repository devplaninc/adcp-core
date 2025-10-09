package generators

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/devplaninc/adcp-core/adcp/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

type Context struct{}

func (c *Context) Materialize(ctx context.Context, contextMsg *adcp.Context) (*adcp.MaterializedResult, error) {
	if contextMsg == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	entries := contextMsg.GetEntries()
	if entries == nil {
		return adcp.MaterializedResult_builder{}.Build(), nil
	}

	var resultEntries []*adcp.MaterializedResult_Entry

	for _, entry := range entries {
		materializedEntry, err := c.materializeEntry(ctx, entry)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize entry for path %s: %w", entry.GetPath(), err)
		}
		resultEntries = append(resultEntries, materializedEntry)
	}

	return adcp.MaterializedResult_builder{
		Entries: resultEntries,
	}.Build(), nil
}

func (c *Context) materializeEntry(ctx context.Context, entry *adcp.ContextEntry) (*adcp.MaterializedResult_Entry, error) {
	path := entry.GetPath()
	if path == "" {
		return nil, fmt.Errorf("entry path cannot be empty")
	}

	if !entry.HasFrom() {
		return nil, fmt.Errorf("entry must have a 'from' source")
	}

	content, err := c.fetchContent(ctx, entry.GetFrom())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	return adcp.MaterializedResult_Entry_builder{
		File: adcp.FullFileContent_builder{
			Path:    path,
			Content: content,
		}.Build(),
	}.Build(), nil
}

func (c *Context) fetchContent(ctx context.Context, from *adcp.ContextFrom) (string, error) {
	if from == nil {
		return "", fmt.Errorf("from source cannot be nil")
	}

	switch from.WhichType() {
	case adcp.ContextFrom_Text_case:
		return from.GetText(), nil

	case adcp.ContextFrom_Cmd_case:
		return c.executeCommand(ctx, from.GetCmd())

	case adcp.ContextFrom_Github_case:
		return c.fetchGithub(ctx, from.GetGithub())

	case adcp.ContextFrom_Combined_case:
		return c.fetchCombined(ctx, from.GetCombined())

	default:
		return "", fmt.Errorf("unknown or unset context source type")
	}
}

func (c *Context) executeCommand(ctx context.Context, cmd string) (string, error) {
	if cmd == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// Execute command using shell with context
	command := exec.CommandContext(ctx, "sh", "-c", cmd)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

func (c *Context) fetchGithub(ctx context.Context, ref *adcp.GitReference) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("github reference cannot be nil")
	}

	githubPath := ref.GetPath()
	if githubPath == "" {
		return "", fmt.Errorf("github path cannot be empty")
	}

	// Convert GitHub URL to raw content URL
	url, err := utils.ConvertToRawURL(githubPath, ref.GetVersion())
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch from github: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github fetch returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

func (c *Context) fetchCombined(ctx context.Context, combined *adcp.CombinedContextSource) (string, error) {
	if combined == nil {
		return "", fmt.Errorf("combined source cannot be nil")
	}

	items := combined.GetItems()
	if len(items) == 0 {
		return "", nil
	}

	var builder strings.Builder
	for i, item := range items {
		content, err := c.fetchCombinedItem(ctx, item)
		if err != nil {
			return "", fmt.Errorf("failed to fetch combined item %d: %w", i, err)
		}
		builder.WriteString(content)
	}

	return builder.String(), nil
}

func (c *Context) fetchCombinedItem(ctx context.Context, item *adcp.CombinedContextSource_Item) (string, error) {
	if item == nil {
		return "", fmt.Errorf("combined item cannot be nil")
	}

	switch item.WhichType() {
	case adcp.CombinedContextSource_Item_Text_case:
		return item.GetText(), nil

	case adcp.CombinedContextSource_Item_Cmd_case:
		return c.executeCommand(ctx, item.GetCmd())

	case adcp.CombinedContextSource_Item_Github_case:
		return c.fetchGithub(ctx, item.GetGithub())

	default:
		return "", fmt.Errorf("unknown or unset combined item type")
	}
}
