package executable

import (
	"context"
	"testing"

	"github.com/devplaninc/adcp/clients/go/adcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutableRecipe_Materialize_Table(t *testing.T) {
	tests := []struct {
		name        string
		ideType     string
		execRecipe  adcp.ExecutableRecipe_builder
		wantErrSub  string
		wantEntries int
	}{
		{
			name:    "happy path: claude ide type with empty recipe produces no entries",
			ideType: "claude",
			execRecipe: adcp.ExecutableRecipe_builder{
				// Provide an empty recipe so that recipes.Recipe doesn't try to use IDE
				Recipe: adcp.Recipe_builder{}.Build(),
			},
			wantEntries: 0,
		},
		{
			name:    "error: unsupported ide type",
			ideType: "unknown-ide",
			execRecipe: adcp.ExecutableRecipe_builder{
				Recipe: adcp.Recipe_builder{}.Build(),
			},
			wantErrSub: "failed to get IDE",
		},
		{
			name:    "error: underlying recipes.Materialize rejects nil Recipe",
			ideType: "claude",
			// leave Recipe unset so GetRecipe() returns nil
			execRecipe: adcp.ExecutableRecipe_builder{},
			wantErrSub: "recipe cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build executable recipe with given entry point ide type and provided recipe
			exec := tt.execRecipe
			exec.EntryPoint = adcp.EntryPoint_builder{IdeType: tt.ideType}.Build()
			re := ForRecipe(exec.Build())

			res, err := re.Materialize(context.Background())
			if tt.wantErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSub)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, res)
			if tt.wantEntries >= 0 {
				assert.Len(t, res.GetEntries(), tt.wantEntries)
			}
		})
	}
}
