import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertCircle,
  ArrowRight,
  ExternalLink,
  Package,
  Search,
  Send,
  Sparkles,
  X,
} from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { Button } from '@/shared/components/ui/button';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { apiBase, errorMessage, portalRequest } from './api';

type AssistantAction = {
  action_type: string;
  label?: string;
  target_url: string;
  prefill_data?: Record<string, unknown>;
};

type ChatResponse = {
  conversation_id: string;
  reply: string;
  actions?: AssistantAction[];
};

type Turn = {
  prompt: string;
  reply: string;
  booking?: string;
  tracking?: { url: string; label: string };
};

function resolveBookingLink(action: AssistantAction, role?: string): string {
  if (role === 'MERCHANT' || (action.target_url && action.target_url.includes('merchant'))) {
    return '/portal/merchants/orders';
  }
  const data = action.prefill_data;
  const params = new URLSearchParams();
  for (const key of ['pickup_postal_code', 'delivery_postal_code']) {
    const code = data?.[key];
    if (typeof code === 'string' && /^\d{5}$/.test(code)) params.set(key, code);
  }
  const weight = data?.weight_kg;
  if (typeof weight === 'number' && weight >= 0.01 && weight <= 30) params.set('weight_kg', String(weight));
  for (const key of ['length_cm', 'width_cm', 'height_cm']) {
    const dimension = data?.[key];
    if (typeof dimension === 'number' && dimension >= 0.01 && dimension <= 150) params.set(key, String(dimension));
  }
  if (data?.service_type === 'EXPRESS') params.set('service_type', 'EXPRESS');
  return params.size ? `/portal/book?${params}` : '/portal/book';
}

function resolveTrackingLink(action: AssistantAction): { url: string; label: string } {
  const trackingNumber = typeof action.prefill_data?.tracking_number === 'string'
    ? action.prefill_data.tracking_number
    : '';
  const url = action.target_url || (trackingNumber ? `/track/${trackingNumber}` : '/track');
  const label = action.label || (trackingNumber ? `Track ${trackingNumber}` : 'View tracking milestone');
  return { url, label };
}

function formatToolLabel(toolName: string): string {
  switch (toolName) {
    case 'estimate_delivery_fee':
    case 'estimate_shipping_fee':
      return 'Calculating delivery tariff…';
    case 'track_parcel_status':
    case 'track_parcel':
      return 'Querying tracking milestones…';
    case 'list_active_parcels':
    case 'get_my_active_parcels':
      return 'Retrieving active parcels…';
    case 'explain_company_policies':
      return 'Checking shipping policies…';
    case 'lookup_postal_code':
      return 'Resolving postal district…';
    default:
      return 'Consulting logistics engine…';
  }
}

function FormattedMessage({ text }: { text: string }) {
  const parts = text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g);
  return (
    <span className="whitespace-pre-wrap break-words">
      {parts.map((part, idx) => {
        if (part.startsWith('**') && part.endsWith('**') && part.length >= 4) {
          return <strong key={idx} className="font-semibold text-foreground">{part.slice(2, -2)}</strong>;
        }
        if (part.startsWith('`') && part.endsWith('`') && part.length >= 2) {
          return (
            <code
              key={idx}
              className="rounded-sm border border-border-subtle bg-surface-subtle px-1.5 py-0.5 font-mono text-xs tabular-nums text-foreground"
            >
              {part.slice(1, -1)}
            </code>
          );
        }
        return part;
      })}
    </span>
  );
}

