import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Check, Package } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Badge } from '@/shared/components/ui/badge';

// Public landing page ("Waybill" design, web/design.md). The tracking card in the hero is a
// labelled example; real tracking happens on /track after the visitor submits a number.

const STEPS = [
  {
    title: 'Get a price',
    body: 'Enter the weight, size and both addresses. The price comes back before you commit, and it is held for 15 minutes.',
  },
  {
    title: 'A courier collects it',
    body: 'A dispatcher assigns a courier with space on their run. You get a message when the parcel is picked up.',
  },
  {
    title: 'Delivered, with proof',
    body: 'The courier records who received it and takes a photo. If nobody is home, we try again, up to three attempts.',
  },
];

const HUBS = [
  { code: 'W', name: 'Western Hub', detail: '3 dispatches daily' },
  { code: 'C', name: 'Central Hub', detail: '2 dispatches daily' },
  { code: 'S', name: 'Southern Hub', detail: '2 dispatches daily' },
  { code: 'EX', name: 'Express lines between hubs', detail: 'Next-day transit' },
];

const MERCHANT_POINTS = [
  'Upload a CSV of orders. Rows with problems are listed so you can fix them and retry.',
  'Create parcels straight from your shop with an API key.',
  'Get a webhook call each time a parcel changes status.',
];

function BrandMark() {
  return (
    <span className="grid size-8 place-items-center rounded-sm bg-chrome text-primary">
      <Package size={17} strokeWidth={2} aria-hidden="true" />
    </span>
  );
}

