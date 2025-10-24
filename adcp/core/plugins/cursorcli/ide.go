package cursorcli

import (
	"context"

	"github.com/devplaninc/adcp-core/adcp/core/plugins/shared"
	"github.com/devplaninc/adcp-core/adcp/core/recipes"
	"github.com/devplaninc/adcp/clients/go/adcp"
)

func NewIDEProvider() recipes.IDEProvider {
	return &shared.IDE{
		CommandsFolder:     ".cursor/commands",
		MCPServersJSONPath: ".cursor/mcp.json",
		Settings:           &settings{},
	}
}

type settings struct {
	shared.IDESettings
}

func (s *settings) Update(_ context.Context, _ shared.SettingsInput) ([]*adcp.MaterializedResult_Entry, error) {
	return nil, nil
}
