import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { Send, Sparkles } from 'lucide-react';
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
  }, [turns, pending]);

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

  return (
    <section aria-labelledby="assistant-chat-title" className="overflow-hidden rounded-lg border border-border bg-surface" data-testid="assistant-panel">
      <header className="flex items-center gap-2 border-b border-border px-4 py-3">
        <Sparkles className="size-4 text-primary" aria-hidden="true" />
        <h2 id="assistant-chat-title" className="text-lg font-semibold">SwiftDrop Assistant</h2>
      </header>
      <div ref={transcript} role="log" aria-label="Conversation" className="min-h-28 max-h-80 space-y-4 overflow-y-auto p-4">
        {!turns.length && <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" disabled={pending} onClick={() => setDraft('Can you help me estimate a delivery fee?')}>Estimate delivery cost</Button>
        </div>}
        {turns.map((turn, index) => <div key={index} className="space-y-3">
          <p className="ml-8 whitespace-pre-wrap break-words rounded-md bg-surface-subtle p-3 text-sm"><span className="sr-only">You: </span>{turn.prompt}</p>
          <div className="mr-8 space-y-2 rounded-md border border-border p-3 text-sm">
            <p className="whitespace-pre-wrap break-words"><span className="sr-only">Assistant: </span>{turn.reply.replace(/\*\*([^*]+)\*\*/g, '$1')}</p>
            {turn.booking && <Button asChild variant="outline"><Link to={turn.booking}>Continue to booking</Link></Button>}
          </div>
        </div>)}
        {pending && <p role="status" className="text-sm text-text-muted">Waiting for a reply…</p>}
      </div>
      <form onSubmit={send} className="space-y-2 border-t border-border p-4">
        {error && <p role="alert" className="break-words text-sm">{error}</p>}
        <Label htmlFor="assistant-prompt">Message</Label>
        <Textarea id="assistant-prompt" data-testid="assistant-prompt-input" value={draft} rows={2} maxLength={1000}
          onChange={event => setDraft(event.target.value)} disabled={pending} />
        <div className="flex justify-end">
          <Button type="submit" data-testid="assistant-send-btn" disabled={pending || !draft.trim()}><Send aria-hidden="true" /> Send</Button>
        </div>
      </form>
    </section>
  );
}
