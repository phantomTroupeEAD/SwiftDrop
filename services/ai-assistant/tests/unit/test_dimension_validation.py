"""Comprehensive unit tests for parcel dimension cross-field validation in EstimateFeeInput."""

import pytest
from pydantic import ValidationError

from src.domain.tools import EstimateFeeInput, ServiceType


def test_dimensions_omitted_valid():
    """Weight-only quote without dimensions is valid."""
    model = EstimateFeeInput(
        weight_kg=2.5,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
    )
    assert model.length_cm is None
    assert model.width_cm is None
    assert model.height_cm is None
    assert model.weight_kg == 2.5


def test_dimensions_all_specified_valid():
    """Quote with all 3 dimensions specified is valid."""
    model = EstimateFeeInput(
        weight_kg=2.5,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=20.0,
        width_cm=15.0,
        height_cm=10.0,
    )
    assert model.length_cm == 20.0
    assert model.width_cm == 15.0
    assert model.height_cm == 10.0


@pytest.mark.parametrize(
    "length,width,height,missing_desc",
    [
        (20.0, None, None, "width and height missing"),
        (None, 15.0, None, "length and height missing"),
        (None, None, 10.0, "length and width missing"),
        (20.0, 15.0, None, "height missing"),
        (20.0, None, 10.0, "width missing"),
        (None, 15.0, 10.0, "length missing"),
    ],
)
def test_partial_dimensions_rejected(length, width, height, missing_desc):
    """Any combination of 1 or 2 dimensions must raise a ValidationError."""
    with pytest.raises(ValidationError) as exc_info:
        EstimateFeeInput(
            weight_kg=2.5,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
            length_cm=length,
            width_cm=width,
            height_cm=height,
        )
    assert "All three dimensions (length_cm, width_cm, height_cm) must be provided together" in str(
        exc_info.value
    )


def test_dimensions_min_boundary_valid():
    """Minimum valid dimensions (0.1 cm) must be accepted."""
    model = EstimateFeeInput(
        weight_kg=0.1,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=0.1,
        width_cm=0.1,
        height_cm=0.1,
    )
    assert model.length_cm == 0.1
    assert model.width_cm == 0.1
    assert model.height_cm == 0.1


def test_dimensions_max_boundary_valid():
    """Maximum valid dimensions (300.0 cm) must be accepted."""
    model = EstimateFeeInput(
        weight_kg=50.0,
        pickup_postal_code="00100",
        delivery_postal_code="20000",
        length_cm=300.0,
        width_cm=300.0,
        height_cm=300.0,
    )
    assert model.length_cm == 300.0
    assert model.width_cm == 300.0
    assert model.height_cm == 300.0


@pytest.mark.parametrize(
    "invalid_dim",
    [0.0, -1.0, -0.01],
)
def test_dimensions_zero_or_negative_rejected(invalid_dim):
    """Zero or negative dimensions must be rejected by gt=0."""
    with pytest.raises(ValidationError) as exc_info:
        EstimateFeeInput(
            weight_kg=2.5,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
            length_cm=invalid_dim,
            width_cm=15.0,
            height_cm=10.0,
        )
    assert "greater than 0" in str(exc_info.value)


def test_dimensions_exceeding_max_rejected():
    """Dimensions greater than 300.0 cm must be rejected by le=300."""
    with pytest.raises(ValidationError) as exc_info:
        EstimateFeeInput(
            weight_kg=2.5,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
            length_cm=300.1,
            width_cm=15.0,
            height_cm=10.0,
        )
    assert "less than or equal to 300" in str(exc_info.value)


def test_dimensions_non_numeric_rejected():
    """Non-numeric string dimensions must be rejected."""
    with pytest.raises(ValidationError):
        EstimateFeeInput(
            weight_kg=2.5,
            pickup_postal_code="00100",
            delivery_postal_code="20000",
            length_cm="twenty",  # type: ignore[arg-type]
            width_cm=15.0,
            height_cm=10.0,
        )
