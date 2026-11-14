export function withRequestReference(message: string, response: Response, correlationId?: unknown): string {
  const reference = typeof correlationId === 'string' && correlationId.trim()
    ? correlationId.trim()
    : response.headers.get('X-Request-ID')?.trim();

  return reference ? `${message} (Reference: ${reference})` : message;
}
