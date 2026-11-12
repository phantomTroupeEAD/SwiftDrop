"""Central registry and dispatcher for approved read-only tools."""

import asyncio
from dataclasses import dataclass
import inspect
import time
from typing import Any, Callable, Dict, List, Optional, Type
from pydantic import BaseModel, ValidationError

from src.domain.chat import ToolCallRecord
from src.domain.errors import AppException, ValidationException


@dataclass
class ToolDefinition:
    """Metadata describing a registered tool capability."""

    name: str
    description: str
    input_schema: Type[BaseModel]
    handler: Callable[..., Any]
    is_read_only: bool = True


# In-memory dictionary storing registered tool definitions
_TOOL_REGISTRY: Dict[str, ToolDefinition] = {}


def register_tool(
    name: str,
    description: str,
    input_schema: Type[BaseModel],
    is_read_only: bool = True,
    aliases: Optional[List[str]] = None,
) -> Callable[[Callable[..., Any]], Callable[..., Any]]:
    """Decorator to register an approved tool in the central tool registry."""

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        tool_def = ToolDefinition(
            name=name,
            description=description,
            input_schema=input_schema,
            handler=func,
            is_read_only=is_read_only,
        )
        _TOOL_REGISTRY[name] = tool_def
        if aliases:
            for alias in aliases:
                _TOOL_REGISTRY[alias] = tool_def
        return func

    return decorator


def get_registered_tools() -> Dict[str, ToolDefinition]:
    """Retrieve all registered tool definitions."""
    return dict(_TOOL_REGISTRY)


def clear_registry() -> None:
    """Clear registered tools (useful for isolated unit testing)."""
    _TOOL_REGISTRY.clear()


def export_openai_tools() -> List[Dict[str, Any]]:
    """Export all registered tools in standard OpenAI function calling format."""
    schemas: List[Dict[str, Any]] = []
    seen = set()
    for tool in _TOOL_REGISTRY.values():
        if tool.name in seen:
            continue
        seen.add(tool.name)
        schemas.append(
            {
                "type": "function",
                "function": {
                    "name": tool.name,
                    "description": tool.description,
                    "parameters": tool.input_schema.model_json_schema(),
                },
            }
        )
    return schemas


async def execute_tool(
    name: str,
    arguments: Dict[str, Any],
    context: Optional[Dict[str, Any]] = None,
) -> ToolCallRecord:
    """Execute a registered tool with validation, execution timing, and structured error boundaries."""
    start_time = time.perf_counter()

    if name not in _TOOL_REGISTRY:
        duration_ms = int((time.perf_counter() - start_time) * 1000)
        return ToolCallRecord(
            tool_name=name,
            arguments=arguments,
            status="ERROR",
            error_message=f"Tool '{name}' is not registered in the tool registry",
            duration_ms=duration_ms,
        )

    tool_def = _TOOL_REGISTRY[name]

    # 1. Validate arguments against Pydantic schema
    try:
        validated_input = tool_def.input_schema(**arguments)
    except ValidationError as val_err:
        duration_ms = int((time.perf_counter() - start_time) * 1000)
        messages = [
            f"{'.'.join(str(loc) for loc in err.get('loc', []))}: {err.get('msg')}"
            for err in val_err.errors()
        ]
        error_summary = "; ".join(messages) if messages else str(val_err)
        return ToolCallRecord(
            tool_name=name,
            arguments=arguments,
            status="ERROR",
            error_message=f"Invalid arguments for tool '{name}': {error_summary}",
            duration_ms=duration_ms,
        )

    # 2. Invoke handler passing context if accepted
    try:
        sig = inspect.signature(tool_def.handler)
        kwargs: Dict[str, Any] = {}
        if "context" in sig.parameters:
            kwargs["context"] = context or {}

        if inspect.iscoroutinefunction(tool_def.handler):
            raw_result = await tool_def.handler(validated_input, **kwargs)
        else:
            raw_result = tool_def.handler(validated_input, **kwargs)

        duration_ms = int((time.perf_counter() - start_time) * 1000)
        return ToolCallRecord(
            tool_name=name,
            arguments=arguments,
            result=raw_result,
            status="SUCCESS",
            duration_ms=duration_ms,
        )

    except AppException as app_err:
        duration_ms = int((time.perf_counter() - start_time) * 1000)
        return ToolCallRecord(
            tool_name=name,
            arguments=arguments,
            result=app_err.to_dict(),
            status="ERROR",
            error_message=app_err.message,
            duration_ms=duration_ms,
        )
    except Exception as exc:
        duration_ms = int((time.perf_counter() - start_time) * 1000)
        return ToolCallRecord(
            tool_name=name,
            arguments=arguments,
            result={"error": {"code": "INTERNAL_TOOL_ERROR", "message": str(exc)}},
            status="ERROR",
            error_message=str(exc),
            duration_ms=duration_ms,
        )
