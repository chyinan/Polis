// pattern: Imperative Shell
package control

import (
	"context"
	"encoding/json"

	"polis/internal/mcptransport"
)

type streamableHTTPMCPRuntimeObserver struct{}

func NewStreamableHTTPMCPRuntimeObserver() StreamableHTTPMCPRuntimeObserver {
	return streamableHTTPMCPRuntimeObserver{}
}

func (streamableHTTPMCPRuntimeObserver) Observe(ctx context.Context, endpoint string) (json.RawMessage, error) {
	client, err := mcptransport.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()
	return client.ListTools(ctx)
}