export const LandingPage: React.FC = () => {
  const navigate = useNavigate();
  const [trackingNumber, setTrackingNumber] = useState('');

  const handleTrackSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = trackingNumber.trim().toUpperCase();
    navigate(trimmed ? `/track/${encodeURIComponent(trimmed)}` : '/track');
  };

  return (
    <div data-role="PUBLIC" className="min-h-screen bg-canvas text-text">
      <header className="border-b border-border px-4 py-4 sm:px-8 lg:px-14">
        <div className="mx-auto flex max-w-[1600px] flex-wrap items-center justify-between gap-4">
          <Link to="/" className="flex items-center gap-2.5 font-display text-xl font-bold tracking-tight text-text" aria-label="SwiftDrop home">
            <BrandMark />
            SwiftDrop
          </Link>
          <nav aria-label="Main" className="hidden items-center gap-8 text-sm font-medium text-text-muted md:flex">
            <Link to="/track" className="hover:text-text">Track a parcel</Link>
            <Link to="/portal/book" className="hover:text-text">Send a parcel</Link>
            <a href="#merchants" className="hover:text-text">For merchants</a>
            <a href="#coverage" className="hover:text-text">Where we deliver</a>
          </nav>
          <div className="flex items-center gap-3">
            <Button asChild variant="outline">
              <Link to="/auth/login">Sign in</Link>
            </Button>
            <Button asChild>
              <Link to="/portal/book">Book a pickup</Link>
            </Button>
          </div>
        </div>
      </header>

      <main>
        <section className="px-4 py-16 sm:px-8 lg:px-14 lg:py-24">
          <div className="mx-auto grid max-w-[1600px] items-center gap-12 lg:grid-cols-[1.1fr_0.9fr] lg:gap-16">
            <div className="flex flex-col gap-7">
              <p className="font-mono text-sm font-medium tracking-[0.04em] text-primary-hover">
                ISLAND-WIDE PARCEL DELIVERY · SRI LANKA
              </p>
              <h1 className="font-display text-5xl font-bold leading-none tracking-tighter md:text-6xl lg:text-7xl">
                Book a pickup in minutes. Follow it to the door.
              </h1>
              <p className="max-w-xl text-lg leading-relaxed text-text-muted">
                See the price before you book, hand the parcel to a courier at your door, and watch every step until it is delivered.
              </p>
              <div className="flex flex-wrap gap-3">
                <Button asChild size="lg">
                  <Link to="/portal/book">Book a pickup</Link>
                </Button>
                <Button asChild size="lg" variant="outline">
                  <Link to="/portal/merchants/apply">Apply as a merchant</Link>
                </Button>
              </div>
            </div>

            <section aria-labelledby="track-title" className="overflow-hidden rounded-md border border-text bg-surface">
              <div className="flex items-center justify-between bg-chrome px-5 py-3.5 text-chrome-foreground">
                <h2 id="track-title" className="font-mono text-sm font-semibold tracking-[0.06em]">TRACK A PARCEL</h2>
                <span className="font-mono text-xs text-chrome-muted">NO SIGN-IN NEEDED</span>
              </div>
              <form onSubmit={handleTrackSubmit} className="grid gap-2 border-b border-dashed border-border-strong px-5 py-6" role="search" aria-label="Track a parcel">
                <label htmlFor="landing-tracking" className="text-sm font-semibold">Tracking number</label>
                <div className="flex gap-2.5">
                  <Input
                    id="landing-tracking"
                    value={trackingNumber}
                    onChange={(e) => setTrackingNumber(e.target.value)}
                    placeholder="SD-20261111-000001"
                    autoComplete="off"
                    className="h-12 font-mono text-base"
                  />
                  <Button type="submit" size="lg" className="h-12">Track</Button>
                </div>
              </form>
              <div className="grid gap-4 px-5 py-5" aria-label="Example tracking result">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <span className="flex items-center gap-2">
                    <Badge variant="neutral" showDot={false}>Example</Badge>
                    <span className="font-mono text-sm font-medium">SD-20261111-000001</span>
                  </span>
                  <Badge variant="active">Out for delivery</Badge>
                </div>
                <div className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3">
                  <div>
                    <div className="text-xs text-text-muted">From</div>
                    <div className="font-display text-lg font-semibold">Colombo</div>
                  </div>
                  <div className="waybill-route" aria-hidden="true">
                    <span className="waybill-stop" />
                    <span className="waybill-leg" />
                    <span className="waybill-stop" />
                    <span className="waybill-leg waybill-leg--ahead" />
                    <span className="waybill-stop waybill-stop--next" />
                  </div>
                  <div className="text-right">
                    <div className="text-xs text-text-muted">To</div>
                    <div className="font-display text-lg font-semibold">Kandy</div>
                  </div>
                </div>
                <div className="waybill-barcode" aria-hidden="true" />
                <p className="text-sm text-text-muted">
                  Public tracking shows the status, time and city only. Names, phone numbers and addresses stay private.
                </p>
              </div>
            </section>
          </div>
        </section>

        <section className="border-y border-border bg-surface px-4 py-20 sm:px-8 lg:px-14">
          <div className="mx-auto flex max-w-[1600px] flex-col gap-14">
            <div className="flex flex-wrap items-end justify-between gap-6">
              <h2 className="max-w-2xl font-display text-4xl font-bold leading-tight tracking-tight lg:text-5xl">
                From your door to theirs in three steps
              </h2>
              <p className="max-w-md text-lg leading-relaxed text-text-muted">
                Every parcel follows the same route. You always know which step it is on.
              </p>
            </div>
            <ol className="grid gap-10 md:grid-cols-3 md:gap-0">
              {STEPS.map((step, index) => (
                <li key={step.title} className="flex flex-col gap-3.5 md:pr-10">
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-sm font-medium text-primary-hover">{String(index + 1).padStart(2, '0')}</span>
                    <span className={index === STEPS.length - 1 ? 'flex-1 border-t-2 border-dashed border-border-strong' : 'flex-1 border-t-2 border-text'} />
                  </div>
                  <h3 className="font-display text-2xl font-semibold">{step.title}</h3>
                  <p className="leading-relaxed text-text-muted">{step.body}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="coverage" className="bg-chrome px-4 py-20 text-chrome-foreground sm:px-8 lg:px-14">
          <div className="mx-auto grid max-w-[1600px] gap-12 lg:grid-cols-[0.8fr_1.2fr] lg:gap-16">
            <div className="flex flex-col gap-5">
              <p className="font-mono text-sm font-medium tracking-[0.06em] text-chrome-accent">WHERE WE DELIVER</p>
              <h2 className="font-display text-4xl font-bold leading-tight tracking-tight lg:text-5xl">Three hubs, linked by daily runs</h2>
              <p className="text-lg leading-relaxed text-chrome-muted">
                Parcels are sorted at the hub nearest the sender, then moved to the hub nearest the recipient. Enter a postal code when you book and we match it to a delivery zone.
              </p>
            </div>
            <ul className="flex flex-col border-b border-chrome-line">
              {HUBS.map((hub) => (
                <li key={hub.code} className="grid grid-cols-[3rem_minmax(0,1fr)_auto] items-center gap-4 border-t border-chrome-line py-5">
                  <span className={hub.code === 'EX' ? 'font-mono text-sm text-chrome-accent' : 'font-mono text-sm text-chrome-muted'}>{hub.code}</span>
                  <span className="font-display text-xl font-semibold">{hub.name}</span>
                  <span className="font-mono text-sm text-chrome-muted">{hub.detail}</span>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section id="merchants" className="px-4 py-20 sm:px-8 lg:px-14">
          <div className="mx-auto grid max-w-[1600px] items-center gap-12 lg:grid-cols-2 lg:gap-16">
            <div className="flex flex-col gap-6">
              <p className="font-mono text-sm font-medium tracking-[0.06em] text-primary-hover">FOR ONLINE SELLERS</p>
              <h2 className="font-display text-4xl font-bold leading-tight tracking-tight lg:text-5xl">Ship your orders without retyping them</h2>
              <ul className="flex flex-col gap-4 text-lg leading-snug">
                {MERCHANT_POINTS.map((point) => (
                  <li key={point} className="flex gap-3">
                    <Check size={22} strokeWidth={2} className="mt-0.5 shrink-0 text-primary" aria-hidden="true" />
                    {point}
                  </li>
                ))}
              </ul>
              <div>
                <Button asChild size="lg">
                  <Link to="/portal/merchants/apply">Apply for a merchant account</Link>
                </Button>
              </div>
            </div>
            <img
              src="/merchant-fulfillment-hero.jpg"
              alt="Loading docks at a parcel fulfillment center"
              className="aspect-video w-full rounded-md object-cover"
              loading="lazy"
            />
          </div>
        </section>
      </main>

      <footer className="bg-chrome px-4 pb-10 pt-14 text-chrome-muted sm:px-8 lg:px-14">
        <div className="mx-auto flex max-w-[1600px] flex-col gap-10">
          <div className="flex flex-wrap justify-between gap-10">
            <span className="font-display text-xl font-bold text-chrome-foreground">SwiftDrop</span>
            <div className="flex flex-wrap gap-16 text-sm">
              <nav aria-label="Send" className="flex flex-col gap-3">
                <span className="font-mono text-xs font-semibold tracking-[0.06em]">SEND</span>
                <Link to="/portal/book" className="hover:text-chrome-foreground">Book a pickup</Link>
                <Link to="/track" className="hover:text-chrome-foreground">Track a parcel</Link>
              </nav>
              <nav aria-label="Merchants" className="flex flex-col gap-3">
                <span className="font-mono text-xs font-semibold tracking-[0.06em]">MERCHANTS</span>
                <Link to="/portal/merchants/apply" className="hover:text-chrome-foreground">Apply</Link>
                <Link to="/auth/login" className="hover:text-chrome-foreground">Merchant sign-in</Link>
              </nav>
              <nav aria-label="Staff" className="flex flex-col gap-3">
                <span className="font-mono text-xs font-semibold tracking-[0.06em]">STAFF</span>
                <Link to="/auth/login" className="hover:text-chrome-foreground">Operations sign-in</Link>
              </nav>
            </div>
          </div>
          <p className="border-t border-chrome-line pt-6 text-sm">© 2026 SwiftDrop</p>
        </div>
      </footer>
    </div>
  );
};
