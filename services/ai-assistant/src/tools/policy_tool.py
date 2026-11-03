"""Company policy knowledge base tool with dynamic admin settings and static markdown fallback."""

import asyncio
from datetime import datetime, timezone
import json
from pathlib import Path
import re
from typing import Any, Dict, Optional
import httpx

from src.core.config import get_settings
from src.domain.tools import CompanyPolicyInput
from src.tools.registry import register_tool

# In-memory policy cache: {"data": ..., "timestamp": float}
_POLICY_CACHE: Dict[str, Any] = {"data": None, "timestamp": 0.0}
_CACHE_TTL_SECONDS = 1800.0  # 30 minutes
_POLICY_LOCK: Optional[asyncio.Lock] = None
_POLICY_LOCK_LOOP: Optional[asyncio.AbstractEventLoop] = None


def get_policy_lock() -> asyncio.Lock:
    """Return the policy lock for the active asyncio event loop."""
    global _POLICY_LOCK, _POLICY_LOCK_LOOP
    loop = asyncio.get_running_loop()
    if _POLICY_LOCK is None or _POLICY_LOCK_LOOP is not loop:
        _POLICY_LOCK = asyncio.Lock()
        _POLICY_LOCK_LOOP = loop
    return _POLICY_LOCK


def reset_policy_cache() -> None:
    """Reset the policy cache for isolated testing."""
    global _POLICY_LOCK, _POLICY_LOCK_LOOP
    _POLICY_CACHE["data"] = None
    _POLICY_CACHE["timestamp"] = 0.0
    _POLICY_LOCK = None
    _POLICY_LOCK_LOOP = None


def _load_static_policy_markdown() -> str:
    """Read the co-located authoritative static markdown policy document."""
    policy_path = Path(__file__).parent / "company_policies.md"
    if policy_path.exists():
        return policy_path.read_text(encoding="utf-8")
    return (
        "# SwiftDrop Policies\n"
        "Max weight: 50kg. Prohibited items: hazardous chemicals, explosives, narcotics."
    )


def _filter_policy_sections(full_text: str, query: str) -> str:
    """Extract relevant sections from the policy document matching query keywords."""
    query_lower = query.lower()
    sections = re.split(r"\n(?=## \d+\. )", full_text)

    # Keywords mapping to section headers or content
    matched_sections = []
    for section in sections:
        section_lower = section.lower()
        # Direct word hit or header match
        tokens = [t for t in re.findall(r"\w+", query_lower) if len(t) > 2]
        if any(token in section_lower for token in tokens):
            matched_sections.append(section.strip())

    if matched_sections:
        return "\n\n---\n\n".join(matched_sections)

    # If no specific section matched, return structured category index to prevent token inflation
    return (
        "No specific policy sections matched your inquiry. "
        "SwiftDrop policies cover 4 main categories:\n"
        "1. **Packaging Standards:** Maximum weight 50.0kg, maximum girth (Length + 2*(Width + Height)) 300.0cm.\n"
        "2. **Prohibited Items:** Explosives, radioactive material, flammable liquids, narcotics, perishables.\n"
        "3. **Transit SLAs:** Express (24 hours), Standard (48-72 hours), Outstation (3-5 business days).\n"
        "4. **Claims & Disputes:** Damaged package claims must be filed with photographic proof within 48 hours.\n"
        "Please specify which category you would like more information on."
    )


@register_tool(
    name="explain_company_policies",
    description="Explain courier policies, prohibited goods, packaging standards, transit SLAs, and dispute rules.",
    input_schema=CompanyPolicyInput,
    is_read_only=True,
)
async def policy_tool(
    input_data: CompanyPolicyInput,
    context: Optional[Dict[str, Any]] = None,
    client: Optional[httpx.AsyncClient] = None,
) -> Dict[str, Any]:
    """Retrieve authoritative policy rules with dynamic Core API precedence and static fallback."""
    settings = get_settings()
    now = datetime.now(timezone.utc).timestamp()

    dynamic_content: Optional[str] = None
    source = "static_knowledge_base"

    # 1. Check in-memory cache for dynamic settings (fast path)
    if _POLICY_CACHE["data"] is not None and (now - _POLICY_CACHE["timestamp"]) < _CACHE_TTL_SECONDS:
        dynamic_content = _POLICY_CACHE["data"]
        source = "dynamic_admin_settings_cached"
    else:
        # Cache miss or expired: acquire lock for thread/coroutine safe refresh
        async with get_policy_lock():
            now = datetime.now(timezone.utc).timestamp()
            # Double-check inside lock: another coroutine might have already refreshed it
            if _POLICY_CACHE["data"] is not None and (now - _POLICY_CACHE["timestamp"]) < _CACHE_TTL_SECONDS:
                dynamic_content = _POLICY_CACHE["data"]
                source = "dynamic_admin_settings_cached"
            else:
                # Attempt to query Core API admin settings if token is configured
                service_token = settings.ai_service_role_token.get_secret_value()
                if service_token:
                    target_url = f"{settings.core_api_base_url.rstrip('/')}/api/v1/admin/settings"
                    headers = {
                        "Authorization": f"Bearer {service_token}",
                        "Accept": "application/json",
                    }
                    timeouts = httpx.Timeout(connect=2.0, read=4.0, write=4.0, pool=4.0)
                    close_client = False
                    if client is None:
                        client_instance = httpx.AsyncClient(timeout=timeouts)
                        close_client = True
                    else:
                        client_instance = client

                    try:
                        resp = await client_instance.get(target_url, headers=headers)
                        if resp.status_code == 200:
                            data = resp.json()
                            # Look for policy overrides in settings JSON
                            if isinstance(data, dict) and "policies" in data:
                                dynamic_content = json.dumps(data["policies"], indent=2)
                                _POLICY_CACHE["data"] = dynamic_content
                                _POLICY_CACHE["timestamp"] = now
                                source = "dynamic_admin_settings"
                    except Exception:
                        # Dynamic fetch failed; gracefully fallback to static file
                        pass
                    finally:
                        if close_client:
                            await client_instance.aclose()

    # 2. Use dynamic content or fallback to static markdown
    if dynamic_content:
        content_to_search = dynamic_content
    else:
        content_to_search = _load_static_policy_markdown()

    relevant_policy = _filter_policy_sections(content_to_search, input_data.query)

    return {
        "query": input_data.query,
        "policy_content": relevant_policy,
        "source": source,
        "grounded": True,
    }
