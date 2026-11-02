"""Unified Mock Upstream Server simulating Core API (8080) and Tracking API (8081).

Provides realistic endpoints for local testing:
- GET  /api/v1/tracking/public/{tracking_number} (Tracking API)
- POST /api/v1/pricing/quotes                    (Core API Pricing)
- GET  /api/v1/parcels                           (Core API Parcels with Bearer auth)
- GET  /api/v1/admin/settings                    (Core API Admin Settings)
"""

from typing import Any, Dict, List, Optional
from fastapi import FastAPI, Header, HTTPException, Request, Response, status
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
import uvicorn

app = FastAPI(
    title="SwiftDrop Unified Mock Upstream Server",
    description="Simulates Member 2 Core API and Member 5 Tracking API for local AI Assistant testing.",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ------------------------------------------------------------------------------
# Mock Data Fixtures
# ------------------------------------------------------------------------------

SEEDED_PARCELS: Dict[str, Dict[str, Any]] = {
    "SD-20261102-00001": {
        "tracking_number": "SD-20261102-00001",
        "status": "IN_TRANSIT",
        "status_description": "Package is on route to Colombo sorting hub",
        "destination_city": "Colombo",
        "estimated_delivery": "2026-11-04T14:00:00Z",
        "milestones": [
            {
                "status": "BOOKED",
                "timestamp": "2026-11-02T08:00:00Z",
                "location": "Kandy Hub",
                "description": "Package dropped off and manifest created",
            },
            {
                "status": "IN_TRANSIT",
                "timestamp": "2026-11-02T13:30:00Z",
                "location": "Peradeniya Transit Facility",
                "description": "Departed sorting facility on line-haul truck",
            },
        ],
        "recipient_phone": "+94771234567",  # Intentional PII to test SEC-009 scrubber
        "driver_notes": "Security gate code #1234",
    },
    "SD-20261102-00002": {
        "tracking_number": "SD-20261102-00002",
        "status": "DELIVERED",
        "status_description": "Delivered and signed by recipient",
        "destination_city": "Galle",
        "estimated_delivery": "2026-11-01T16:45:00Z",
        "milestones": [
            {
                "status": "BOOKED",
                "timestamp": "2026-10-31T09:00:00Z",
                "location": "Colombo Central",
                "description": "Parcel booked",
            },
            {
                "status": "DELIVERED",
                "timestamp": "2026-11-01T16:45:00Z",
                "location": "Galle Hub",
                "description": "Delivered to recipient with digital signature",
            },
        ],
        "recipient_phone": "+94719876543",
        "driver_notes": "Left on front porch behind gate",
    },
    "SD-20261102-00003": {
        "tracking_number": "SD-20261102-00003",
        "status": "OUT_FOR_DELIVERY",
        "status_description": "Courier is out for final delivery",
        "destination_city": "Colombo 03",
        "estimated_delivery": "2026-11-02T18:00:00Z",
        "milestones": [
            {
                "status": "OUT_FOR_DELIVERY",
                "timestamp": "2026-11-02T08:30:00Z",
                "location": "Colombo Hub 03",
                "description": "Assigned to courier Kamal Perera",
            }
        ],
        "recipient_phone": "+94701122334",
        "driver_notes": "Call customer before arrival",
    },
}

CUSTOMER_ACTIVE_PARCELS: List[Dict[str, Any]] = [
    {
        "id": "e2a3c4b5-6789-40ab-bcde-f12345678901",
        "tracking_number": "SD-20261102-00001",
        "status": "IN_TRANSIT",
        "recipient_city": "Colombo",
        "service_type": "STANDARD",
        "weight_kg": 2.5,
        "created_at": "2026-11-02T08:00:00Z",
        "updated_at": "2026-11-02T13:30:00Z",
    },
    {
        "id": "f3b4c5d6-7890-41bc-cdef-012345678902",
        "tracking_number": "SD-20261102-00003",
        "status": "OUT_FOR_DELIVERY",
        "recipient_city": "Colombo 03",
        "service_type": "EXPRESS",
        "weight_kg": 1.2,
        "created_at": "2026-11-02T08:30:00Z",
        "updated_at": "2026-11-02T14:15:00Z",
    },
]


# ------------------------------------------------------------------------------
# Request Schemas
# ------------------------------------------------------------------------------

class QuoteDimensions(BaseModel):
    length_cm: Optional[float] = None
    width_cm: Optional[float] = None
    height_cm: Optional[float] = None


class PricingQuoteRequest(BaseModel):
    weight_kg: float = Field(ge=0.01, description="Gross parcel weight in kg")
    pickup_postal_code: str = Field(description="5-digit origin postal code")
    delivery_postal_code: str = Field(description="5-digit destination postal code")
    service_type: str = Field(default="STANDARD", description="STANDARD, EXPRESS, or SAME_DAY")
    dimensions: Optional[QuoteDimensions] = None


# ------------------------------------------------------------------------------
# Routes
# ------------------------------------------------------------------------------

@app.get("/", summary="Mock Server Greeting", tags=["Root"])
def root_info() -> Dict[str, Any]:
    """Return mock server status and available endpoints."""
    return {
        "service": "SwiftDrop Unified Mock Upstream Server",
        "status": "OPERATIONAL",
        "port": 8085,
        "endpoints": {
            "tracking": "GET /api/v1/tracking/public/{tracking_number}",
            "pricing": "POST /api/v1/pricing/quotes",
            "parcels": "GET /api/v1/parcels",
            "admin_settings": "GET /api/v1/admin/settings",
        },
        "docs_url": "/docs",
    }


# 1. Tracking API Endpoint (Member 5)
@app.get(
    "/api/v1/tracking/public/{tracking_number}",
    summary="Public Parcel Tracking",
    tags=["Tracking API (Member 5)"],
)
def get_public_tracking(tracking_number: str) -> Dict[str, Any]:
    """Retrieve parcel milestones and current status. Returns 404 if not found."""
    clean_num = tracking_number.strip().upper()
    if clean_num in SEEDED_PARCELS:
        return SEEDED_PARCELS[clean_num]

    raise HTTPException(
        status_code=status.HTTP_404_NOT_FOUND,
        detail={
            "error": {
                "code": "PARCEL_NOT_FOUND",
                "message": f"No shipment found matching tracking number '{clean_num}'.",
            }
        },
    )


# 2. Core API Pricing Engine (Member 2)
@app.post(
    "/api/v1/pricing/quotes",
    summary="Calculate Shipping Fee Quote",
    tags=["Core API - Pricing (Member 2)"],
)
def calculate_quote(quote_req: PricingQuoteRequest) -> Dict[str, Any]:
    """Calculate shipping tariffs in LKR integer cents."""
    if quote_req.weight_kg > 50.0:
        raise HTTPException(
            status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
            detail="Package weight exceeds maximum allowed boundary of 50.0kg",
        )

    # Base pricing logic (in LKR minor unit cents)
    base_fee_cents = 35000  # LKR 350.00
    weight_fee_cents = int(quote_req.weight_kg * 15000)  # LKR 150.00 per kg

    # Tier multipliers
    st_upper = quote_req.service_type.upper()
    if st_upper == "EXPRESS":
        multiplier = 1.5
        transit_hours = 24
    elif st_upper == "SAME_DAY":
        multiplier = 2.0
        transit_hours = 8
    else:
        multiplier = 1.0
        transit_hours = 48

    total_fee_cents = int((base_fee_cents + weight_fee_cents) * multiplier)

    return {
        "base_fee_cents": base_fee_cents,
        "weight_fee_cents": weight_fee_cents,
        "total_fee_cents": total_fee_cents,
        "currency": "LKR",
        "estimated_transit_hours": transit_hours,
        "service_type": st_upper,
    }


# 3. Core API Customer Parcels (Member 2)
@app.get(
    "/api/v1/parcels",
    summary="List Active Customer Parcels",
    tags=["Core API - Parcels (Member 2)"],
)
def list_active_parcels(
    authorization: Optional[str] = Header(default=None),
    limit: int = 10,
    status_filter: Optional[str] = None,
) -> List[Dict[str, Any]]:
    """Return active customer parcels. Requires valid Bearer JWT."""
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail={"error": {"code": "UNAUTHORIZED", "message": "Bearer authentication token required"}},
        )

    # Return seeded parcels limited by request param
    results = CUSTOMER_ACTIVE_PARCELS
    if status_filter:
        results = [p for p in results if p["status"] == status_filter.upper()]

    return results[:limit]


