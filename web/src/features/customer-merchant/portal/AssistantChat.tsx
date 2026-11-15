import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { MessageCircle, Send, Sparkles, X } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { errorMessage, portalRequest } from './api';

type BookingAction = { action_type: string; target_url: string; prefill_data?: Record<string, unknown> };
type ChatResponse = { conversation_id: string; reply: string; actions?: BookingAction[] };
type Turn = { prompt: string; reply: string; booking?: string };

function bookingLink(actions?: BookingAction[]) {
  const action = Array.isArray(actions) && actions.find(item =>
    item?.action_type === 'NAVIGATE_BOOKING' && ['/portal/book', '/customer/book'].includes(item.target_url));
  if (!action) return undefined;
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

export function AssistantChat({ token }: { token: string }) {
  const [open, setOpen] = useState(false);
  const launcher = useRef<HTMLButtonElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const [draft, setDraft] = useState('');
  const [turns, setTurns] = useState<Turn[]>([]);
  const [conversation, setConversation] = useState<string>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const request = useRef<AbortController>();
  const transcript = useRef<HTMLDivElement>(null);

  useEffect(() => () => request.current?.abort(), []);
  useEffect(() => {
    const element = transcript.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [turns, pending, open]);
  useEffect(() => { if (open) composer.current?.focus(); }, [open]);

  async function send(event: FormEvent) {
    event.preventDefault();
    const prompt = draft.trim();
    if (!prompt || request.current) return;
    const controller = new AbortController();
    request.current = controller;
    setError('');
    setPending(true);
    try {
      const response = await portalRequest<ChatResponse>('/ai/chat', {
        method: 'POST', token, signal: controller.signal, allowUnwrapped: true,
        body: { prompt, conversation_id: conversation },
      });
      if (!response?.reply || !response.conversation_id) throw new Error('Missing chat response');
      if (response.reply.startsWith('Unable to calculate shipping estimate:')) {
        setTurns(previous => [...previous, { prompt, reply: 'I could not get a delivery estimate here. You can request a quote in the booking form.', booking: '/portal/book' }]);
        setConversation(response.conversation_id);
        setDraft('');
        return;
      }
      setTurns(previous => [...previous, { prompt, reply: response.reply, booking: bookingLink(response.actions) }]);
      setConversation(response.conversation_id);
      setDraft('');
    } catch (cause) {
      if (!controller.signal.aborted) setError(errorMessage(cause));
    } finally {
      request.current = undefined;
      if (!controller.signal.aborted) setPending(false);
    }
  }

  function close() {
    setOpen(false);
    launcher.current?.focus();
  }

  return <>
    <Button ref={launcher} type="button" className="fixed bottom-5 right-4 z-40 min-h-12 rounded-full px-5 shadow-lg sm:right-6"
      aria-expanded={open} aria-controls={open ? 'assistant-chat' : undefined} data-testid="assistant-open-btn"
      onClick={() => { if (open) close(); else setOpen(true); }}>
      <MessageCircle aria-hidden="true" /> Assistant
    </Button>
    {open && <section id="assistant-chat" aria-labelledby="assistant-chat-title" data-testid="assistant-panel"
      onKeyDown={event => { if (event.key === 'Escape') close(); }}
      className="fixed inset-x-3 bottom-20 z-50 flex max-h-[min(36rem,calc(100dvh-6rem))] flex-col overflow-hidden rounded-2xl border border-border bg-surface shadow-xl sm:left-auto sm:right-6 sm:w-96">
      <header className="flex shrink-0 items-center gap-3 border-b border-border bg-primary-tint px-4 py-3">
        <span className="grid size-10 shrink-0 place-items-center rounded-full bg-primary text-primary-foreground">
          <Sparkles className="size-5" aria-hidden="true" />
        </span>
        <h2 id="assistant-chat-title" className="flex-1 text-base font-semibold">SwiftDrop Assistant</h2>
        <Button variant="ghost" size="icon" type="button" aria-label="Close assistant" onClick={close}><X aria-hidden="true" /></Button>
      </header>
      <div ref={transcript} role="log" aria-label="Conversation" className="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain bg-canvas p-4">
        {!turns.length && <div className="space-y-3">
          <p className="mr-8 rounded-2xl rounded-tl-sm border border-border bg-surface px-4 py-3 text-sm">
            Need help booking a parcel or estimating delivery cost?
          </p>
          <Button asChild variant="outline"><Link to="/portal/book">Estimate delivery cost</Link></Button>
        </div>}
        {turns.map((turn, index) => <div key={index} className="flex flex-col gap-3">
          <p className="max-w-[88%] self-end whitespace-pre-wrap break-words rounded-2xl rounded-tr-sm bg-primary px-4 py-3 text-sm text-primary-foreground"><span className="sr-only">You: </span>{turn.prompt}</p>
          <div className="max-w-[88%] self-start space-y-2 rounded-2xl rounded-tl-sm border border-border bg-surface px-4 py-3 text-sm">
            <p className="whitespace-pre-wrap break-words"><span className="sr-only">Assistant: </span>{turn.reply.replace(/\*\*([^*]+)\*\*/g, '$1')}</p>
            {turn.booking && <Button asChild variant="outline"><Link to={turn.booking}>Continue to booking</Link></Button>}
          </div>
        </div>)}
        {pending && <div className="flex flex-col gap-3">
          <p className="max-w-[88%] self-end whitespace-pre-wrap break-words rounded-2xl rounded-tr-sm bg-primary px-4 py-3 text-sm text-primary-foreground"><span className="sr-only">You: </span>{draft.trim()}</p>
          <p role="status" className="self-start rounded-2xl rounded-tl-sm border border-border bg-surface px-4 py-3 text-sm text-text-muted">Assistant is typing…</p>
        </div>}
      </div>
      <form onSubmit={send} className="shrink-0 space-y-2 border-t border-border bg-surface p-3">
        {error && <p role="alert" className="break-words text-sm text-status-error-ink">{error}</p>}
        <Label htmlFor="assistant-prompt" className="sr-only">Message</Label>
        <div className="flex items-end gap-2">
          <Textarea ref={composer} id="assistant-prompt" data-testid="assistant-prompt-input" value={draft} rows={2} maxLength={1000}
            className="min-h-11 resize-none rounded-xl" placeholder="Write a message…"
            onChange={event => setDraft(event.target.value)} readOnly={pending} />
          <Button type="submit" size="icon" className="size-11 shrink-0 rounded-full" aria-label="Send message" data-testid="assistant-send-btn"
            disabled={pending || !draft.trim()}><Send aria-hidden="true" /></Button>
        </div>
      </form>
    </section>}
  </>;
}
