package claude

import "github.com/devplaninc/adcp/clients/go/adcp"

type Plugin struct {
}

func (p *Plugin) GetMCP(m *adcp.Mcp) error {
	return nil
}