# 4. Core API Admin Settings (Member 7)
@app.get(
    "/api/v1/admin/settings",
    summary="Retrieve Dynamic Courier Rules & SLAs",
    tags=["Core API - Admin (Member 7)"],
)
def get_admin_settings(
    authorization: Optional[str] = Header(default=None),
) -> Dict[str, Any]:
    """Return dynamic logistics settings and prohibited items list."""
    if not authorization:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail={"error": {"code": "UNAUTHORIZED", "message": "Service authorization token required"}},
        )

    return {
        "settings": {
            "standard_sla_hours": 48,
            "express_sla_hours": 24,
            "same_day_sla_hours": 8,
            "max_parcel_weight_kg": 50,
            "prohibited_items": [
                "Firearms, ammunition, and military weapons",
                "Explosives, fireworks, and flammable liquids",
                "Illegal narcotics and unauthorized pharmaceuticals",
                "Corrosive acids, lithium batteries without UN certification",
                "Perishable food items without cold-chain packaging",
            ],
            "operating_hours": "Monday to Saturday, 08:00 AM - 08:00 PM",
            "support_hotline": "+94 11 234 5678",
        }
    }


# ------------------------------------------------------------------------------
# Server Entrypoint
# ------------------------------------------------------------------------------

def main():
    """Run mock server on port 8085."""
    print("=" * 70)
    print("🚀 SwiftDrop Unified Mock Upstream Server Starting on port 8085...")
    print("📍 Simulates Core API (Pricing, Parcels, Admin) & Tracking API")
    print("📖 Interactive Swagger UI at: http://127.0.0.1:8085/docs")
    print("=" * 70)
    uvicorn.run("scripts.mock_upstream_server:app", host="127.0.0.1", port=8085, reload=True)


if __name__ == "__main__":
    main()
