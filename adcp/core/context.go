package core

import "github.com/devplaninc/adcp/clients/go/adcp"

type GenerationContext struct {
	Prefetched map[string]*adcp.FetchedData
}

func (g *GenerationContext) GetPrefetched() map[string]*adcp.FetchedData {
	if g == nil {
		return nil
	}
	return g.Prefetched
}
