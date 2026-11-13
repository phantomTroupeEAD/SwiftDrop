import { useRef, useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useAuth } from '@/app/providers/AuthContext';
import { Input } from '@/shared/components/ui/input';
import { Button } from '@/shared/components/ui/button';
import { errorMessage, PortalError, portalRequest } from './api';
import { PortalErrorMessage, PortalLayout } from './PortalLayout';

const steps = ['Addresses', 'Parcel', 'Quote', 'Confirm'];
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
type Booking = { id?: string; tracking_number: string; status: string };
type Receipt = {
  receipt_number: string;
  financial_summary: { total_fee_formatted: string };
  payment_status: string;
};

export function BookingPage() {
  const { accessToken, user } = useAuth();
  const [params] = useSearchParams();
  const weight = Number(params.get('weight_kg'));
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<Record<string, string>>(() => ({
    service: params.get('service_type') === 'EXPRESS' ? 'EXPRESS' : 'STANDARD',
    weight: weight >= 0.01 && weight <= 30 ? String(weight) : '',
    'pickup-postal': params.get('pickup_postal_code')?.match(/^\d{5}$/)?.[0] ?? '',
    'dropoff-postal': params.get('delivery_postal_code')?.match(/^\d{5}$/)?.[0] ?? '',
    'pickup-name': user?.display_name?.trim() || [user?.first_name, user?.last_name].filter(Boolean).join(' '),
    'pickup-phone': user?.phone ?? '',
  }));
  const [quote, setQuote] = useState<Quote & { pickupZone: string; dropoffZone: string }>();
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
    setDraft(previous => ({ ...previous, [key]: input }));
    setQuote(undefined);
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
      const result = await portalRequest<Receipt>(`/parcels/${encodeURIComponent(booking.id)}/receipt`, {
        token: accessToken,
      });
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

  async function advance(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (requestPending.current) return;
    setError('');
    if (step === 0) {
      if (contacts.some(([prefix]) => fields.some(([field]) => !value(prefix + '-' + field)))) {
        setError('Enter all sender and recipient details.');
        return;
      }
      setStep(1);
      return;
    }
    if (step === 2 && quote && Date.parse(quote.expires_at) > Date.now()) {
      setStep(3);
      return;
    }
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

  return (
    <PortalLayout title="Book a parcel" profile="customer">
      {booking ? <section className="m6-card" aria-live="polite" data-testid="booking-confirmation">
        <h2>Booking confirmed</h2>
        <p>Tracking number <strong data-testid="booking-confirmation-tracking-no">{booking.tracking_number}</strong></p>
        <Link to={`/track/${encodeURIComponent(booking.tracking_number)}`}>Track this parcel</Link>
        {booking.id && !receipt && <div className="m6-actions">
          <Button type="button" variant="outline" disabled={receiptPending} onClick={loadReceipt} data-testid="booking-receipt-btn">
            {receiptPending ? 'Loading receipt…' : receiptError ? 'Retry receipt' : 'View receipt'}
          </Button>
        </div>}
        <PortalErrorMessage message={receiptError} />
        {receipt && <section aria-label="Booking receipt" data-testid="booking-receipt">
          <h3>Booking receipt</h3>
          <dl>
            <dt>Receipt number</dt><dd>{receipt.receipt_number}</dd>
            <dt>Total</dt><dd>{receipt.financial_summary.total_fee_formatted}</dd>
            <dt>Payment status</dt><dd>{receipt.payment_status}</dd>
          </dl>
        </section>}
      </section> : <>
          <ol className="m6-steps" aria-label="Booking progress">
          {steps.map((label, index) => <li key={label} aria-current={step === index ? 'step' : undefined}
            data-completed={index < step}>{index + 1}. {label}</li>)}
        </ol>
        <form className="m6-card" onSubmit={advance} data-testid="booking-wizard">
          {step === 0 && <div className="m6-grid">
            {contacts.map(([prefix, label]) => <fieldset key={prefix}>
              <legend><strong>{label}</strong></legend>
              {fields.map(([field, name, type]) => {
                const key = prefix + '-' + field;
                return <label className="m6-field" key={key}>{name}
                  <Input required type={type} name={key} value={draft[key] ?? ''} data-testid={`booking-${key}-input`}
                    autoComplete={`section-${prefix} ${addressAutocomplete[field]}`}
                    inputMode={field === 'postal' ? 'numeric' : undefined}
                    pattern={field === 'postal' ? '[0-9]{5}' : undefined}
                    onChange={event => change(key, event.target.value)} />
                </label>;
              })}
            </fieldset>)}
          </div>}
          {step === 1 && <fieldset className="m6-grid" disabled={pending}>
            {parcelFields.map(([key, label, max]) => <label className="m6-field" key={key}>{label}
              <input required type="number" min="0.01" max={max} step="0.01" value={draft[key] ?? ''}
                data-testid={`booking-${key}-input`} onChange={event => change(key, event.target.value)} />
            </label>)}
            <label className="m6-field">Description
              <input required value={draft.description ?? ''} data-testid="booking-description-input"
                onChange={event => change('description', event.target.value)} />
            </label>
            <label className="m6-field">Service
              <select value={draft.service} data-testid="booking-service-select" onChange={event => change('service', event.target.value)}>
                <option value="STANDARD">Standard</option><option value="EXPRESS">Express</option>
              </select>
            </label>
          </fieldset>}
          {step === 2 && quote && <>
            <h2>Delivery quote</h2>
            <p className="m6-quote-total" data-testid="quote-price-cents" data-amount-cents={quote.fee_cents}>{money}</p>
            <p className="m6-muted">Valid until {new Date(quote.expires_at).toLocaleString()}</p>
          </>}
          {step === 3 && <><h2>Review your booking</h2>
            <p>{value('pickup-name')} · {value('pickup-address')}, {value('pickup-city')}</p>
            <p>{value('dropoff-name')} · {value('dropoff-address')}, {value('dropoff-city')}</p>
            <p>{value('description')} · {value('weight')} kg · {money}</p>
            {bookingKey.current && error && <p>Retry this booking to reuse the same request and avoid a duplicate.</p>}
          </>}
          <PortalErrorMessage message={error} />
          <div className="m6-actions">
            {step > 0 && <button className="m6-btn m6-secondary" type="button" data-testid="booking-back-btn"
              disabled={pending || Boolean(bookingKey.current)} onClick={() => { setStep(step - 1); setError(''); }}>Back</button>}
            <button className="m6-btn" disabled={pending} data-testid={step === 3 ? 'booking-submit-btn' : 'booking-next-btn'}>
              {pending ? (step === 3 ? 'Confirming booking…' : 'Getting quote…') : step === 2 && quote && Date.parse(quote.expires_at) <= Date.now()
                ? 'Get a new quote' : ['Continue', 'Get quote', 'Continue', 'Confirm booking'][step]}
            </button>
          </div>
        </form>
      </>}
    </PortalLayout>
  );
}
