import { Textarea } from '@/shared/components/ui/textarea';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { FileText, Package, Shirt } from 'lucide-react';
import { Link, useSearchParams } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import { Input } from '@/shared/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/shared/components/ui/dialog';
import { BookingZone } from './BookingZone';
import { Stepper } from '@/shared/components/ui/stepper';
import { NativeSelect } from '@/shared/components/ui/native-select';
import { Button } from '@/shared/components/ui/button';
import { errorMessage, PortalError, portalRequest } from './api';
import { PortalErrorMessage, PortalLayout } from './PortalLayout';

const steps = ['Addresses', 'Parcel', 'Quote', 'Confirm'];
const categories = [
  { label: 'Documents', Icon: FileText },
  { label: 'Clothing', Icon: Shirt },
  { label: 'Other goods', Icon: Package },
];
const contacts = [
  ['pickup', 'Sender'], ['dropoff', 'Recipient'],
] as const;
const fields = [
  ['name', 'Full name', 'text'], ['phone', 'Phone', 'tel'],
  ['address', 'Street address', 'text'], ['city', 'City', 'text'],
  ['postal', 'Postal code', 'text'],
] as const;
const parcelFields = [
  ['weight', 'Weight (kg)', 30], ['length', 'Length (cm)', 150],
  ['width', 'Width (cm)', 150], ['height', 'Height (cm)', 150],
] as const;
const addressAutocomplete = {
  name: 'name',
  phone: 'tel',
  address: 'address-line1',
  city: 'address-level2',
  postal: 'postal-code',
} as const;

type Quote = { quote_id: string; fee_cents: number; currency: string; expires_at: string };
type Booking = { id?: string; tracking_number: string; status: string; item_description?: string; special_instructions?: string };
type Receipt = {
  receipt_number: string;
  barcode?: string;
  financial_summary: { total_fee_formatted: string };
  payment_status: string;
};

