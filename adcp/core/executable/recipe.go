package executable

import (
	"context"
	"fmt"

	"github.com/devplaninc/adcp-core/adcp/core/recipes"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

func ForRecipe(recipe *adcp.ExecutableRecipe) *Recipe {
	return &Recipe{recipe}
}

type Recipe struct {
	recipe *adcp.ExecutableRecipe
}

func (r *Recipe) Materialize(ctx context.Context) (*adcp.MaterializedResult, error) {
	ideType := r.recipe.GetEntryPoint().GetIdeType()
	ide, err := getIDE(ideType)
	if err != nil {
		return nil, fmt.Errorf("failed to get IDE: %w", err)
	}
	rec := &recipes.Recipe{IDE: ide}
	return rec.Materialize(ctx, r.recipe.GetRecipe())
}
