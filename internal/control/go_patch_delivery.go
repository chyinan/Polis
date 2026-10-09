// pattern: Imperative Shell
package control

import (
	"context"
	"polis/internal/kernel"
)

type PublishGoPatchRequest struct {
	JobRunID  string `json:"jobRunId"`
	RequestID string `json:"requestId"`
}
type GoPatchDeliveryService interface {
	PublishGoPatch(context.Context, string, string, PublishGoPatchRequest) (kernel.ProductDeliveryResult, error)
}

func (s *Service) PublishGoPatch(ctx context.Context, company, task string, in PublishGoPatchRequest) (kernel.ProductDeliveryResult, error) {
	return s.runtime.TXPublishGoPatchDelivery(ctx, company, kernel.GoPatchDeliveryInput{TaskID: task, JobRunID: in.JobRunID, RequestID: in.RequestID})
}
