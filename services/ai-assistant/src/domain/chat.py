"""Domain models and DTOs for conversational interactions and generative UI actions."""

from datetime import datetime, timezone
from enum import Enum
import json
import re
from typing import Any, Dict, List, Optional
from uuid import UUID, uuid4
from pydantic import BaseModel, Field, field_validator


class MessageRole(str, Enum):
    """Permitted conversational roles."""

    USER = "user"
    ASSISTANT = "assistant"
    SYSTEM = "system"


class ActionType(str, Enum):
    """Permitted generative UI action types for client-side execution."""

    NAVIGATE_BOOKING = "NAVIGATE_BOOKING"
    VIEW_TRACKING = "VIEW_TRACKING"
    CONTACT_SUPPORT = "CONTACT_SUPPORT"


class ChatMessage(BaseModel):
    """A single turn within a multi-turn conversation."""

    role: MessageRole
    content: str = Field(min_length=1, max_length=10000)
    timestamp: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))

    @field_validator("content")
    @classmethod
    def sanitize_content(cls, v: str) -> str:
        """Strip null bytes and control characters from chat message content."""
        cleaned = v.replace("\x00", "").strip()
        if not cleaned:
            raise ValueError("Message content cannot be empty or whitespace only")
        return cleaned


class GenerativeAction(BaseModel):
    """Structured action card rendered by the React web portal."""

    action_type: ActionType
    label: str = Field(min_length=1, max_length=100, description="Button/card label")
    target_url: str = Field(description="Strictly relative internal URL path")
    prefill_data: Dict[str, Any] = Field(
        default_factory=dict,
        description="Parameters passed to destination form",
    )

    @field_validator("target_url")
    @classmethod
    def validate_target_url(cls, v: str) -> str:
        """Enforce strict relative URL whitelisting to eliminate Open Redirect and XSS."""
        v_clean = v.strip()
        # Must start with forward slash and match approved portal routes
        approved_pattern = r"^/(portal/book|customer/book|tracking|support)(\?.*)?$"
        if not re.match(approved_pattern, v_clean):
            raise ValueError(
                f"Invalid or prohibited target_url '{v_clean}'. "
                "Must be a relative path to /portal/book, /tracking, or /support"
            )
        # Explicitly ban javascript:, data:, or protocol relative //
        if v_clean.lower().startswith(("//", "javascript:", "data:", "vbscript:")):
            raise ValueError(f"Dangerous URI scheme detected in target_url: {v_clean}")
        return v_clean


class ChatMessageRequest(BaseModel):
    """Inbound customer or merchant message payload for POST /api/v1/ai/chat."""

    prompt: str = Field(
        min_length=1,
        max_length=1000,
        description="User natural language inquiry (capped at 1,000 characters)",
    )
    conversation_id: UUID = Field(
        default_factory=uuid4,
        description="Unique session UUID for conversation correlation. Reusing the returned conversation_id across turns maintains chat memory.",
    )
    locale: str = Field(
        default="en-US",
        pattern=r"^[a-z]{2}(-[A-Z]{2})?$",
        description="Client language locale",
    )
    history: List[ChatMessage] = Field(
        default_factory=list,
        description="Optional recent message history (capped at max 10 past turns)",
    )

    @field_validator("prompt")
    @classmethod
    def sanitize_prompt(cls, v: str) -> str:
        """Sanitize prompt input, stripping null bytes and enforcing non-empty string."""
        cleaned = v.replace("\x00", "").strip()
        if not cleaned:
            raise ValueError("Prompt cannot be empty or whitespace only")
        if len(cleaned) > 1000:
            raise ValueError("Prompt length cannot exceed 1000 characters")
        return cleaned

    @field_validator("history")
    @classmethod
    def cap_history(cls, v: List[ChatMessage]) -> List[ChatMessage]:
        """Limit historical context window to the last 10 messages to avoid token blowup."""
        if len(v) > 10:
            return v[-10:]
        return v


class ToolCallRecord(BaseModel):
    """Audit record of a tool invocation executed during prompt resolution."""

    tool_name: str
    arguments: Dict[str, Any] = Field(default_factory=dict)
    result: Optional[Any] = None
    status: str = Field(default="SUCCESS", pattern="^(SUCCESS|ERROR|SKIPPED)$")
    error_message: Optional[str] = None
    duration_ms: Optional[int] = Field(
        default=None,
        description="Execution duration in milliseconds",
    )


class ChatMessageResponse(BaseModel):
    """Standardized response envelope returned by the AI assistant."""

    conversation_id: UUID
    reply: str = Field(description="Markdown-formatted assistant response")
    actions: List[GenerativeAction] = Field(
        default_factory=list,
        description="Array of structured UI navigation cards",
    )
    tools_called: List[str] = Field(
        default_factory=list,
        description="List of tool names executed during generation",
    )
    latency_ms: int = Field(ge=0, description="Total request latency in milliseconds")
    grounded: bool = Field(
        default=True,
        description="Indicates whether the answer is verified and grounded in tool output",
    )


class StreamEventType(str, Enum):
    """Permitted Server-Sent Events (SSE) event names."""

    TOOL_START = "tool_start"
    TOOL_END = "tool_end"
    TOKEN = "token"
    ACTION_CARD = "action_card"
    DONE = "done"
    ERROR = "error"
    PING = "ping"


class StreamChunkEvent(BaseModel):
    """Standardized representation of a single SSE stream chunk before serialization."""

    event: StreamEventType
    data: Dict[str, Any] = Field(default_factory=dict)

    def to_sse_frame(self) -> str:
        """Format chunk event as standard W3C Server-Sent Event frame."""
        return to_sse_frame(self.event, self.data)


def to_sse_frame(event_type: StreamEventType | str, data: Any) -> str:
    """Format arbitrary event payload as a valid W3C SSE frame."""
    event_name = event_type.value if isinstance(event_type, StreamEventType) else str(event_type)
    if isinstance(data, BaseModel):
        data_json = data.model_dump_json()
    elif isinstance(data, dict):
        data_json = json.dumps(data, separators=(",", ":"), ensure_ascii=False)
    else:
        data_json = json.dumps({"value": data}, separators=(",", ":"), ensure_ascii=False)
    return f"event: {event_name}\ndata: {data_json}\n\n"
