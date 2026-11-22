package application

import (
	"context"
)

// GetCourierCandidatesQueryHandler queries users with COURIER role who do not yet have an active fleet profile.
type GetCourierCandidatesQueryHandler struct {
	candidateReader CourierCandidateReader
}

// NewGetCourierCandidatesQueryHandler constructs the query handler.
func NewGetCourierCandidatesQueryHandler(candidateReader CourierCandidateReader) *GetCourierCandidatesQueryHandler {
	return &GetCourierCandidatesQueryHandler{candidateReader: candidateReader}
}

// Execute retrieves eligible courier candidates.
func (h *GetCourierCandidatesQueryHandler) Execute(ctx context.Context) ([]CourierCandidateDTO, error) {
	candidates, err := h.candidateReader.ListCandidates(ctx)
	if err != nil {
		return nil, err
	}
	if candidates == nil {
		candidates = []CourierCandidateDTO{}
	}
	return candidates, nil
}
