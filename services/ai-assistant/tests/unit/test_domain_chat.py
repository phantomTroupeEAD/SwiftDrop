"""Unit tests for chat DTOs, prompt length hygiene, and Generative UI URL validation."""

from uuid import uuid4
import pytest
from pydantic import ValidationError
from src.domain.chat import (
    ActionType,
    ChatMessage,
    ChatMessageRequest,
    ChatMessageResponse,
    GenerativeAction,
    MessageRole,
)


def test_chat_message_valid():
    """Verify valid chat message creation and sanitization."""
    msg = ChatMessage(role=MessageRole.USER, content="Hello assistant\x00")
    # Null byte should be stripped
    assert msg.content == "Hello assistant"
    assert msg.role == MessageRole.USER


def test_chat_message_empty_content_rejected():
    """Verify empty or whitespace-only messages are rejected."""
    with pytest.raises(ValidationError):
        ChatMessage(role=MessageRole.USER, content="   ")

    with pytest.raises(ValidationError):
        ChatMessage(role=MessageRole.USER, content="\x00\x00")


def test_generative_action_valid_urls():
    """Verify approved internal relative URLs pass validation."""
    valid_urls = [
        "/portal/book",
        "/customer/book",
        "/tracking",
        "/tracking?num=SD-20261021-04921",
        "/support",
        "/support?issue=damaged_box",
    ]
    for url in valid_urls:
        action = GenerativeAction(
            action_type=ActionType.NAVIGATE_BOOKING,
            label="Book Now",
            target_url=url,
        )
        assert action.target_url == url


def test_generative_action_prohibited_urls():
    """Verify open redirects, external protocols, and XSS attack vectors are rejected."""
    prohibited_urls = [
        "https://evil-phishing.com/swiftdrop",
        "http://attacker.com/login",
        "//evil.com/redirect",
        "javascript:alert(document.cookie)",
        "javascript:void(0)",
        "data:text/html,<script>alert(1)</script>",
        "/unapproved/internal/path",
        "/admin/secret/database",
    ]
    for bad_url in prohibited_urls:
        with pytest.raises(ValidationError):
            GenerativeAction(
                action_type=ActionType.NAVIGATE_BOOKING,
                label="Click Here",
                target_url=bad_url,
            )


def test_chat_request_prompt_length_constraints():
    """Verify prompt length is bounded to 1-1,000 characters."""
    valid_prompt = "A" * 1000
    req = ChatMessageRequest(prompt=valid_prompt)
    assert len(req.prompt) == 1000

    # Over 1000 characters
    too_long_prompt = "A" * 1001
    with pytest.raises(ValidationError) as exc_info:
        ChatMessageRequest(prompt=too_long_prompt)
    assert "Prompt length cannot exceed 1000 characters" in str(exc_info.value) or "at most 1000 characters" in str(exc_info.value)

    # Empty or whitespace only
    with pytest.raises(ValidationError):
        ChatMessageRequest(prompt="    ")


def test_chat_request_history_capping():
    """Verify that history is capped to the last 10 messages."""
    history = [
        ChatMessage(role=MessageRole.USER, content=f"Message {i}") for i in range(15)
    ]
    req = ChatMessageRequest(prompt="Latest prompt", history=history)
    assert len(req.history) == 10
    # Must preserve the most recent messages (indices 5 through 14)
    assert req.history[0].content == "Message 5"
    assert req.history[-1].content == "Message 14"


def test_chat_response_structure():
    """Verify ChatMessageResponse serialization."""
    session_id = uuid4()
    resp = ChatMessageResponse(
        conversation_id=session_id,
        reply="Your delivery fee is LKR 500.00",
        actions=[
            GenerativeAction(
                action_type=ActionType.NAVIGATE_BOOKING,
                label="Proceed",
                target_url="/portal/book",
                prefill_data={"fee": 500.0},
            )
        ],
        tools_called=["estimate_shipping_fee"],
        latency_ms=120,
        grounded=True,
    )
    assert resp.conversation_id == session_id
    assert resp.latency_ms == 120
    assert len(resp.actions) == 1
    assert resp.tools_called == ["estimate_shipping_fee"]
