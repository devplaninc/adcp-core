package prefetch

import (
	"context"
	"fmt"

	"github.com/devplaninc/adcp-core/adcp/core/utils"
	"github.com/devplaninc/adcp/clients/go/adcp"
	"google.golang.org/protobuf/encoding/protojson"
)

type Processor struct{}

func (p *Processor) Process(ctx context.Context, prefetch *adcp.Prefetch) (map[string]*adcp.FetchedData, error) {
	entries := prefetch.GetEntries()
	if len(entries) == 0 {
		return nil, nil
	}

	result := make(map[string]*adcp.FetchedData)

	for i, entry := range entries {
		if entry == nil {
			return nil, fmt.Errorf("prefetch entry at index %d is nil", i)
		}

		// Process the entry based on its type
		data, err := p.processEntry(ctx, entry)
		if err != nil {
			return nil, fmt.Errorf("failed to process entry at index %d: %w", i, err)
		}
		res := &adcp.PrefetchResult{}
		u := protojson.UnmarshalOptions{DiscardUnknown: true}
		if err := u.Unmarshal([]byte(data), res); err != nil {
			return nil, fmt.Errorf("failed to unmarshal prefetch result: %w", err)
		}
		for _, d := range res.GetData() {
			result[d.GetId()] = d
		}
	}

	return result, nil
}

func (p *Processor) processEntry(ctx context.Context, entry *adcp.PrefetchEntry) (string, error) {
	switch entry.WhichType() {
	case adcp.PrefetchEntry_Cmd_case:
		cmd := entry.GetCmd()
		if cmd == "" {
			return "", fmt.Errorf("cmd cannot be empty")
		}
		data, err := utils.ExecuteCommand(ctx, cmd)
		if err != nil {
			return "", fmt.Errorf("command execution failed: %w", err)
		}
		return data, nil

	default:
		return "", fmt.Errorf("unknown or unset prefetch entry type [%v]", entry.WhichType())
	}
}
