import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  Package,
  Search,
  ArrowRight,
  Truck,
  ShieldCheck,
  Sparkles,
  CheckCircle2,
  Zap,
  FileCheck2,
} from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Badge } from '@/shared/components/ui/badge';
import { Separator } from '@/shared/components/ui/separator';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
} from '@/shared/components/ui/card';
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from '@/shared/components/ui/tabs';

export const LandingPage: React.FC = () => {
  const navigate = useNavigate();
  const [trackingNumber, setTrackingNumber] = useState('');

  const handleTrackSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = trackingNumber.trim().toUpperCase();
    if (trimmed) {
      navigate(`/track/${encodeURIComponent(trimmed)}`);
    } else {
      navigate('/track');
    }
  };

  return (
    <main data-role="PUBLIC" className="min-h-screen bg-canvas text-text flex flex-col antialiased selection:bg-primary-tint selection:text-primary">
      {/* 1. Sticky Navigation Topbar */}
      <header className="sticky top-0 z-40 border-b border-border bg-surface/85 backdrop-blur-md px-4 sm:px-8 py-3.5 transition-colors">
        <div className="mx-auto flex max-w-6xl items-center justify-between">
          <Link
            to="/"
            className="inline-flex items-center gap-2.5 font-semibold tracking-tight text-text hover:opacity-90 transition-opacity"
            aria-label="SwiftDrop Home"
          >
            <div className="flex size-8 items-center justify-center rounded-lg bg-primary-tint text-primary border border-border">
              <Package className="size-4" aria-hidden="true" />
            </div>
            <span className="text-lg font-bold">
              Swift<span className="text-primary">Drop</span>
            </span>
            <Badge variant="neutral" className="hidden sm:inline-flex ml-1.5 text-xs">
              COURIER PLATFORM
            </Badge>
          </Link>

          <nav className="hidden md:flex items-center gap-6 text-xs font-medium text-text-muted" aria-label="Main Navigation">
            <a href="#services" className="hover:text-text transition-colors">Services</a>
            <a href="#tracking" className="hover:text-text transition-colors">Tracking</a>
            <a href="#solutions" className="hover:text-text transition-colors">Solutions</a>
            <a href="#coverage" className="hover:text-text transition-colors">Coverage</a>
          </nav>

          <div className="flex items-center gap-2 sm:gap-3">
            <Button asChild variant="ghost" size="sm" className="text-xs">
              <Link to="/track">Track a parcel</Link>
            </Button>
            <Button asChild variant="default" size="sm" className="text-xs">
              <Link to="/auth/login">Sign in</Link>
            </Button>
          </div>
        </div>
      </header>

      {/* 2. Cinematic Hero Section with Instant Tracking Utility */}
      <section className="relative overflow-hidden border-b border-border py-16 sm:py-24 lg:py-32">
        {/* Full-width High-Impact Hero Background Artwork */}
        <div className="absolute inset-0 pointer-events-none select-none overflow-hidden" aria-hidden="true">
          <img
            src="/login-bg.jpg"
            alt=""
            className="h-full w-full object-cover object-center filter blur-sm scale-105"
          />
          <div className="absolute inset-0 bg-canvas/80 backdrop-blur-xs" />
        </div>

        <div className="relative z-10 mx-auto max-w-5xl px-4 sm:px-8 text-center space-y-6">
          <div className="flex justify-center">
            <Badge variant="active" className="px-3 py-1 text-xs">
              <Sparkles className="mr-1.5 size-3.5 text-primary" aria-hidden="true" />
              Next-Day Courier Delivery Across Sri Lanka
            </Badge>
          </div>

          <h1 className="text-3xl sm:text-5xl lg:text-6xl font-bold tracking-tight text-text max-w-4xl mx-auto leading-tight">
            Precision Parcel Delivery. Tracked to the Second.
          </h1>

          <p className="text-sm sm:text-base text-text-muted max-w-2xl mx-auto leading-relaxed">
            Doorstep pickup, automated zone routing, and immutable milestone projections. Designed for individual senders and enterprise B2B merchants.
          </p>

          {/* Instant Hero Tracking Widget */}
          <div id="tracking" className="pt-2 max-w-xl mx-auto">
            <Card className="p-2 border border-border bg-surface rounded-xl">
              <form onSubmit={handleTrackSubmit} className="flex flex-col sm:flex-row items-center gap-2">
                <div className="relative w-full flex-1">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-text-muted" aria-hidden="true" />
                  <Input
                    type="text"
                    placeholder="Enter tracking number (e.g. SD-202611-99801)"
                    value={trackingNumber}
                    onChange={(e) => setTrackingNumber(e.target.value)}
                    className="pl-9 h-10 border-0 bg-transparent shadow-none focus-visible:ring-0 text-xs sm:text-sm"
                  />
                </div>
                <Button type="submit" size="default" className="w-full sm:w-auto h-10 px-5 text-xs font-semibold shrink-0">
                  Track Shipment
                  <ArrowRight className="ml-1.5 size-3.5" aria-hidden="true" />
                </Button>
              </form>
            </Card>
            <p className="mt-2 text-xs text-text-muted flex items-center justify-center gap-1.5">
              <ShieldCheck className="size-3.5 text-primary" aria-hidden="true" />
              <span>Public lookup displays sanitized, privacy-safe milestone updates</span>
            </p>
          </div>
        </div>
      </section>

      {/* 3. Proof & Performance Metric Strip */}
      <section className="border-b border-border bg-surface-subtle py-8">
        <div className="mx-auto max-w-6xl px-4 sm:px-8">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
            <div className="space-y-1">
              <p className="text-2xl sm:text-3xl font-bold font-mono tabular-nums text-text">99.4%</p>
              <p className="text-xs text-text-muted font-medium">On-Time Delivery SLA</p>
            </div>
            <div className="space-y-1">
              <p className="text-2xl sm:text-3xl font-bold font-mono tabular-nums text-text">&lt; 24h</p>
              <p className="text-xs text-text-muted font-medium">Intra-Zone Turnaround</p>
            </div>
            <div className="space-y-1">
              <p className="text-2xl sm:text-3xl font-bold font-mono tabular-nums text-text">25+ Zones</p>
              <p className="text-xs text-text-muted font-medium">Automated Routing Hubs</p>
            </div>
            <div className="space-y-1">
              <p className="text-2xl sm:text-3xl font-bold font-mono tabular-nums text-text">LKR 250</p>
              <p className="text-xs text-text-muted font-medium">Standard Base Tariff</p>
            </div>
          </div>
        </div>
      </section>

      {/* 4. Photographic Feature Grid (Image-Heavy Showcase) */}
      <section id="services" className="py-16 sm:py-24 border-b border-border bg-canvas">
        <div className="mx-auto max-w-6xl px-4 sm:px-8 space-y-12">
          <div className="text-center max-w-2xl mx-auto space-y-2">
            <Badge variant="neutral" className="text-xs">LOGISTICS EXCELLENCE</Badge>
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-text">
              Engineered for Speed, Certainty, and Trust
            </h2>
            <p className="text-xs sm:text-sm text-text-muted leading-relaxed">
              Every package moves through an authoritative state machine, from doorstep collection to signed recipient delivery.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* Card 1: Courier Fleet */}
            <Card className="border border-border bg-surface rounded-xl overflow-hidden flex flex-col justify-between">
              <div className="aspect-4/3 w-full bg-surface-subtle overflow-hidden relative">
                <img
                  src="/feature-courier-fleet.jpg"
                  alt="SwiftDrop electric courier fleet"
                  className="h-full w-full object-cover object-center transition-transform duration-500 hover:scale-105"
                  onError={(e) => {
                    // Fallback to hero asset if image generation is pending
                    (e.target as HTMLImageElement).src = '/login-bg.jpg';
                  }}
                />
                <Badge variant="active" className="absolute top-3 left-3 text-xs bg-surface/90 backdrop-blur-sm">
                  <Truck className="size-3 mr-1" /> FLEET DISPATCH
                </Badge>
              </div>
              <CardHeader className="space-y-1.5 p-5">
                <CardTitle className="text-base font-semibold text-text">
                  Rapid Doorstep Collection
                </CardTitle>
                <CardDescription className="text-xs text-text-muted leading-relaxed">
                  Scheduled courier pickup windows across Colombo and Kandy with direct contact and live ETA updates.
                </CardDescription>
              </CardHeader>
            </Card>

            {/* Card 2: Automated Sorting */}
            <Card className="border border-border bg-surface rounded-xl overflow-hidden flex flex-col justify-between">
              <div className="aspect-4/3 w-full bg-surface-subtle overflow-hidden relative">
                <img
                  src="/feature-smart-sorting.jpg"
                  alt="Automated barcode parcel sorting rail"
                  className="h-full w-full object-cover object-center transition-transform duration-500 hover:scale-105"
                  onError={(e) => {
                    (e.target as HTMLImageElement).src = '/login-bg.jpg';
                  }}
                />
                <Badge variant="active" className="absolute top-3 left-3 text-xs bg-surface/90 backdrop-blur-sm">
                  <Zap className="size-3 mr-1" /> SMART ROUTING
                </Badge>
              </div>
              <CardHeader className="space-y-1.5 p-5">
                <CardTitle className="text-base font-semibold text-text">
                  Automated Sorting Hubs
                </CardTitle>
                <CardDescription className="text-xs text-text-muted leading-relaxed">
                  Laser dimensional scanning and barcode sorting assign shipments to optimal courier zones without delay.
                </CardDescription>
              </CardHeader>
            </Card>

            {/* Card 3: Proof of Delivery */}
            <Card className="border border-border bg-surface rounded-xl overflow-hidden flex flex-col justify-between">
              <div className="aspect-4/3 w-full bg-surface-subtle overflow-hidden relative">
                <img
                  src="/feature-proof-delivery.jpg"
                  alt="Digital proof of delivery on handheld device"
                  className="h-full w-full object-cover object-center transition-transform duration-500 hover:scale-105"
                  onError={(e) => {
                    (e.target as HTMLImageElement).src = '/login-bg.jpg';
                  }}
                />
                <Badge variant="active" className="absolute top-3 left-3 text-xs bg-surface/90 backdrop-blur-sm">
                  <FileCheck2 className="size-3 mr-1" /> VERIFIED POD
                </Badge>
              </div>
              <CardHeader className="space-y-1.5 p-5">
                <CardTitle className="text-base font-semibold text-text">
                  Proof of Delivery
                </CardTitle>
                <CardDescription className="text-xs text-text-muted leading-relaxed">
                  Cryptographically timestamped recipient signatures and photo evidence attached directly to the aggregate record.
                </CardDescription>
              </CardHeader>
            </Card>
          </div>
        </div>
      </section>

      {/* 5. The Two Operating Engines (Customer vs Merchant Tabbed Solutions) */}
      <section id="solutions" className="py-16 sm:py-24 border-b border-border bg-surface">
        <div className="mx-auto max-w-5xl px-4 sm:px-8 space-y-10">
          <div className="text-center space-y-2">
            <Badge variant="neutral" className="text-xs">ONE UNIFIED SYSTEM</Badge>
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-text">
              Tailored Experiences for Senders and Commercial Retailers
            </h2>
          </div>

          <Tabs defaultValue="customers" className="w-full space-y-6">
            <div className="flex justify-center">
              <TabsList className="grid w-full max-w-md grid-cols-2">
                <TabsTrigger value="customers" className="text-xs font-semibold">
                  Personal Senders
                </TabsTrigger>
                <TabsTrigger value="merchants" className="text-xs font-semibold">
                  B2B Merchants
                </TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="customers">
              <Card className="border border-border bg-surface-subtle p-6 sm:p-8 rounded-xl">
                <div className="grid md:grid-cols-2 gap-8 items-center">
                  <div className="space-y-4">
                    <h3 className="text-xl font-bold text-text">Consumer Doorstep Booking</h3>
                    <p className="text-xs text-text-muted leading-relaxed">
                      Send packages from your home or office in 4 simple steps. Enjoy transparent tariff estimation, automatic address completion, and printable receipts.
                    </p>
                    <ul className="space-y-2.5 text-xs text-text">
                      <li className="flex items-center gap-2">
                        <CheckCircle2 className="size-4 text-primary shrink-0" />
                        <span>Instant fee calculation based on real distance & parcel weight</span>
                      </li>
                      <li className="flex items-center gap-2">
                        <CheckCircle2 className="size-4 text-primary shrink-0" />
                        <span>Real-time SSE live milestone updates sent to your phone</span>
                      </li>
                      <li className="flex items-center gap-2">
                        <CheckCircle2 className="size-4 text-primary shrink-0" />
                        <span>Standard and Same-Day priority delivery tiers</span>
                      </li>
                    </ul>
                    <Button asChild size="default" className="mt-2 text-xs">
                      <Link to="/portal/book">
                        Book a parcel now
                        <ArrowRight className="ml-1.5 size-3.5" />
                      </Link>
                    </Button>
                  </div>

                  <Card className="border border-border bg-surface p-5 rounded-xl space-y-3">
                    <div className="flex items-center justify-between border-b border-border pb-2.5">
                      <span className="text-xs font-semibold text-text">Sample Rate Breakdown</span>
                      <Badge variant="neutral">STANDARD TIER</Badge>
                    </div>
                    <div className="space-y-1.5 text-xs">
                      <div className="flex justify-between text-text-muted">
                        <span>Base Pickup Fee</span>
                        <span className="font-mono tabular-nums text-text">LKR 250.00</span>
                      </div>
                      <div className="flex justify-between text-text-muted">
                        <span>Weight Surcharge (2.5 kg)</span>
                        <span className="font-mono tabular-nums text-text">LKR 300.00</span>
                      </div>
                      <Separator />
                      <div className="flex justify-between font-semibold text-text pt-1">
                        <span>Total Estimated Cost</span>
                        <span className="font-mono tabular-nums text-primary text-sm">LKR 550.00</span>
                      </div>
                    </div>
                  </Card>
                </div>
              </Card>
            </TabsContent>

            <TabsContent value="merchants">
              <Card className="border border-border bg-surface-subtle p-6 sm:p-8 rounded-xl">
                <div className="grid md:grid-cols-2 gap-8 items-center">
                  <div className="space-y-4">
                    <h3 className="text-xl font-bold text-text">B2B Shipping & Order Ingestion</h3>
                    <p className="text-xs text-text-muted leading-relaxed">
                      Connect your e-commerce storefront or ERP directly. Bulk ingest hundreds of orders via CSV or programmatically create shipments via REST API keys.
                    </p>
                    <ul className="space-y-2.5 text-xs text-text">
                      <li className="flex items-center gap-2">
                        <CheckCircle2 className="size-4 text-primary shrink-0" />
                        <span>CSV batch upload with per-row validation and retry handles</span>
                      </li>
                      <li className="flex items-center gap-2">
                        <CheckCircle2 className="size-4 text-primary shrink-0" />
                        <span>Live webhook events dispatched on status transitions</span>
                      </li>
                      <li className="flex items-center gap-2">
                        <CheckCircle2 className="size-4 text-primary shrink-0" />
                        <span>Multi-tenant isolation guaranteeing zero cross-tenant leakage</span>
                      </li>
                    </ul>
                    <div className="flex items-center gap-3 pt-2">
                      <Button asChild size="default" className="text-xs">
                        <Link to="/portal/merchants/apply">
                          Apply for merchant account
                          <ArrowRight className="ml-1.5 size-3.5" />
                        </Link>
                      </Button>
                      <Button asChild variant="outline" size="default" className="text-xs">
                        <Link to="/auth/login">Merchant Login</Link>
                      </Button>
                    </div>
                  </div>

                  <Card className="border border-border bg-surface p-5 rounded-xl space-y-3 font-mono text-xs">
                    <div className="flex items-center justify-between border-b border-border pb-2.5">
                      <span className="font-sans font-semibold text-text">Integration API</span>
                      <Badge variant="active">LIVE REST API</Badge>
                    </div>
                    <div className="bg-surface-subtle border border-border-subtle p-3 rounded-lg text-text-muted space-y-1">
                      <p className="text-text font-bold">POST /api/v1/merchants/me/orders</p>
                      <p className="text-xs">Authorization: Bearer sd_live_...</p>
                      <p className="text-xs text-status-success-ink font-sans">&bull; 201 Created — Tracking ID Issued</p>
                    </div>
                  </Card>
                </div>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </section>

      {/* 6. Island-Wide Coverage Hubs */}
      <section id="coverage" className="py-16 sm:py-24 border-b border-border bg-canvas">
        <div className="mx-auto max-w-6xl px-4 sm:px-8 space-y-10">
          <div className="text-center space-y-2 max-w-xl mx-auto">
            <Badge variant="success" className="text-xs">ACTIVE DISPATCH NETWORK</Badge>
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-text">
              Connecting Sri Lanka’s Commercial Corridors
            </h2>
            <p className="text-xs sm:text-sm text-text-muted">
              Dedicated transit hubs with synchronized morning and afternoon delivery runs.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <Card className="border border-border bg-surface p-5 rounded-xl space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-text">Western Hub</span>
                <span className="size-2 rounded-full bg-status-success-ink" />
              </div>
              <p className="text-xs text-text-muted">
                Colombo 01–15, Dehiwala, Mount Lavinia, Maharagama, Sri Jayawardenepura Kotte.
              </p>
              <span className="inline-block text-xs font-mono font-medium text-primary">Daily 3× Dispatch</span>
            </Card>

            <Card className="border border-border bg-surface p-5 rounded-xl space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-text">Central Hub</span>
                <span className="size-2 rounded-full bg-status-success-ink" />
              </div>
              <p className="text-xs text-text-muted">
                Kandy Urban, Peradeniya, Katugastota, Kundasale, Matale Central.
              </p>
              <span className="inline-block text-xs font-mono font-medium text-primary">Daily 2× Dispatch</span>
            </Card>

            <Card className="border border-border bg-surface p-5 rounded-xl space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-text">Southern Hub</span>
                <span className="size-2 rounded-full bg-status-success-ink" />
              </div>
              <p className="text-xs text-text-muted">
                Galle Fort, Karapitiya, Matara City, Weligama coastal belt.
              </p>
              <span className="inline-block text-xs font-mono font-medium text-primary">Daily 2× Dispatch</span>
            </Card>

            <Card className="border border-border bg-surface p-5 rounded-xl space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-text">Express Lines</span>
                <span className="size-2 rounded-full bg-status-success-ink" />
              </div>
              <p className="text-xs text-text-muted">
                Jaffna Peninsula, Anuradhapura, Kurunegala, Negombo Corridor.
              </p>
              <span className="inline-block text-xs font-mono font-medium text-primary">Next-Day Transit</span>
            </Card>
          </div>
        </div>
      </section>

      {/* 7. Conversion CTA Banner */}
      <section className="py-16 sm:py-20 bg-surface">
        <div className="mx-auto max-w-4xl px-4 sm:px-8 text-center space-y-6">
          <h2 className="text-2xl sm:text-4xl font-bold tracking-tight text-text">
            Start Shipping with SwiftDrop Today
          </h2>
          <p className="text-xs sm:text-sm text-text-muted max-w-xl mx-auto leading-relaxed">
            Create an account in less than two minutes. Book pickups, automate your shipping pipeline, and enjoy verified milestone tracking.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Button asChild size="lg" className="text-xs font-semibold">
              <Link to="/auth/register">
                Create Free Account
                <ArrowRight className="ml-1.5 size-4" />
              </Link>
            </Button>
            <Button asChild variant="outline" size="lg" className="text-xs font-semibold">
              <Link to="/auth/login">Sign In to Dashboard</Link>
            </Button>
          </div>
        </div>
      </section>

      {/* 8. Minimalist Global Footer */}
      <footer className="border-t border-border bg-canvas px-4 sm:px-8 py-10 text-xs text-text-muted">
        <div className="mx-auto max-w-6xl grid grid-cols-2 md:grid-cols-4 gap-8 pb-8">
          <div className="space-y-2.5">
            <div className="flex items-center gap-2 font-bold text-text">
              <Package className="size-4 text-primary" />
              SwiftDrop
            </div>
            <p className="text-xs leading-relaxed">
              Authoritative parcel delivery and courier logistics management system.
            </p>
          </div>
          <div className="space-y-2">
            <p className="font-semibold text-text">Services</p>
            <ul className="space-y-1.5">
              <li><Link to="/track" className="hover:text-text transition-colors">Public Tracking</Link></li>
              <li><Link to="/portal/book" className="hover:text-text transition-colors">Doorstep Booking</Link></li>
              <li><Link to="/portal/merchants/apply" className="hover:text-text transition-colors">Merchant Application</Link></li>
            </ul>
          </div>
          <div className="space-y-2">
            <p className="font-semibold text-text">Access Portals</p>
            <ul className="space-y-1.5">
              <li><Link to="/auth/login" className="hover:text-text transition-colors">Customer Login</Link></li>
              <li><Link to="/auth/login" className="hover:text-text transition-colors">Merchant Portal</Link></li>
              <li><Link to="/auth/login" className="hover:text-text transition-colors">Operations Console</Link></li>
            </ul>
          </div>
          <div className="space-y-2">
            <p className="font-semibold text-text">System Status</p>
            <div className="flex items-center gap-1.5 text-status-success-ink font-medium">
              <span className="size-2 rounded-full bg-status-success-ink" />
              All Systems Operational
            </div>
            <p className="text-xs text-text-muted">Core API &bull; Tracking SSE Active</p>
          </div>
        </div>

        <Separator />

        <div className="mx-auto max-w-6xl pt-6 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
          <span>&copy; 2026 SwiftDrop Logistics Inc. All rights reserved.</span>
          <span className="font-mono text-text-muted">Apple Cupertino Precision &bull; Sri Lanka</span>
        </div>
      </footer>
    </main>
  );
};
