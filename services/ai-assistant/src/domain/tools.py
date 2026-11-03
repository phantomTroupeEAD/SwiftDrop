"""Pydantic input argument schemas and enums for read-only tools."""

from enum import Enum
import re
from typing import Any, Optional
from pydantic import BaseModel, Field, field_validator, model_validator

from src.core.postal_codes import resolve_postal_code


class ServiceType(str, Enum):
    """Delivery speed service levels recognized by the pricing engine."""

    STANDARD = "STANDARD"
    EXPRESS = "EXPRESS"
    SAME_DAY = "SAME_DAY"


class ParcelStatus(str, Enum):
    """Authoritative parcel lifecycle states."""

    CREATED = "CREATED"
    PICKED_UP = "PICKED_UP"
    IN_TRANSIT = "IN_TRANSIT"
    OUT_FOR_DELIVERY = "OUT_FOR_DELIVERY"
    DELIVERED = "DELIVERED"
    FAILED_ATTEMPT = "FAILED_ATTEMPT"
    RETURNED_TO_HUB = "RETURNED_TO_HUB"
    CANCELLED = "CANCELLED"


class TrackParcelInput(BaseModel):
    """Input argument model for track_parcel_status tool."""

    tracking_number: str = Field(
        description="Authoritative tracking number in SD-YYYYMMDD-XXXXX format",
    )

    @field_validator("tracking_number")
    @classmethod
    def validate_tracking_number(cls, v: str) -> str:
        """Validate tracking number against canonical format (SD-YYYYMMDD-XXXXX)."""
        cleaned = v.strip().upper()
        # Non-backtracking canonical tracking regex
        pattern = r"^SD-\d{8}-\d{5}$"
        if not re.match(pattern, cleaned):
            raise ValueError(
                f"Invalid tracking number format '{v}'. Expected format: SD-YYYYMMDD-XXXXX "
                "(e.g. SD-20261021-04921)"
            )
        return cleaned


class EstimateFeeInput(BaseModel):
    """Input argument model for estimate_shipping_fee tool."""

    weight_kg: float = Field(
        gt=0.0,
        le=50.0,
        description="Gross package weight in kilograms (must be > 0.0 and <= 50.0 kg)",
    )
    pickup_postal_code: str = Field(
        pattern=r"^\d{5}$",
        description="5-digit pickup postal code (e.g. 00100 for Colombo)",
    )
    delivery_postal_code: str = Field(
        pattern=r"^\d{5}$",
        description="5-digit destination postal code (e.g. 20000 for Kandy)",
    )

    @field_validator("pickup_postal_code", "delivery_postal_code", mode="before")
    @classmethod
    def resolve_city_input(cls, v: Any) -> str:
        """Resolve city name (e.g. 'Anuradhapura', 'Walasmulla') to 5-digit postal code (e.g. '50000', '82450')."""
        if isinstance(v, str):
            resolved = resolve_postal_code(v)
            if resolved:
                return resolved
        return str(v)

    service_type: ServiceType = Field(
        default=ServiceType.STANDARD,
        description="Delivery service level tier",
    )
    length_cm: Optional[float] = Field(
        default=None,
        gt=0.0,
        le=300.0,
        description="Optional package length in centimeters",
    )
    width_cm: Optional[float] = Field(
        default=None,
        gt=0.0,
        le=300.0,
        description="Optional package width in centimeters",
    )
    height_cm: Optional[float] = Field(
        default=None,
        gt=0.0,
        le=300.0,
        description="Optional package height in centimeters",
    )

    @model_validator(mode="after")
    def validate_complete_dimensions(self) -> "EstimateFeeInput":
        """Ensure parcel dimensions are either fully specified (all three: length, width, height) or omitted entirely."""
        dims = [self.length_cm, self.width_cm, self.height_cm]
        specified = [d for d in dims if d is not None]
        if specified and len(specified) < 3:
            raise ValueError(
                "All three dimensions (length_cm, width_cm, height_cm) must be provided together. "
                "Partial dimensions cannot be calculated."
            )
        return self


class ActiveParcelsInput(BaseModel):
    """Input argument model for get_my_active_parcels tool."""

    status: Optional[ParcelStatus] = Field(
        default=None,
        description="Optional status filter",
    )
    limit: int = Field(
        default=10,
        ge=1,
        le=50,
        description="Maximum number of active parcels to return (1-50)",
    )


class CompanyPolicyInput(BaseModel):
    """Input argument model for explain_company_policies tool."""

    query: str = Field(
        min_length=1,
        max_length=200,
        description="Policy query topic (e.g. 'prohibited goods', 'packaging rules', 'SLA')",
    )

    @field_validator("query")
    @classmethod
    def sanitize_query(cls, v: str) -> str:
        """Sanitize query string."""
        cleaned = v.replace("\x00", "").strip()
        if not cleaned:
            raise ValueError("Query string cannot be empty")
        return cleaned


class LookupPostalCodeInput(BaseModel):
    """Input argument model for lookup_postal_code tool."""

    query: str = Field(
        min_length=1,
        max_length=100,
        description="Sri Lankan city, town, suburb or area name (e.g. 'Walasmulla', 'Kadugannawa', 'Colombo 03')",
    )

    @field_validator("query")
    @classmethod
    def sanitize_location(cls, v: str) -> str:
        """Sanitize location query string."""
        cleaned = v.replace("\x00", "").strip()
        if not cleaned:
            raise ValueError("Location query cannot be empty")
        return cleaned
