"""Unit tests verifying SEC-009 privacy masking and destination_city extraction in tracking_tool."""

import pytest
from src.tools.tracking_tool import mask_tracking_data


def test_mask_tracking_data_flat_payload_extracts_city():
    """Flat upstream payload with destination_city must extract city accurately."""
    raw = {
        "tracking_number": "SD-20261102-00001",
        "status": "IN_TRANSIT",
        "destination_city": "Kandy",
        "destination": None,
        "recipient_phone": "+94771234567",
        "driver_notes": "Handle with care",
    }
    result = mask_tracking_data(raw, "SD-20261102-00001")
    assert result["found"] is True
    assert result["destination_city"] == "Kandy"
    assert "recipient_phone" not in result
    assert "driver_notes" not in result


def test_mask_tracking_data_nested_payload_extracts_city():
    """Nested upstream payload with destination dict must extract city accurately."""
    raw = {
        "tracking_number": "SD-20261102-00002",
        "status": "OUT_FOR_DELIVERY",
        "destination": {"city": "Galle", "postal_code": "80000"},
    }
    result = mask_tracking_data(raw, "SD-20261102-00002")
    assert result["destination_city"] == "Galle"


def test_mask_tracking_data_nested_precedence_over_flat():
    """When both are present, valid nested city takes precedence."""
    raw = {
        "tracking_number": "SD-20261102-00003",
        "destination": {"city": "Matara"},
        "destination_city": "FallbackCity",
    }
    result = mask_tracking_data(raw, "SD-20261102-00003")
    assert result["destination_city"] == "Matara"


def test_mask_tracking_data_empty_dict_destination_falls_back_to_flat():
    """When destination is an empty dict, fall back to top-level destination_city."""
    raw = {
        "tracking_number": "SD-20261102-00004",
        "destination": {},
        "destination_city": "Jaffna",
    }
    result = mask_tracking_data(raw, "SD-20261102-00004")
    assert result["destination_city"] == "Jaffna"


def test_mask_tracking_data_none_city_in_dict_falls_back_to_flat():
    """When destination dict has city=None, fall back to top-level destination_city."""
    raw = {
        "tracking_number": "SD-20261102-00005",
        "destination": {"city": None},
        "destination_city": "Kurunegala",
    }
    result = mask_tracking_data(raw, "SD-20261102-00005")
    assert result["destination_city"] == "Kurunegala"


def test_mask_tracking_data_missing_city_defaults_to_unknown():
    """When no city information exists anywhere, default to 'Unknown'."""
    raw = {
        "tracking_number": "SD-20261102-00006",
        "status": "CREATED",
    }
    result = mask_tracking_data(raw, "SD-20261102-00006")
    assert result["destination_city"] == "Unknown"
