"""Authoritative Sri Lankan City, Town and Suburb to 5-digit Postal Code directory.

Covers major cities, commercial towns, postal divisions, and regional sub-post offices
across all 25 administrative districts in Sri Lanka.
Data is decoupled into co-located JSON asset (Separation of Concerns).
"""

import json
from pathlib import Path
from typing import Any, Dict, List, Optional

_DATA_FILE = Path(__file__).parent / "data" / "postal_directory.json"

if _DATA_FILE.exists():
    with open(_DATA_FILE, "r", encoding="utf-8") as _f:
        SRI_LANKA_POSTAL_DIRECTORY: List[Dict[str, str]] = json.load(_f)
else:
    SRI_LANKA_POSTAL_DIRECTORY = []

# Quick key-value mapping for fast O(1) exact lookups
SRI_LANKA_CITY_POSTAL_CODES: Dict[str, str] = {
    entry["alias"]: entry["code"] for entry in SRI_LANKA_POSTAL_DIRECTORY
}
# Also index by lowercase full city name
for entry in SRI_LANKA_POSTAL_DIRECTORY:
    SRI_LANKA_CITY_POSTAL_CODES[entry["city"].lower()] = entry["code"]


def lookup_postal_record(query: str) -> Optional[Dict[str, str]]:
    """Search the authoritative Sri Lankan postal directory by town, city, suburb or code.

    Matches exact name, alias, or substring. Returns complete record with district and province.
    """
    if not query or not isinstance(query, str):
        return None

    cleaned = query.strip().lower()

    # 1. Exact numeric code match
    if cleaned.isdigit() and len(cleaned) == 5:
        for entry in SRI_LANKA_POSTAL_DIRECTORY:
            if entry["code"] == cleaned:
                return dict(entry)
        return {"city": f"Postal Zone {cleaned}", "code": cleaned, "district": "Sri Lanka", "province": "National"}

    # 2. Exact alias or city match
    if cleaned in SRI_LANKA_CITY_POSTAL_CODES:
        code = SRI_LANKA_CITY_POSTAL_CODES[cleaned]
        for entry in SRI_LANKA_POSTAL_DIRECTORY:
            if entry["code"] == code:
                return dict(entry)

    # 3. Substring matching (e.g. "walasmulla town" -> "walasmulla", "kadugannawa junction" -> "kadugannawa")
    for entry in SRI_LANKA_POSTAL_DIRECTORY:
        alias = entry["alias"]
        city_lower = entry["city"].lower()
        if alias in cleaned or city_lower in cleaned or (len(cleaned) >= 4 and cleaned in alias):
            return dict(entry)

    return None


def resolve_postal_code(input_val: str) -> Optional[str]:
    """Resolve a 5-digit postal code or town name (e.g. 'Walasmulla', 'Kadugannawa') to 5-digit code."""
    record = lookup_postal_record(input_val)
    if record:
        return record.get("code")
    return None
