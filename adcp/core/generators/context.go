package generators

import (
	"context"
	"fmt"
	"strings"

	"github.com/devplaninc/adcp-core/adcp/core"
	utils2 "github.com/devplaninc/adcp-core/adcp/core/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

type Context struct{}

func (c *Context) Materialize(ctx context.Context, contextMsg *adcp.Context, genCtx *core.GenerationContext) (*adcp.MaterializedResult, error) {
	if contextMsg == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	entries := contextMsg.GetEntries()
	if entries == nil {
		return adcp.MaterializedResult_builder{}.Build(), nil
	}

	var resultEntries []*adcp.MaterializedResult_Entry

	for _, entry := range entries {
		materializedEntry, err := c.materializeEntry(ctx, entry, genCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize entry for path %s: %w", entry.GetPath(), err)
		}
		resultEntries = append(resultEntries, materializedEntry)
	}

	return adcp.MaterializedResult_builder{
		Entries: resultEntries,
	}.Build(), nil
}

func (c *Context) materializeEntry(ctx context.Context, entry *adcp.ContextEntry, genCtx *core.GenerationContext) (*adcp.MaterializedResult_Entry, error) {
	path := entry.GetPath()
	if path == "" {
		return nil, fmt.Errorf("entry path cannot be empty")
	}

	if !entry.HasFrom() {
		return nil, fmt.Errorf("entry must have a 'from' source")
	}

	content, err := c.fetchContent(ctx, entry.GetFrom(), genCtx)
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

func (c *Context) fetchContent(ctx context.Context, from *adcp.ContextFrom, genCtx *core.GenerationContext) (string, error) {
	if from == nil {
		return "", fmt.Errorf("from source cannot be nil")
	}

	switch from.WhichType() {
	case adcp.ContextFrom_Text_case:
		return from.GetText(), nil

	case adcp.ContextFrom_Cmd_case:
		return utils2.ExecuteCommand(ctx, from.GetCmd())

	case adcp.ContextFrom_Github_case:
		return utils2.FetchGithub(ctx, from.GetGithub())

	case adcp.ContextFrom_Combined_case:
		return c.fetchCombined(ctx, from.GetCombined(), genCtx)

	case adcp.ContextFrom_PrefetchId_case:
		data, ok := genCtx.GetPrefetched()[from.GetPrefetchId()]
		if !ok {
			return "", fmt.Errorf("prefetch id [%v] not found", from.GetPrefetchId())
		}
		return data.GetData(), nil

	default:
		return "", fmt.Errorf("unknown or unset context source type")
	}
}

func (c *Context) fetchCombined(ctx context.Context, combined *adcp.CombinedContextSource, genCtx *core.GenerationContext) (string, error) {
	if combined == nil {
		return "", fmt.Errorf("combined source cannot be nil")
	}

	items := combined.GetItems()
	if len(items) == 0 {
		return "", nil
	}

	var builder strings.Builder
	for i, item := range items {
		content, err := c.fetchCombinedItem(ctx, item, genCtx)
		if err != nil {
			return "", fmt.Errorf("failed to fetch combined item %d: %w", i, err)
		}
		builder.WriteString(content)
	}

	return builder.String(), nil
}

func (c *Context) fetchCombinedItem(ctx context.Context, item *adcp.CombinedContextSource_Item, genCtx *core.GenerationContext) (string, error) {
	if item == nil {
		return "", fmt.Errorf("combined item cannot be nil")
	}

	switch item.WhichType() {
	case adcp.CombinedContextSource_Item_Text_case:
		return item.GetText(), nil

	case adcp.CombinedContextSource_Item_Cmd_case:
		return utils2.ExecuteCommand(ctx, item.GetCmd())

	case adcp.CombinedContextSource_Item_Github_case:
		return utils2.FetchGithub(ctx, item.GetGithub())

	case adcp.CombinedContextSource_Item_PrefetchId_case:
		data, ok := genCtx.GetPrefetched()[item.GetPrefetchId()]
		if !ok {
			return "", fmt.Errorf("prefetch id [%v] not found", item.GetPrefetchId())
		}
		return data.GetData(), nil

	default:
		return "", fmt.Errorf("unknown or unset combined item type")
	}
}
