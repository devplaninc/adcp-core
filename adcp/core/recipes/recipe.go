package recipes

import (
	"context"
	"fmt"

	"github.com/devplaninc/adcp-core/adcp/core"
	"github.com/devplaninc/adcp-core/adcp/core/generators"
	"github.com/devplaninc/adcp-core/adcp/core/prefetch"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

type Recipe struct {
	IDE IDEProvider
}

func (r *Recipe) Materialize(ctx context.Context, recipe *adcp.Recipe) (*adcp.MaterializedResult, error) {
	if recipe == nil {
		return nil, fmt.Errorf("recipe cannot be nil")
	}
	genCtx := &core.GenerationContext{}
	if pf := recipe.GetPrefetch(); pf != nil {
		p := prefetch.Processor{}
		entries, err := p.Process(ctx, pf)
		if err != nil {
			return nil, fmt.Errorf("failed to process prefetch: %w", err)
		}
		genCtx.Prefetched = entries
	}

	var resultEntries []*adcp.MaterializedResult_Entry

	// Materialize context entries if present
	if recipe.HasContext() {
		contextGen := &generators.Context{}
		contextResult, err := contextGen.Materialize(ctx, recipe.GetContext(), genCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize context: %w", err)
		}
		resultEntries = append(resultEntries, contextResult.GetEntries()...)
	}

	// Materialize IDE configuration if present
	if recipe.HasIde() {
		ideResult, err := r.IDE.Materialize(ctx, recipe.GetIde())
		if err != nil {
			return nil, fmt.Errorf("failed to materialize IDE configuration: %w", err)
		}
		resultEntries = append(resultEntries, ideResult.GetEntries()...)
	}

	return adcp.MaterializedResult_builder{
		Entries: resultEntries,
	}.Build(), nil
}
