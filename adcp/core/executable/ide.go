package executable

import (
	"fmt"
	"strings"

	"github.com/devplaninc/adcp-core/adcp/core/plugins/claude"
	"github.com/devplaninc/adcp-core/adcp/core/plugins/cursorcli"
	"github.com/devplaninc/adcp-core/adcp/core/recipes"
)

func getIDE(ideType string) (recipes.IDEProvider, error) {
	switch strings.ToLower(ideType) {
	case "claude":
		return claude.NewIDEProvider(), nil
	case "cursor-cli":
		return cursorcli.NewIDEProvider(), nil
	}
	return nil, fmt.Errorf("unsupported IDE type: %v", ideType)
}
