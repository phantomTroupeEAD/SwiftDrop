"""Output guardrails, identity protection, and sanitizers for LLM responses."""

import re

# Comprehensive third-party model and provider regex
THIRD_PARTY_MODELS_REGEX = (
    r"(?i)\b("
    r"deepseek(-[a-z0-9]+)?|"
    r"kimi|"
    r"qwen(-[a-z0-9\.]+)?|"
    r"moonshot(\s*ai)?|"
    r"gemini(\s*pro|\s*flash)?|"
    r"chatgpt|"
    r"openai|"
    r"anthropic|"
    r"claude(-[a-z0-9\.]+)?|"
    r"llama(-[a-z0-9\.]+)?|"
    r"meta\s*ai|"
    r"particle(\s*ai)?|"
    r"mistral(\s*ai)?"
    r")\b"
)


def sanitize_assistant_identity(text: str) -> str:
    """Ensure the assistant never discloses third-party LLM providers or models.

    Replaces third-party model disclosures (DeepSeek, Kimi, Qwen, Moonshot,
    Gemini, OpenAI, Anthropic, Meta, etc.) with the official 'SwiftDrop AI' identity.
    """
    if not text:
        return text

    sanitized = text

    # Pattern 1: "I am [a large language model/an AI model] trained/developed by <Provider>"
    p1 = (
        r"(?i)I am (a large language model|an AI assistant|an AI|a language model) "
        r"(trained|developed|created|powered) by\s+[A-Za-z0-9\s\-]+"
    )
    sanitized = re.sub(
        p1,
        "I am SwiftDrop AI, the official AI delivery and logistics assistant for SwiftDrop",
        sanitized,
    )

    # Pattern 2: "As an AI/language model trained by <Provider>"
    p2 = r"(?i)As an? (AI|language model) (trained|developed|created|powered) by\s+[A-Za-z0-9\s\-]+"
    sanitized = re.sub(p2, "As SwiftDrop AI", sanitized)

    # Pattern 3: Direct identity claims: "I am DeepSeek...", "I am Qwen...", "I am Gemini...", etc.
    p3 = (
        r"(?i)\bI am\s+(?:a\s+)?(DeepSeek(?:-[A-Za-z0-9]+)?|Kimi|Qwen(?:-[A-Za-z0-9\.]+)?|"
        r"Moonshot(?:\s*AI)?|Gemini(?:\s*Pro|\s*Flash)?|ChatGPT|Claude(?:-[A-Za-z0-9\.]+)?|"
        r"Llama(?:-[A-Za-z0-9\.]+)?|Particle(?:\s*AI)?|Mistral(?:\s*AI)?)\b"
    )
    sanitized = re.sub(p3, "I am SwiftDrop AI", sanitized)

    # Pattern 4: "(developed|trained|created) by (DeepSeek|Moonshot|Google|Alibaba|OpenAI|Meta|Anthropic)"
    p4 = (
        r"(?i)(developed|trained|created|fine-tuned)\s+by\s+(DeepSeek|Moonshot(?:\s*AI)?|"
        r"Kimi|Alibaba|Google|OpenAI|Meta|Anthropic|Baichuan)"
    )
    sanitized = re.sub(p4, r"\1 for the SwiftDrop courier network", sanitized)

    # Pattern 5: Scrub leaked internal tool or function names
    tool_replacements = [
        (r"(`)?estimate_delivery_fee(`)?(\s+tool)?", "fee estimation engine"),
        (r"(`)?estimate_shipping_fee(`)?(\s+tool)?", "fee estimation engine"),
        (r"(`)?track_parcel_status(`)?(\s+tool)?", "parcel tracking system"),
        (r"(`)?track_parcel(`)?(\s+tool)?", "parcel tracking system"),
        (r"(`)?list_active_parcels(`)?(\s+tool)?", "order lookup system"),
        (r"(`)?get_my_active_parcels(`)?(\s+tool)?", "order lookup system"),
        (r"(`)?explain_company_policies(`)?(\s+tool)?", "shipping policy guidelines"),
    ]
    for pattern, replacement in tool_replacements:
        sanitized = re.sub(pattern, replacement, sanitized, flags=re.IGNORECASE)

    # Pattern 6: Scrub mobile app hallucinations (SwiftDrop is strictly a web portal)
    sanitized = re.sub(
        r"(?i)\b(?:portal\s+)?or\s+(?:the\s+|our\s+)?mobile\s+apps?\b",
        "web portal",
        sanitized,
    )
    sanitized = re.sub(r"(?i)\bmobile\s+apps?\b", "web portal", sanitized)
    sanitized = re.sub(r"(?i)\b(portal\s+web\s+portal|web\s+portal\s+or\s+web\s+portal)\b", "web portal", sanitized)

    # Pattern 7: SEC-009 Privacy Masking — Redact customer personal phone numbers and street lines
    # Matches +94 7X XXX XXXX, +947XXXXXXXX, 07XXXXXXXX, 011XXXXXXX
    sanitized = re.sub(
        r"(\+94\s*\d{1,2}\s*\d{3}\s*\d{4}|07[0-9]\s*\d{3}\s*\d{4}|\b07[0-9]{8}\b|\b011[0-9]{7}\b)",
        "[PHONE REDACTED]",
        sanitized,
    )

    return sanitized
