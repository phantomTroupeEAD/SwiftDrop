"""Unit tests for tool input arguments, format regex, and numerical boundary conditions."""

import pytest
from pydantic import ValidationError
from src.domain.tools import (
    ActiveParcelsInput,
    CompanyPolicyInput,
    EstimateFeeInput,
    ParcelStatus,
    ServiceType,
    TrackParcelInput,
)


def test_track_parcel_input_valid():
    """Verify canonical tracking numbers are accepted and normalized to uppercase."""
    valid_numbers = [
        "SD-20261021-04921",
        "sd-20261021-04921",  # Lowercase should be uppercased
        " SD-20261021-99999 ",  # Whitespace trimmed
    ]
    for num in valid_numbers:
        inp = TrackParcelInput(tracking_number=num)
        assert inp.tracking_number == num.strip().upper()


def test_track_parcel_input_invalid():
    """Verify malformed tracking numbers fail validation."""
    invalid_numbers = [
        "INVALID-TRACKING",
        "SD-12345",
        "SD-20261021-1234",  # Only 4 trailing digits
        "SD-20261021-123456",  # 6 trailing digits
        "SD-2026102-12345",  # 7 date digits
        "20261021-04921",
        "SD_20261021_04921",  # Underscores instead of dashes
    ]
    for bad in invalid_numbers:
        with pytest.raises(ValidationError) as exc_info:
            TrackParcelInput(tracking_number=bad)
        assert "Invalid tracking number format" in str(exc_info.value)


def test_estimate_fee_input_valid():
    """Verify valid fee inputs pass validation."""
    inp = EstimateFeeInput(
        weight_kg=2.5,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        service_type=ServiceType.EXPRESS,
        length_cm=30.0,
        width_cm=20.0,
        height_cm=15.0,
    )
    assert inp.weight_kg == 2.5
    assert inp.service_type == ServiceType.EXPRESS
    assert inp.pickup_postal_code == "00100"


def test_estimate_fee_input_weight_boundaries():
    """Verify weight boundaries: 0.0 < weight_kg <= 50.0."""
    # Boundary: min valid
    assert EstimateFeeInput(
        weight_kg=0.01,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
    ).weight_kg == 0.01

    # Boundary: max valid
    assert EstimateFeeInput(
        weight_kg=50.0,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
    ).weight_kg == 50.0

    # Boundary: zero weight (violates gt=0.0)
    with pytest.raises(ValidationError):
        EstimateFeeInput(
            weight_kg=0.0,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
        )

    # Boundary: negative weight
    with pytest.raises(ValidationError):
        EstimateFeeInput(
            weight_kg=-2.5,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
        )

    # Boundary: exceeds max weight 50.0
    with pytest.raises(ValidationError):
        EstimateFeeInput(
            weight_kg=50.01,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
        )


def test_estimate_fee_input_postal_code_regex():
    """Verify postal codes must be exactly 5 digits."""
    invalid_codes = ["1234", "123456", "ABCDE", "0010A"]
    for code in invalid_codes:
        with pytest.raises(ValidationError):
            EstimateFeeInput(
                weight_kg=1.0,
                pickup_postal_code=code,
                delivery_postal_code="20000",
            )


def test_active_parcels_input_limits():
    """Verify limit pagination bounds: 1 <= limit <= 50."""
    assert ActiveParcelsInput(limit=1).limit == 1
    assert ActiveParcelsInput(limit=50).limit == 50

    with pytest.raises(ValidationError):
        ActiveParcelsInput(limit=0)

    with pytest.raises(ValidationError):
        ActiveParcelsInput(limit=51)


def test_company_policy_input():
    """Verify company policy input sanitization."""
    inp = CompanyPolicyInput(query="  prohibited hazardous items \x00 ")
    assert inp.query == "prohibited hazardous items"

    with pytest.raises(ValidationError):
        CompanyPolicyInput(query="   ")
