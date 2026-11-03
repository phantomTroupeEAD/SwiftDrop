"""In-memory conversational session cache with TTL eviction and DoS memory limits."""

import asyncio
from typing import Any, Dict, List, Optional
from uuid import UUID
from cachetools import TTLCache

from src.core.config import get_settings
from src.domain.chat import ChatMessage


class SessionCache:
    """Thread-safe, bounded in-memory LRU cache with TTL expiration for conversation turns."""

    def __init__(self, maxsize: Optional[int] = None, ttl_seconds: Optional[int] = None):
        settings = get_settings()
        self._maxsize = maxsize if maxsize is not None else settings.AI_SESSION_MAX_ACTIVE
        self._ttl = ttl_seconds if ttl_seconds is not None else (settings.AI_SESSION_TTL_MINUTES * 60)
        self._cache: TTLCache = TTLCache(maxsize=self._maxsize, ttl=self._ttl)
        self._lock = asyncio.Lock()
        self._eviction_count = 0

    def _make_key(self, tenant_id: Optional[str], conversation_id: UUID | str) -> str:
        """Construct scoped cache key ensuring strict tenant boundary isolation."""
        tenant_scope = tenant_id.strip() if tenant_id and tenant_id.strip() else "anonymous"
        return f"{tenant_scope}:{str(conversation_id).strip().lower()}"

    async def get_history(
        self,
        conversation_id: UUID | str,
        tenant_id: Optional[str] = None,
    ) -> List[ChatMessage]:
        """Retrieve recent conversation turns for a session. Returns empty list if expired or not found."""
        key = self._make_key(tenant_id, conversation_id)
        async with self._lock:
            # TTLCache automatically handles expiration upon key access
            turns = self._cache.get(key)
            if turns is None:
                return []
            return list(turns)

    async def append_turns(
        self,
        conversation_id: UUID | str,
        user_turn: ChatMessage,
        assistant_turn: ChatMessage,
        tenant_id: Optional[str] = None,
    ) -> None:
        """Append user and assistant turns to the active session, capping context at 10 messages."""
        key = self._make_key(tenant_id, conversation_id)
        async with self._lock:
            current: List[ChatMessage] = self._cache.get(key, [])
            updated = list(current) + [user_turn, assistant_turn]
            # Cap at last 10 messages to protect memory and LLM token limits
            if len(updated) > 10:
                updated = updated[-10:]
            self._cache[key] = updated

    async def clear_session(
        self,
        conversation_id: UUID | str,
        tenant_id: Optional[str] = None,
    ) -> bool:
        """Evict a specific session from cache. Returns True if found and removed."""
        key = self._make_key(tenant_id, conversation_id)
        async with self._lock:
            if key in self._cache:
                del self._cache[key]
                return True
            return False

    async def clear_all(self) -> None:
        """Flush all active sessions (used during test teardown)."""
        async with self._lock:
            self._cache.clear()

    async def get_stats(self) -> Dict[str, Any]:
        """Return operational metrics for monitoring and capacity audits."""
        async with self._lock:
            return {
                "active_sessions": len(self._cache),
                "max_capacity": self._maxsize,
                "ttl_seconds": self._ttl,
            }


# Global session cache singleton
_global_cache: Optional[SessionCache] = None


def get_session_cache() -> SessionCache:
    """Return application session cache singleton."""
    global _global_cache
    if _global_cache is None:
        _global_cache = SessionCache()
    return _global_cache