export function BookingPage() {
  const { accessToken, user } = useAuth();
  const [params] = useSearchParams();
  const weight = Number(params.get('weight_kg'));
  const prefillDimension = (name: string) => {
    const dimension = Number(params.get(name + '_cm'));
    return dimension >= 0.01 && dimension <= 150 ? String(dimension) : '';
  };
  const [category, setCategory] = useState('');
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<Record<string, string>>(() => ({
    service: params.get('service_type') === 'EXPRESS' ? 'EXPRESS' : 'STANDARD',
    weight: weight >= 0.01 && weight <= 30 ? String(weight) : '',
    length: prefillDimension('length'),
    width: prefillDimension('width'),
    height: prefillDimension('height'),
    'pickup-postal': params.get('pickup_postal_code')?.match(/^\d{5}$/)?.[0] ?? '',
    'dropoff-postal': params.get('delivery_postal_code')?.match(/^\d{5}$/)?.[0] ?? '',
    'pickup-name': user?.display_name?.trim() || [user?.first_name, user?.last_name].filter(Boolean).join(' '),
    'pickup-phone': user?.phone ?? '',
  }));
  const [quote, setQuote] = useState<Quote & { pickupZone: string; dropoffZone: string }>();
  const [secondsRemaining, setSecondsRemaining] = useState<number>();
  useEffect(() => {
    if (!quote) return;
    const expiresAt = Date.parse(quote.expires_at);
    const tick = () => setSecondsRemaining(Math.max(0, Math.ceil((expiresAt - Date.now()) / 1000)));
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [quote]);
  const [booking, setBooking] = useState<Booking>();
  const [receipt, setReceipt] = useState<Receipt>();
  const [receiptPending, setReceiptPending] = useState(false);
  const [receiptError, setReceiptError] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const bookingKey = useRef<string>();
  const requestPending = useRef(false);
  const money = quote && new Intl.NumberFormat('en-LK', { style: 'currency', currency: quote.currency })
    .format(quote.fee_cents / 100);

  const value = (key: string) => draft[key]?.trim() ?? '';
  function change(key: string, input: string) {
    setDraft(previous => ({ ...previous, [key]: input, ...(key.endsWith('-postal') ? { [key + '-zone']: '' } : {}) }));
    if (['weight', 'length', 'width', 'height', 'service'].includes(key) || key.includes('-postal')) setQuote(undefined);
    setError('');
  }
  function contact(prefix: string, zoneId: string) {
    return {
      contact_name: value(prefix + '-name'),
      contact_phone: value(prefix + '-phone'),
      address_line1: value(prefix + '-address'),
      city: value(prefix + '-city'),
      postal_code: value(prefix + '-postal'),
      zone_id: zoneId,
    };
  }

  async function loadReceipt() {
    if (!booking?.id || !accessToken || receiptPending) return;
    setReceiptPending(true);
    setReceiptError('');
    try {
      setReceipt(undefined);
      const result = await portalRequest<Receipt | Blob>(`/parcels/${encodeURIComponent(booking.id)}/receipt`, {
        token: accessToken, acceptPdf: true, allowUnwrapped: true,
      });
      if (result instanceof Blob) {
        const url = URL.createObjectURL(result);
        const link = document.createElement('a');
        link.href = url;
        link.download = `swiftdrop-receipt-${booking.id}.pdf`;
        link.click();
        window.setTimeout(() => URL.revokeObjectURL(url), 30_000);
        return;
      }
      if (!result?.receipt_number || !result.financial_summary?.total_fee_formatted) {
        setReceiptError('The server did not return a complete receipt.');
        return;
      }
      setReceipt(result);
    } catch (cause) {
      setReceiptError(cause instanceof PortalError && cause.status === 404
        ? 'An official receipt is currently unavailable for this booking.'
        : errorMessage(cause));
    } finally {
      setReceiptPending(false);
    }
  }

  async function advance(event?: FormEvent<HTMLFormElement>, quoteOnly = false) {
    event?.preventDefault();
    if (requestPending.current) return;
    setError('');
    if (step === 0 && !quoteOnly) {
      if (contacts.some(([prefix]) => fields.some(([field]) => !value(prefix + '-' + field)))) {
        setError('Enter all sender and recipient details.');
        return;
      }
      setStep(1);
      return;
    }
    if (step === 2 && quote && Date.parse(quote.expires_at) > Date.now()) {
      const needsDetails = contacts.some(([prefix]) => fields.some(([field]) => !value(prefix + '-' + field))) || !value('description');
      setStep(needsDetails ? 0 : 3);
      return;
    }
    if (step === 1 && quote && Date.parse(quote.expires_at) > Date.now()) { setStep(2); return; }
    const weight = Number(value('weight'));
    const dimensions = {
      length_cm: Number(value('length')),
      width_cm: Number(value('width')),
      height_cm: Number(value('height')),
    };
    const serviceLevel = value('service');
    requestPending.current = true;
    setPending(true);
    try {
      if (step < 3) {
        const resolveZone = async (prefix: string, label: string) => {
          const selectedZone = value(prefix + '-postal-zone');
          if (selectedZone) return selectedZone;
          const postalCode = value(prefix + '-postal');
          try {
            const zone = await portalRequest<{ id: string }>(`/zones?postal_code=${encodeURIComponent(postalCode)}`);
            if (!zone?.id) throw new PortalError(`The service did not return a zone for the ${label} address.`, 200);
            return zone.id;
          } catch (cause) {
            throw new PortalError(`Couldn’t find a delivery zone for the ${label} address. ${errorMessage(cause)}`,
              cause instanceof PortalError ? cause.status : 0);
          }
        };
        const [pickupZone, dropoffZone] = await Promise.all([resolveZone('pickup', 'sender'), resolveZone('dropoff', 'recipient')]);
        const next = await portalRequest<Quote>('/pricing/quotes', {
          token: accessToken, method: 'POST',
          body: {
            origin_zone_id: pickupZone,
            destination_zone_id: dropoffZone,
            actual_weight_kg: weight,
            dimensions,
            service_level: serviceLevel,
          },
        });
        if (!next?.quote_id || !Number.isSafeInteger(next.fee_cents) || next.fee_cents < 0 || !/^[A-Z]{3}$/.test(next.currency)
          || !(Date.parse(next.expires_at) > Date.now())) {
          setError('The pricing service did not return a valid quote.');
          return;
        }
        setSecondsRemaining(Math.max(0, Math.ceil((Date.parse(next.expires_at) - Date.now()) / 1000)));
        setQuote({ ...next, pickupZone, dropoffZone });
        setStep(2);
      } else if (quote) {
        const pickup = contact('pickup', quote.pickupZone);
        const dropoff = contact('dropoff', quote.dropoffZone);
        if (!bookingKey.current && Date.parse(quote.expires_at) <= Date.now()) {
          setStep(2);
          setError('The quote has expired. Request a new quote to continue.');
          return;
        }
        bookingKey.current ??= crypto.randomUUID();
        const result = await portalRequest<Booking>('/parcels', {
          token: accessToken, method: 'POST', idempotencyKey: bookingKey.current,
          body: {
            quote_id: quote.quote_id,
            recipient_name: dropoff.contact_name,
            recipient_phone: dropoff.contact_phone,
            weight_kg: weight,
            dimensions,
            pickup,
            dropoff,
            item_description: value('description'),
            special_instructions: value('special-instructions'),
          },
        });
        if (!result?.tracking_number) {
          setError('No tracking number was returned. Retry this submission to check the booking.');
          return;
        }
        setBooking(result);
      }
    } catch (cause) {
      if (step === 3 && cause instanceof PortalError && [400, 422].includes(cause.status)) {
        bookingKey.current = undefined;
        if (quote && Date.parse(quote.expires_at) <= Date.now()) setStep(2);
      }
      setError(errorMessage(cause));
    } finally {
      requestPending.current = false;
      setPending(false);
    }
  }

  const assistantQuoteRequested = useRef(false);
  useEffect(() => {
    if (assistantQuoteRequested.current || step !== 0 || !accessToken) return;
    if (!params.has('pickup_postal_code') || !draft['pickup-postal'] || !draft['dropoff-postal']
        || parcelFields.some(([field]) => !draft[field])) return;
    assistantQuoteRequested.current = true;
    void advance(undefined, true);
  });

  return (
    <PortalLayout title="Book a parcel" profile="customer">
      {booking ? <section className="m6-card" aria-live="polite" data-testid="booking-confirmation">
        <h2>Booking confirmed</h2>
        <p>Tracking number <strong data-testid="booking-confirmation-tracking-no">{booking.tracking_number}</strong></p>
        <dl className="mt-4 grid gap-3">
          {booking.item_description && <div>
            <dt className="font-semibold">Parcel contents</dt>
            <dd className="whitespace-pre-wrap break-words">{booking.item_description}</dd>
          </div>}
          {booking.special_instructions && <div>
            <dt className="font-semibold">Special instructions</dt>
            <dd className="whitespace-pre-wrap break-words">{booking.special_instructions}</dd>
          </div>}
        </dl>
        <Link to={`/track/${encodeURIComponent(booking.tracking_number)}`}>Track this parcel</Link>
        {booking.id && <div className="m6-actions">
          <Button type="button" variant="outline" disabled={receiptPending} onClick={loadReceipt} data-testid="booking-receipt-btn">
            {receiptPending ? 'Loading receipt…' : receiptError ? 'Retry receipt' : 'View receipt'}
          </Button>
        </div>}
        <PortalErrorMessage message={receiptError} />
        {receipt && <Dialog open onOpenChange={open => { if (!open) setReceipt(undefined); }}>
          <DialogContent className="space-y-4 p-6" data-testid="booking-receipt">
          <DialogTitle>Booking receipt</DialogTitle>
          <DialogDescription>{booking.tracking_number}</DialogDescription>
          <dl>
            <dt>Receipt number</dt><dd>{receipt.receipt_number}</dd>
            <dt>Total</dt><dd>{receipt.financial_summary.total_fee_formatted}</dd>
            <dt>Payment status</dt><dd>{receipt.payment_status}</dd>
            {receipt.barcode && <><dt>Barcode value</dt><dd>{receipt.barcode}</dd></>}
          </dl>
          <Button type="button" variant="outline" onClick={() => window.print()}>Print receipt</Button>
          </DialogContent>
        </Dialog>}
      </section> : <>
        <Stepper steps={steps} current={step} />
        <form className="rounded-2xl border border-border bg-surface p-4 sm:p-6" onSubmit={event => { void advance(event); }} data-testid="booking-wizard">
          {step === 0 && <div className="grid gap-5 md:grid-cols-2">
            {contacts.map(([prefix, label]) => <fieldset key={prefix} disabled={pending} className="min-w-0 rounded-xl border border-border bg-surface-subtle px-4 pb-4 pt-2">
              <legend className="px-2 text-base font-semibold">{label === 'Sender' ? 'Pickup address' : 'Delivery address'}</legend>
              <div className="grid gap-3 sm:grid-cols-2">
              {fields.map(([field, name, type]) => {
                const key = prefix + '-' + field;
                return <label className={`space-y-1.5 text-sm font-medium ${field === 'address' ? 'sm:col-span-2' : ''}`} key={key}>
                  <span className="block">{name}</span>
                  <Input required type={type} name={key} value={draft[key] ?? ''} data-testid={`booking-${key}-input`}
                    placeholder={field === 'postal' ? 'e.g. 00100' : field === 'phone' ? 'e.g. +94771234567' : undefined}
                    autoComplete={`section-${prefix} ${addressAutocomplete[field]}`}
                    inputMode={field === 'postal' ? 'numeric' : undefined}
                    pattern={field === 'postal' ? '[0-9]{5}' : field === 'phone' ? '[+]?[0-9]{9,15}' : undefined}
                    maxLength={field === 'postal' ? 5 : undefined}
                    onChange={event => change(key, event.target.value)} />
                </label>;
              })}
              </div>
              <BookingZone postal={value(prefix + '-postal')} selected={value(prefix + '-postal-zone')}
                onSelect={id => change(prefix + '-postal-zone', id)} />
            </fieldset>)}
          </div>}
          {step === 1 && <fieldset className="m6-grid" disabled={pending}>
            <div className="col-span-full">
              <h2 className="mb-2 font-semibold">Package category</h2>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                {categories.map(({ label, Icon }) => <Button key={label} type="button"
                  className="justify-start" variant={category === label ? 'default' : 'outline'} aria-pressed={category === label}
                  onClick={() => { setCategory(label); change('description', label); }}>
                  <Icon aria-hidden="true" />{label}
                </Button>)}
              </div>
            </div>
            {parcelFields.map(([key, label, max]) => <label className="m6-field" key={key}>{label}
              <Input required type="number" min="0.01" max={max} step="0.01" value={draft[key] ?? ''}
                data-testid={`booking-${key}-input`} onChange={event => change(key, event.target.value)} />
            </label>)}
            <label className="m6-field">Description
              <Input required value={draft.description ?? ''} data-testid="booking-description-input"
                onChange={event => { setCategory(''); change('description', event.target.value); }} />
            </label>
            <label className="m6-field">Service
              <NativeSelect value={draft.service} data-testid="booking-service-select" onChange={event => change('service', event.target.value)}>
                <option value="STANDARD">Standard</option><option value="EXPRESS">Express</option>
              </NativeSelect>
            </label>
            <label className="m6-field col-span-full">Special instructions (optional)
              <Textarea value={draft['special-instructions'] ?? ''} data-testid="booking-special-instructions-input"
                placeholder="Add any handling or delivery instructions"
                onChange={event => change('special-instructions', event.target.value)} />
            </label>
          </fieldset>}
          {step === 2 && quote && <>
            <h2>Delivery quote</h2>
            {(!value('pickup-address') || !value('dropoff-address') || !value('description')) &&
              <p className="m6-muted">Continue to complete your addresses and parcel details.</p>}
            <p className="m6-quote-total" data-testid="quote-price-cents" data-amount-cents={quote.fee_cents}>{money}</p>
            {secondsRemaining !== undefined && secondsRemaining > 0 &&
              <p className="m6-muted" data-testid="booking-quote-countdown" role="timer">{Math.floor(secondsRemaining / 60)}:{String(secondsRemaining % 60).padStart(2, '0')} remaining</p>}
          </>}
          {step === 3 && <><h2>Review your booking</h2>
            <p>{value('pickup-name')} · {value('pickup-address')}, {value('pickup-city')}</p>
            <p>{value('dropoff-name')} · {value('dropoff-address')}, {value('dropoff-city')}</p>
            <p>{value('description')} · {value('weight')} kg · {money}</p>
            {value('special-instructions') && <p className="whitespace-pre-wrap break-words"><strong>Special instructions</strong> {value('special-instructions')}</p>}
            {bookingKey.current && error && <p>Retry this booking to reuse the same request and avoid a duplicate.</p>}
          </>}
          <PortalErrorMessage message={error} />
          <div className="mt-6 flex flex-wrap items-center justify-end gap-3 border-t border-border pt-4">
            {step > 0 && <Button variant="outline" type="button" data-testid="booking-back-btn"
              disabled={pending || Boolean(bookingKey.current)} onClick={() => { setStep(step - 1); setError(''); }}>Back</Button>}
            {quote && secondsRemaining === 0 && !bookingKey.current &&
              <p role="status">Quote expired. {step === 3 ? 'Go back to request a new quote.' : 'Request a new quote to continue.'}</p>}
            <Button disabled={pending || (step === 3 && secondsRemaining === 0 && !bookingKey.current)} data-testid={step === 3 ? 'booking-submit-btn' : 'booking-next-btn'}>
              {pending ? (step === 3 ? 'Confirming booking…' : 'Getting quote…') : step === 2 && quote && Date.parse(quote.expires_at) <= Date.now()
                ? 'Get a new quote' : ['Continue to parcel details', 'Get quote', 'Review booking', 'Confirm booking'][step]}
            </Button>
          </div>
        </form>
      </>}
    </PortalLayout>
  );
}
