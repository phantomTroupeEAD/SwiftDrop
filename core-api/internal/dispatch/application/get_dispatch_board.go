package application

import (
	"context"
	"strings"

	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// GetDispatchBoardQueryHandler aggregates unassigned parcel queues and active courier workloads for a delivery zone.
type GetDispatchBoardQueryHandler struct {
	boardReader DispatchBoardReader
}

// NewGetDispatchBoardQueryHandler constructs the query handler.
func NewGetDispatchBoardQueryHandler(boardReader DispatchBoardReader) *GetDispatchBoardQueryHandler {
	return &GetDispatchBoardQueryHandler{boardReader: boardReader}
}

// Execute retrieves unassigned parcels and courier workloads for the target zone.
func (h *GetDispatchBoardQueryHandler) Execute(ctx context.Context, zoneID string) (*DispatchBoardDTO, error) {
	trimmedZone := strings.TrimSpace(zoneID)
	if trimmedZone == "" {
		return nil, ErrInvalidZoneID
	}

	unassigned, err := h.boardReader.GetUnassignedParcels(ctx, trimmedZone)
	if err != nil {
		return nil, err
	}
	if unassigned == nil {
		unassigned = []UnassignedParcelDTO{}
	}

	couriers, err := h.boardReader.GetCourierLoads(ctx, trimmedZone)
	if err != nil {
		return nil, err
	}
	if couriers == nil {
		couriers = []CourierLoadDTO{}
	}

	return &DispatchBoardDTO{
		ZoneID:            trimmedZone,
		UnassignedParcels: unassigned,
		ActiveCouriers:    couriers,
	}, nil
}

// ListCouriersQueryHandler lists couriers with optional zone, status, and vehicle filters.
type ListCouriersQueryHandler struct {
	courierRepo CourierRepository
}

// NewListCouriersQueryHandler constructs the query handler.
func NewListCouriersQueryHandler(courierRepo CourierRepository) *ListCouriersQueryHandler {
	return &ListCouriersQueryHandler{courierRepo: courierRepo}
}

// Execute retrieves matching courier profiles.
func (h *ListCouriersQueryHandler) Execute(
	ctx context.Context,
	zoneID string,
	status *domain.ShiftStatus,
	vehicleType *domain.VehicleType,
) ([]*domain.Courier, error) {
	return h.courierRepo.List(ctx, zoneID, status, vehicleType)
}

