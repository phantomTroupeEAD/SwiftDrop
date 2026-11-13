package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/dispatch/domain"
)

// ToggleShiftUseCase allows couriers to toggle their shift status or dispatchers to set administrative holds.
type ToggleShiftUseCase struct {
	courierRepo CourierRepository
}

// NewToggleShiftUseCase constructs the shift toggling use case.
func NewToggleShiftUseCase(courierRepo CourierRepository) *ToggleShiftUseCase {
	return &ToggleShiftUseCase{courierRepo: courierRepo}
}

// Execute changes the courier's shift status following domain state machine transition rules.
func (uc *ToggleShiftUseCase) Execute(ctx context.Context, cmd ToggleShiftCommand) (*domain.Courier, error) {
	courierUUID, err := uuid.Parse(cmd.CourierID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid courier_id format", ErrInvalidCommand)
	}

	courier, err := uc.courierRepo.GetByID(ctx, courierUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch courier: %w", err)
	}
	if courier == nil {
		return nil, ErrCourierNotFound
	}

	if err := courier.ToggleShift(cmd.TargetStatus, cmd.IsSelfService); err != nil {
		return nil, err
	}

	if err := uc.courierRepo.Save(ctx, nil, courier); err != nil {
		return nil, fmt.Errorf("failed to save courier shift: %w", err)
	}

	return courier, nil
}

// RegisterCourierUseCase handles onboarding of operational courier fleet profiles by dispatchers/admins.
type RegisterCourierUseCase struct {
	courierRepo CourierRepository
	roleChecker UserRoleChecker
}

// NewRegisterCourierUseCase constructs the registration use case.
func NewRegisterCourierUseCase(courierRepo CourierRepository, roleChecker UserRoleChecker) *RegisterCourierUseCase {
	return &RegisterCourierUseCase{
		courierRepo: courierRepo,
		roleChecker: roleChecker,
	}
}

// Execute validates and saves a new courier fleet profile.
func (uc *RegisterCourierUseCase) Execute(ctx context.Context, cmd RegisterCourierCommand) (*domain.Courier, error) {
	userUUID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid user_id format", ErrInvalidCommand)
	}

	// 1. Verify user exists and has COURIER role in identity store
	if uc.roleChecker != nil {
		role, exists, err := uc.roleChecker.GetUserRole(ctx, userUUID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrUserNotFound
		}
		if role != "COURIER" {
			return nil, ErrUserNotCourier
		}
	}

	// 2. Invariant BR-FLT-001: Unique operational profile per courier
	existing, err := uc.courierRepo.GetByID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing courier profile: %w", err)
	}
	if existing != nil {
		return nil, ErrCourierProfileExists
	}

	courier, err := domain.NewCourier(
		userUUID,
		cmd.EmployeeCode,
		cmd.VehicleType,
		cmd.MaxCapacity,
		cmd.CurrentZoneID,
	)
	if err != nil {
		return nil, err
	}

	if err := uc.courierRepo.Save(ctx, nil, courier); err != nil {
		return nil, fmt.Errorf("failed to save new courier: %w", err)
	}

	return courier, nil
}
