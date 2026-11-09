"""Dual LLM engine: Universal OpenAI-compatible adapter and deterministic mock adapter."""

from src.core.config import get_settings
from src.llm.base import BaseLLMClient
from src.llm.mock_adapter import MockLLMAdapter
from src.llm.openai_adapter import OpenAIAdapter
from src.llm.prompts import SYSTEM_PROMPT


def get_llm_client() -> BaseLLMClient:
    """Return the configured LLM engine: MockLLMAdapter for offline CI or OpenAIAdapter for production."""
    settings = get_settings()
    if settings.is_mock_mode:
        return MockLLMAdapter()
    return OpenAIAdapter()


__all__ = [
    "BaseLLMClient",
    "OpenAIAdapter",
    "MockLLMAdapter",
    "get_llm_client",
    "SYSTEM_PROMPT",
]
