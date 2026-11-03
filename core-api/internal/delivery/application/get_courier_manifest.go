package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// GetCourierManifestQueryHandler retrieves the personalized daily delivery manifest for an authenticated courier.
// Strict Data Minimization: Redacts wholesale pricing, merchant API credentials, and unrelated shipments.
type GetCourierManifestQueryHandler struct {
	manifestReader CourierManifestReader
}

// NewGetCourierManifestQueryHandler constructs the manifest query handler.
func NewGetCourierManifestQueryHandler(manifestReader CourierManifestReader) *GetCourierManifestQueryHandler {
	return &GetCourierManifestQueryHandler{manifestReader: manifestReader}
}

// Execute retrieves active assigned deliveries for the caller's courier ID.
func (h *GetCourierManifestQueryHandler) Execute(ctx context.Context, courierID string) ([]CourierManifestItemDTO, error) {
	courierUUID, err := uuid.Parse(courierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	manifest, err := h.manifestReader.GetActiveManifest(ctx, courierUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve courier manifest: %w", err)
	}
	if manifest == nil {
		manifest = []CourierManifestItemDTO{}
	}

	return manifest, nil
}

