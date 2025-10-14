package recipes

import (
	"context"
	"fmt"

	"github.com/devplaninc/adcp-core/adcp/generators"
	"github.com/devplaninc/adcp-core/adcp/plugins/claude"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

type Recipe struct{}

func (r *Recipe) Materialize(ctx context.Context, recipe *adcp.Recipe) (*adcp.MaterializedResult, error) {
	if recipe == nil {
		return nil, fmt.Errorf("recipe cannot be nil")
	}

	var resultEntries []*adcp.MaterializedResult_Entry

	// Materialize context entries if present
	if recipe.HasContext() {
		contextGen := &generators.Context{}
		contextResult, err := contextGen.Materialize(ctx, recipe.GetContext())
		if err != nil {
			return nil, fmt.Errorf("failed to materialize context: %w", err)
		}
		resultEntries = append(resultEntries, contextResult.GetEntries()...)
	}

	// Materialize IDE configuration if present
	if recipe.HasIde() {
		ideGen := &claude.IDE{}
		ideResult, err := ideGen.Materialize(ctx, recipe.GetIde())
		if err != nil {
			return nil, fmt.Errorf("failed to materialize IDE configuration: %w", err)
		}
		resultEntries = append(resultEntries, ideResult.GetEntries()...)
	}

	return adcp.MaterializedResult_builder{
		Entries: resultEntries,
	}.Build(), nil
}