export function AssistantChat({
  token,
  role,
}: {
  token?: string | null;
  role?: string;
}) {
  const [open, setOpen] = useState(false);
  const launcher = useRef<HTMLButtonElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const [draft, setDraft] = useState('');
  const [turns, setTurns] = useState<Turn[]>([]);
  const [conversation, setConversation] = useState<string>();
  const [pending, setPending] = useState(false);
  const [activeTool, setActiveTool] = useState<string | null>(null);
  const [streamingReply, setStreamingReply] = useState('');
  const [error, setError] = useState('');
  const request = useRef<AbortController>();
  const transcript = useRef<HTMLDivElement>(null);

  useEffect(() => () => request.current?.abort(), []);
  useEffect(() => {
    const element = transcript.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [turns, pending, streamingReply, activeTool, open]);
  useEffect(() => {
    if (open) composer.current?.focus();
  }, [open]);

  const quickPrompts = role === 'MERCHANT'
    ? [
        'Estimate freight tariff for bulk orders',
        'How do I configure order webhook notifications?',
        'What are the express courier SLAs?',
      ]
    : [
        'How much to ship 2.5kg from 00100 to 00400?',
        'Where do I find my parcel tracking number?',
        'What are the standard package dimensions?',
      ];

  async function send(event: FormEvent) {
    event.preventDefault();
    const prompt = draft.trim();
    if (!prompt || request.current) return;

    const controller = new AbortController();
    request.current = controller;
    setError('');
    setActiveTool(null);
    setPending(true);
    setStreamingReply('');

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream, application/json',
    };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    try {
      // 1. Attempt SSE Streaming endpoint first for lively Apple HIG UX
      let streamResponse: Response | undefined;
      try {
        streamResponse = await fetch(apiBase + '/ai/chat/stream', {
          method: 'POST',
          headers,
          body: JSON.stringify({ prompt, conversation_id: conversation }),
          signal: controller.signal,
        });
      } catch {
        // Network or CORS error on streaming; will fall back to /ai/chat below
      }

      // If streaming responded with 200 and text/event-stream, consume the stream
      if (
        streamResponse?.ok &&
        streamResponse.headers.get('content-type')?.includes('text/event-stream') &&
        streamResponse.body
      ) {
        const reader = streamResponse.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let accumulatedReply = '';
        const accumulatedActions: AssistantAction[] = [];
        let nextConvId = conversation;

        let reading = true;
        while (reading) {
          const { done, value } = await reader.read();
          if (done) {
            reading = false;
            break;
          }
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() ?? '';

          let currentEvent = 'message';
          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed) {
              currentEvent = 'message';
              continue;
            }
            if (trimmed.startsWith('event:')) {
              currentEvent = trimmed.slice(6).trim();
            } else if (trimmed.startsWith('data:')) {
              const dataStr = trimmed.slice(5).trim();
              try {
                const data = JSON.parse(dataStr);
                if (currentEvent === 'token') {
                  if (typeof data.delta === 'string') {
                    accumulatedReply += data.delta;
                    setStreamingReply(accumulatedReply);
                  }
                } else if (currentEvent === 'tool_start') {
                  setActiveTool(formatToolLabel(data.tool));
                } else if (currentEvent === 'tool_end') {
                  setActiveTool(null);
                } else if (currentEvent === 'action_card') {
                  if (data.action_type && data.target_url) {
                    accumulatedActions.push(data);
                  }
                } else if (currentEvent === 'done') {
                  if (data.conversation_id) nextConvId = data.conversation_id;
                } else if (currentEvent === 'error') {
                  throw new Error(data.message || 'Error occurred during streaming');
                }
              } catch {
                // Ignore ping or non-json framing lines
              }
            }
          }
        }

        if (accumulatedReply.startsWith('Unable to calculate shipping estimate:')) {
          setTurns(prev => [
            ...prev,
            {
              prompt,
              reply: 'I could not get a delivery estimate here. You can request a quote in the booking form.',
              booking: role === 'MERCHANT' ? '/portal/merchants/orders' : '/portal/book',
            },
          ]);
        } else {
          const bookingAction = accumulatedActions.find(a => a.action_type === 'NAVIGATE_BOOKING');
          const trackingAction = accumulatedActions.find(
            a => a.action_type === 'VIEW_TRACKING' || a.action_type === 'NAVIGATE_TRACKING',
          );

          setTurns(prev => [
            ...prev,
            {
              prompt,
              reply: accumulatedReply,
              booking: bookingAction ? resolveBookingLink(bookingAction, role) : undefined,
              tracking: trackingAction ? resolveTrackingLink(trackingAction) : undefined,
            },
          ]);
        }

        if (nextConvId) setConversation(nextConvId);
        setDraft('');
        return;
      }

      // If streamResponse returned a direct JSON response (e.g. test mock on /ai/chat/stream)
      if (streamResponse?.ok && streamResponse.headers.get('content-type')?.includes('application/json')) {
        const jsonResponse: ChatResponse = await streamResponse.json();
        applyJsonResponse(prompt, jsonResponse);
        return;
      }

      // 2. Fallback to standard blocking /ai/chat (also preserves compatibility with test mocks)
      const response = await portalRequest<ChatResponse>('/ai/chat', {
        method: 'POST',
        token,
        signal: controller.signal,
        allowUnwrapped: true,
        body: { prompt, conversation_id: conversation },
      });

      if (!response?.reply || !response.conversation_id) {
        throw new Error('Missing chat response');
      }

      applyJsonResponse(prompt, response);
    } catch (cause) {
      if (!controller.signal.aborted) setError(errorMessage(cause));
    } finally {
      request.current = undefined;
      setActiveTool(null);
      setStreamingReply('');
      if (!controller.signal.aborted) setPending(false);
    }
  }

  function applyJsonResponse(prompt: string, response: ChatResponse) {
    if (response.reply.startsWith('Unable to calculate shipping estimate:')) {
      setTurns(prev => [
        ...prev,
        {
          prompt,
          reply: 'I could not get a delivery estimate here. You can request a quote in the booking form.',
          booking: role === 'MERCHANT' ? '/portal/merchants/orders' : '/portal/book',
        },
      ]);
      setConversation(response.conversation_id);
      setDraft('');
      return;
    }

    const bookingAction = Array.isArray(response.actions)
      ? response.actions.find(a => a?.action_type === 'NAVIGATE_BOOKING')
      : undefined;
    const trackingAction = Array.isArray(response.actions)
      ? response.actions.find(a => a?.action_type === 'VIEW_TRACKING' || a?.action_type === 'NAVIGATE_TRACKING')
      : undefined;

    setTurns(prev => [
      ...prev,
      {
        prompt,
        reply: response.reply,
        booking: bookingAction ? resolveBookingLink(bookingAction, role) : undefined,
        tracking: trackingAction ? resolveTrackingLink(trackingAction) : undefined,
      },
    ]);
    setConversation(response.conversation_id);
    setDraft('');
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      if (draft.trim() && !pending) {
        void send(event as unknown as FormEvent);
      }
    }
  }

  function close() {
    setOpen(false);
    launcher.current?.focus();
  }

  return (
    <>
      <Button
        ref={launcher}
        type="button"
        variant="default"
        className="fixed bottom-5 right-4 z-40 min-h-11 rounded-full border border-border px-4 transition-all duration-150 active:scale-95 sm:right-6"
        aria-expanded={open}
        aria-controls={open ? 'assistant-chat' : undefined}
        data-testid="assistant-open-btn"
        onClick={() => {
          if (open) close();
          else setOpen(true);
        }}
      >
        <Sparkles className="size-4" aria-hidden="true" />
        <span>Assistant</span>
      </Button>

      {open && (
        <section
          id="assistant-chat"
          aria-labelledby="assistant-chat-title"
          data-testid="assistant-panel"
          onKeyDown={event => {
            if (event.key === 'Escape') close();
          }}
          className="fixed inset-x-3 bottom-20 z-50 flex max-h-[min(38rem,calc(100dvh-6rem))] flex-col overflow-hidden rounded-lg border border-border bg-surface sm:left-auto sm:right-6 sm:w-[26rem]"
        >
          {/* Apple Cupertino Styled Topbar Header */}
          <header className="flex shrink-0 items-center justify-between border-b border-border bg-surface px-4 py-3">
            <div className="flex items-center gap-2.5">
              <span className="grid size-8 shrink-0 place-items-center rounded-full bg-primary-tint text-primary">
                <Sparkles className="size-4" aria-hidden="true" />
              </span>
              <div>
                <h2 id="assistant-chat-title" className="text-sm font-semibold tracking-tight text-foreground leading-none">
                  SwiftDrop Assistant
                </h2>
                <div className="flex items-center gap-1.5 mt-1">
                  <span className="size-1.5 rounded-full bg-status-success-ink shrink-0" aria-hidden="true" />
                  <span className="text-xs font-medium text-text-muted">Live Tariff & Tracking</span>
                </div>
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon"
              type="button"
              className="size-8 rounded-sm text-text-muted hover:text-foreground"
              aria-label="Close assistant"
              onClick={close}
            >
              <X className="size-4" aria-hidden="true" />
            </Button>
          </header>

          {/* Transcript Area */}
          <div
            ref={transcript}
            role="log"
            aria-label="Conversation"
            className="min-h-0 flex-1 space-y-3.5 overflow-y-auto overscroll-contain bg-canvas p-4"
          >
            {!turns.length && !pending && (
              <div className="space-y-3 rounded-md border border-border bg-surface p-4">
                <div className="flex items-center gap-2 text-foreground font-medium text-sm">
                  <Package className="size-4 text-primary" aria-hidden="true" />
                  <span>How can I assist you today?</span>
                </div>
                <p className="text-xs text-text-muted leading-relaxed">
                  {role === 'MERCHANT'
                    ? 'Get instant freight calculations, look up merchant order milestones, or ask about business dispatch policies.'
                    : 'Calculate delivery fees, check parcel dimensions, lookup live tracking status, or review shipping rules.'}
                </p>

                {/* Quick Suggestion Chips */}
                <div className="space-y-1.5 pt-1">
                  <span className="text-xs font-medium text-text-muted uppercase tracking-wider">Suggested Queries</span>
                  <div className="flex flex-col gap-1.5">
                    {quickPrompts.map((promptText, i) => (
                      <Button
                        key={i}
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="justify-start text-left text-xs font-normal h-auto py-1.5 px-2.5 rounded-sm"
                        onClick={() => {
                          setDraft(promptText);
                          composer.current?.focus();
                        }}
                      >
                        <Search className="size-3 text-text-muted mr-1.5 shrink-0" aria-hidden="true" />
                        <span className="truncate">{promptText}</span>
                      </Button>
                    ))}
                  </div>
                </div>

                <div className="pt-2 border-t border-border-subtle">
                  <Button asChild variant="outline" size="sm" className="w-full text-xs rounded-sm" data-testid="ai-initial-book-btn">
                    <Link to={role === 'MERCHANT' ? '/portal/merchants/orders' : '/portal/book'}>
                      {role === 'MERCHANT' ? 'Manage merchant orders' : 'Estimate delivery cost'}
                    </Link>
                  </Button>
                </div>
              </div>
            )}

            {turns.map((turn, index) => (
              <div key={index} className="flex flex-col gap-2.5">
                {/* User Message Bubble */}
                <p className="max-w-[85%] self-end whitespace-pre-wrap break-words rounded-md rounded-br-sm bg-primary px-3.5 py-2.5 text-sm text-primary-foreground">
                  <span className="sr-only">You: </span>
                  {turn.prompt}
                </p>

                {/* Assistant Message Bubble */}
                <div className="max-w-[88%] self-start space-y-2.5 rounded-md rounded-bl-sm border border-border bg-surface px-3.5 py-2.5 text-sm">
                  <p className="text-foreground leading-relaxed">
                    <span className="sr-only">Assistant: </span>
                    <FormattedMessage text={turn.reply} />
                  </p>

                  {/* Grounded Action Cards */}
                  {turn.booking && (
                    <div className="mt-2 rounded-sm border border-border bg-surface-subtle p-2.5 flex items-center justify-between gap-2.5">
                      <div className="min-w-0 flex-1">
                        <p className="text-xs font-medium text-foreground truncate">Shipment Booking</p>
                        <p className="text-xs text-text-muted truncate">Prefilled tariff parameters</p>
                      </div>
                      <Button asChild variant="default" size="sm" className="text-xs shrink-0 rounded-sm" data-testid="ai-action-btn-book">
                        <Link to={turn.booking}>
                          <span>Continue</span>
                          <ArrowRight className="size-3 ml-1" aria-hidden="true" />
                        </Link>
                      </Button>
                    </div>
                  )}

                  {turn.tracking && (
                    <div className="mt-2 rounded-sm border border-border bg-surface-subtle p-2.5 flex items-center justify-between gap-2.5">
                      <div className="min-w-0 flex-1">
                        <p className="text-xs font-medium text-foreground truncate">Tracking Details</p>
                        <p className="text-xs text-text-muted truncate">Live parcel milestones</p>
                      </div>
                      <Button asChild variant="outline" size="sm" className="text-xs shrink-0 rounded-sm" data-testid="ai-action-btn-track">
                        <Link to={turn.tracking.url}>
                          <span>{turn.tracking.label}</span>
                          <ExternalLink className="size-3 ml-1" aria-hidden="true" />
                        </Link>
                      </Button>
                    </div>
                  )}
                </div>
              </div>
            ))}

            {pending && (
              <div className="flex flex-col gap-2.5">
                <p className="max-w-[85%] self-end whitespace-pre-wrap break-words rounded-md rounded-br-sm bg-primary px-3.5 py-2.5 text-sm text-primary-foreground">
                  <span className="sr-only">You: </span>
                  {draft.trim()}
                </p>

                {activeTool && (
                  <Badge
                    variant="active"
                    className="self-start text-xs py-1 px-2.5 rounded-sm border border-border"
                    data-testid="ai-active-tool-badge"
                  >
                    <span className="size-1.5 rounded-full bg-primary animate-pulse shrink-0" aria-hidden="true" />
                    <span>{activeTool}</span>
                  </Badge>
                )}

                {streamingReply ? (
                  <div className="max-w-[88%] self-start space-y-2 rounded-md rounded-bl-sm border border-border bg-surface px-3.5 py-2.5 text-sm text-foreground">
                    <p className="leading-relaxed">
                      <span className="sr-only">Assistant: </span>
                      <FormattedMessage text={streamingReply} />
                      <span className="inline-block w-1.5 h-3.5 bg-primary ml-0.5 animate-pulse align-middle" aria-hidden="true" />
                    </p>
                  </div>
                ) : (
                  <div className="self-start flex items-center gap-1.5 rounded-md rounded-bl-sm border border-border bg-surface px-3.5 py-2.5 text-xs text-text-muted">
                    <span className="size-1.5 rounded-full bg-primary animate-pulse" aria-hidden="true" />
                    <span className="size-1.5 rounded-full bg-primary animate-pulse" aria-hidden="true" />
                    <span className="size-1.5 rounded-full bg-primary animate-pulse" aria-hidden="true" />
                    <span className="ml-1 text-xs">Assistant is preparing response…</span>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Composer Footer */}
          <form onSubmit={send} className="shrink-0 space-y-2 border-t border-border bg-surface p-3">
            {error && (
              <div role="alert" className="flex items-center gap-2 rounded-sm border border-status-error-border bg-status-error-bg px-2.5 py-1.5 text-xs text-status-error-ink">
                <AlertCircle className="size-3.5 shrink-0" aria-hidden="true" />
                <span className="break-words">{error}</span>
              </div>
            )}
            <Label htmlFor="assistant-prompt" className="sr-only">Message</Label>
            <div className="flex items-end gap-2">
              <Textarea
                ref={composer}
                id="assistant-prompt"
                data-testid="assistant-prompt-input"
                value={draft}
                rows={1}
                maxLength={1000}
                className="min-h-10 max-h-24 resize-none rounded-sm border-border text-sm placeholder:text-text-muted"
                placeholder="Ask about tariffs, tracking, or booking…"
                onKeyDown={handleKeyDown}
                onChange={event => setDraft(event.target.value)}
                readOnly={pending}
              />
              <Button
                type="submit"
                size="icon"
                className="size-10 shrink-0 rounded-sm"
                aria-label="Send message"
                data-testid="assistant-send-btn"
                disabled={pending || !draft.trim()}
              >
                <Send className="size-4" aria-hidden="true" />
              </Button>
            </div>
            <p className="text-xs text-text-subtle text-center">
              Grounded in official tariffs &amp; real-time tracking records
            </p>
          </form>
        </section>
      )}
    </>
  );
}
