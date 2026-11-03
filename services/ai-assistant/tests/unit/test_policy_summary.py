"""Unit tests verifying policy query filtering and unmatched summary index (SEC-CLD-007)."""

import pytest
from src.domain.tools import CompanyPolicyInput
from src.tools.policy_tool import policy_tool, reset_policy_cache


@pytest.mark.asyncio
async def test_policy_unmatched_query_returns_four_chapter_summary():
    """Unmatched inquiries return a structured policy index rather than dumping the full document."""
    reset_policy_cache()
    # Obscure or off-topic query with no policy keywords
    result = await policy_tool(CompanyPolicyInput(query="cryptocurrency mining hardware installation"))

    assert result["grounded"] is True
    content = result["policy_content"]
    # 1. Contains the 4 main category pointers
    assert "Packaging Standards:" in content
    assert "Prohibited Items:" in content
    assert "Transit SLAs:" in content
    assert "Claims & Disputes:" in content
    # 2. Significantly shorter than full 5,000-char document
    assert len(content) < 800


@pytest.mark.asyncio
async def test_policy_matched_query_returns_specific_section():
    """Matched inquiries return the exact relevant section."""
    reset_policy_cache()
    result = await policy_tool(CompanyPolicyInput(query="prohibited explosive goods"))

    assert result["grounded"] is True
    content = result["policy_content"].lower()
    assert "prohibited" in content
    assert "explosive" in content
