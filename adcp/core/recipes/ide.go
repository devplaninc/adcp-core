package recipes

import (
	"context"

	"github.com/devplaninc/adcp/clients/go/adcp"
)

type IDEProvider interface {
	Materialize(ctx context.Context, ide *adcp.Ide) (*adcp.MaterializedResult, error)
}
