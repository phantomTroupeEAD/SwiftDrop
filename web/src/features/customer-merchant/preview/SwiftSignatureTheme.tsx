import React, { useState } from 'react';
import {
  Package,
  ArrowRight,
  ArrowLeft,
  Check,
  ChevronRight,
  Clock3,
  MapPin,
  Search,
  Truck,
  AlertTriangle,
  Upload,
  Info,
  Moon,
  Sun,
  User,
  Phone,
  Scale,
  Maximize2,
} from 'lucide-react';
import './swift-signature.css';

type ScreenView = 'customer-booking' | 'merchant-orders';
type BookingStep = 1 | 2 | 3 | 4;

interface OrderItem {
  id: string;
  trackingNumber: string;
  recipientName: string;
  recipientCity: string;
  zone: string;
  status: 'IN_TRANSIT' | 'OUT_FOR_DELIVERY' | 'DELIVERED' | 'ATTEMPT_FAILED';
  weightKg: string;
  date: string;
}

const SAMPLE_ORDERS: OrderItem[] = [
  {
    id: 'ord-101',
    trackingNumber: 'SD-CMB-8921-LK',
    recipientName: 'Nimal Perera',
    recipientCity: 'Colombo 03',
    zone: 'WESTERN_COLOMBO_01',
    status: 'IN_TRANSIT',
    weightKg: '1.2 kg',
    date: '06 Nov 2026, 14:30',
  },
  {
    id: 'ord-102',
    trackingNumber: 'SD-KND-4309-LK',
    recipientName: 'Anura Wickramasinghe',
    recipientCity: 'Kandy',
    zone: 'CENTRAL_KANDY_01',
    status: 'OUT_FOR_DELIVERY',
    weightKg: '3.5 kg',
    date: '06 Nov 2026, 11:15',
  },
  {
    id: 'ord-103',
    trackingNumber: 'SD-GAL-7721-LK',
    recipientName: 'Dilani Samarasekera',
    recipientCity: 'Galle Fort',
    zone: 'SOUTHERN_GALLE_01',
    status: 'DELIVERED',
    weightKg: '0.8 kg',
    date: '05 Nov 2026, 16:45',
  },
  {
    id: 'ord-104',
    trackingNumber: 'SD-JAF-1184-LK',
    recipientName: 'Kavitha Rajan',
    recipientCity: 'Jaffna Town',
    zone: 'NORTHERN_JAFFNA_01',
    status: 'ATTEMPT_FAILED',
    weightKg: '2.1 kg',
    date: '06 Nov 2026, 09:20',
  },
  {
    id: 'ord-105',
    trackingNumber: 'SD-NEG-6632-LK',
    recipientName: 'Ruwan Fernando',
    recipientCity: 'Negombo',
    zone: 'WESTERN_GAMPAHA_02',
    status: 'IN_TRANSIT',
    weightKg: '4.0 kg',
    date: '06 Nov 2026, 15:10',
  },
];

export const SwiftSignatureTheme: React.FC = () => {
  const [night, setNight] = useState(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      return params.get('palette') === 'evening' || params.get('mode') === 'night';
    }
    return false;
  });

  const [activeScreen, setActiveScreen] = useState<ScreenView>(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      const screen = params.get('screen');
      if (screen === 'merchant' || screen === 'merchant-orders') return 'merchant-orders';

      if (window.location.hash === '#merchant') return 'merchant-orders';
    }
    return 'customer-booking';
  });

  // Customer Booking Wizard State
  const [bookingStep, setBookingStep] = useState<BookingStep>(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      const step = parseInt(params.get('step') || '1', 10);
      if (step >= 1 && step <= 4) return step as BookingStep;
    }
    return 1;
  });
  const [pickupCity, setPickupCity] = useState('Colombo 03');
  const [deliveryCity, setDeliveryCity] = useState('Kandy Central');
  const [senderName, setSenderName] = useState('Sajini');
  const [senderPhone, setSenderPhone] = useState('+94 77 123 4567');
  const [recipientName, setRecipientName] = useState('Kamal Gunaratne');
  const [recipientPhone, setRecipientPhone] = useState('+94 71 987 6543');
  const [packageWeight, setPackageWeight] = useState('2.50');
  const [packageDims, setPackageDims] = useState('25 x 20 x 15 cm');
  const [pickupZone, setPickupZone] = useState('WESTERN_COLOMBO_01');
  const [destZone, setDestZone] = useState('CENTRAL_KANDY_01');
  const [serviceLevel, setServiceLevel] = useState<'standard' | 'express'>('standard');
  const [isFragile, setIsFragile] = useState(false);

  // Tracking Widget State
  const [trackingNumberInput, setTrackingNumberInput] = useState('SD-DEMO-001');

  // Orders State
  const [merchantSearch, setMerchantSearch] = useState('');
  const [merchantStatusFilter, setMerchantStatusFilter] = useState<'ALL' | 'IN_TRANSIT' | 'OUT_FOR_DELIVERY' | 'DELIVERED' | 'ATTEMPT_FAILED'>('ALL');


  // User message banner state
  const [feedbackMessage, setFeedbackMessage] = useState('');


  const filteredOrders = SAMPLE_ORDERS.filter((ord) => {
    const matchesFilter = merchantStatusFilter === 'ALL' || ord.status === merchantStatusFilter;
    const matchesSearch =
      ord.trackingNumber.toLowerCase().includes(merchantSearch.toLowerCase()) ||
      ord.recipientName.toLowerCase().includes(merchantSearch.toLowerCase()) ||
      ord.recipientCity.toLowerCase().includes(merchantSearch.toLowerCase());
    return matchesFilter && matchesSearch;
  });

  const renderStatusBadge = (status: OrderItem['status']) => {
    switch (status) {
      case 'IN_TRANSIT':
        return (
          <span className="ss-badge ss-badge-teal">
            <span className="ss-badge-dot" />
            <span>In Transit</span>
          </span>
        );
      case 'OUT_FOR_DELIVERY':
        return (
          <span className="ss-badge ss-badge-teal">
            <span className="ss-badge-dot" />
            <span>Out for Delivery</span>
          </span>
        );
      case 'DELIVERED':
        return (
          <span className="ss-badge ss-badge-emerald">
            <span className="ss-badge-dot" />
            <span>Delivered</span>
          </span>
        );
      case 'ATTEMPT_FAILED':
        return (
          <span className="ss-badge ss-badge-amber">
            <span className="ss-badge-dot" />
            <span>Action Needed</span>
          </span>
        );
    }
  };

  return (
    <section
      className={`swift-signature ${night ? 'swift-signature--night' : ''}`}
      data-testid="signature-theme-root"
    >
      {/* 1. Header Bar with Theme Switcher and Night Mode */}


      {/* 2. Screen Switcher Navigation Bar */}


      {/* 3. Main App Container */}
      <div className="ss-app">
        {/* App Sidebar */}
        <aside className="ss-sidebar">
          <div className="ss-logo">
            <span className="ss-logo-icon">
              <Package size={20} />
            </span>
            <span>swift</span>
            <span className="ss-logo-drop">drop</span>
            <span className="ss-logo-dot" />
          </div>

          <div className="ss-workspace-label">YOUR WORKSPACE</div>

          <nav aria-label="Sidebar main navigation">
            <a
              href="#customer"
              onClick={(e) => {
                e.preventDefault();
                setActiveScreen('customer-booking');
                setFeedbackMessage('');
              }}
              className={`ss-sidebar-link ${activeScreen === 'customer-booking' ? 'active' : ''}`}
            >
              <Package size={18} />
              <span>Book a parcel</span>
            </a>

            <a
              href="#merchant"
              onClick={(e) => {
                e.preventDefault();
                setActiveScreen('merchant-orders');
                setFeedbackMessage('');
              }}
              className={`ss-sidebar-link ${activeScreen === 'merchant-orders' ? 'active' : ''}`}
            >
              <Truck size={18} />
              <span>Orders</span>
              <span className="ss-count">{SAMPLE_ORDERS.length}</span>
            </a>


          </nav>



          <div className="ss-account">
            <div className="ss-account-avatar">S</div>
            <div className="ss-account-info">
              <strong>Your account</strong>
              <small>Shipping account</small>
            </div>
          </div>
        </aside>

        {/* Main Content Area */}
        <div className="ss-main">
          {/* Top Bar */}
          <header className="ss-topbar">
            <div className="ss-topbar-breadcrumb">
              <span>Portals</span>
              <ChevronRight size={14} />
              <span>
                {activeScreen === 'customer-booking' ? 'Book a delivery' : 'Orders'}
              </span>
            </div>

            <div className="ss-topbar-actions"><span className="ss-sample-label">Sample data</span><button className="ss-appearance-toggle" data-testid="signature-night-toggle" aria-label={night ? 'Switch to light mode' : 'Switch to dark mode'} aria-pressed={night} onClick={() => setNight(!night)}>{night ? <Sun size={18}/> : <Moon size={18}/>}</button></div>
          </header>

          {/* Screen Content Panels */}
          <div className="ss-content">
            {/* SCREEN A: CUSTOMER BOOKING WIZARD */}
            {activeScreen === 'customer-booking' && (
              <div className="space-y-6">
                {/* 4-Step Stepper Navigation */}
                <ol className="ss-stepper" aria-label="Customer parcel booking steps">
                  <li
                    className={`ss-step-item ${bookingStep === 1 ? 'active' : ''} ${bookingStep > 1 ? 'completed' : ''}`}
                    onClick={() => setBookingStep(1)}
                    data-testid="signature-booking-step-1"
                  >
                    <div className="ss-step-num">{bookingStep > 1 ? <Check size={16} strokeWidth={2.5} /> : '1'}</div>
                    <div className="ss-step-label">Addresses</div>
                  </li>

                  <li
                    className={`ss-step-item ${bookingStep === 2 ? 'active' : ''} ${bookingStep > 2 ? 'completed' : ''}`}
                    onClick={() => setBookingStep(2)}
                    data-testid="signature-booking-step-2"
                  >
                    <div className="ss-step-num">{bookingStep > 2 ? <Check size={16} strokeWidth={2.5} /> : '2'}</div>
                    <div className="ss-step-label">Parcel details</div>
                  </li>

                  <li
                    className={`ss-step-item ${bookingStep === 3 ? 'active' : ''} ${bookingStep > 3 ? 'completed' : ''}`}
                    onClick={() => setBookingStep(3)}
                    data-testid="signature-booking-step-3"
                  >
                    <div className="ss-step-num">{bookingStep > 3 ? <Check size={16} strokeWidth={2.5} /> : '3'}</div>
                    <div className="ss-step-label">Quote</div>
                  </li>

                  <li
                    className={`ss-step-item ${bookingStep === 4 ? 'active' : ''}`}
                    onClick={() => setBookingStep(4)}
                    data-testid="signature-booking-step-4"
                  >
                    <div className="ss-step-num">4</div>
                    <div className="ss-step-label">Review</div>
                  </li>
                </ol>

                {/* STEP 1: ADDRESSES */}
                {bookingStep === 1 && (
                  <section className="ss-card" aria-labelledby="step-1-title">
                    <div className="ss-card-header">
                      <div>
                        <h1 id="step-1-title">Pickup &amp; delivery</h1>
                        <p>Add the sender and recipient details.</p>
                      </div>
                    </div>

                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        setBookingStep(2);
                        setFeedbackMessage('');
                      }}
                    >
                      <div className="ss-form-grid">
                        {/* Sender Information */}
                        <div className="ss-field-group">
                          <label htmlFor="ss-sender-name">
                            Sender Full Name
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <User size={16} />
                            <input
                              id="ss-sender-name"
                              data-testid="signature-booking-sender-name"
                              value={senderName}
                              onChange={(e) => setSenderName(e.target.value)}
                              placeholder="e.g. Your account"
                              required
                            />
                          </div>
                        </div>

                        <div className="ss-field-group">
                          <label htmlFor="ss-sender-phone">
                            Sender Phone (Mobile)
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <Phone size={16} />
                            <input
                              id="ss-sender-phone"
                              data-testid="signature-booking-sender-phone"
                              value={senderPhone}
                              onChange={(e) => setSenderPhone(e.target.value)}
                              placeholder="+94 77 123 4567"
                              required
                            />
                          </div>
                        </div>

                        <div className="ss-field-group full-width">
                          <label htmlFor="ss-pickup">
                            Pickup Address &amp; City
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <MapPin size={16} />
                            <input
                              id="ss-pickup"
                              data-testid="signature-pickup-input"
                              value={pickupCity}
                              onChange={(e) => setPickupCity(e.target.value)}
                              placeholder="Where are we collecting the parcel from?"
                              required
                            />
                          </div>
                          <span className="ss-field-helper">Include building number and street name.</span>
                        </div>

                        {/* Recipient Information */}
                        <div className="ss-field-group">
                          <label htmlFor="ss-recipient-name">
                            Recipient Full Name
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <User size={16} />
                            <input
                              id="ss-recipient-name"
                              data-testid="signature-booking-recipient-name"
                              value={recipientName}
                              onChange={(e) => setRecipientName(e.target.value)}
                              placeholder="e.g. Kamal Gunaratne"
                              required
                            />
                          </div>
                        </div>

                        <div className="ss-field-group">
                          <label htmlFor="ss-recipient-phone">
                            Recipient Phone (Mobile)
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <Phone size={16} />
                            <input
                              id="ss-recipient-phone"
                              data-testid="signature-booking-recipient-phone"
                              value={recipientPhone}
                              onChange={(e) => setRecipientPhone(e.target.value)}
                              placeholder="+94 71 987 6543"
                              required
                            />
                          </div>
                        </div>

                        <div className="ss-field-group full-width">
                          <label htmlFor="ss-destination">
                            Delivery Destination &amp; City
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <MapPin size={16} />
                            <input
                              id="ss-destination"
                              data-testid="signature-delivery-input"
                              value={deliveryCity}
                              onChange={(e) => setDeliveryCity(e.target.value)}
                              placeholder="Where is your parcel going?"
                              required
                            />
                          </div>
                        </div>
                      </div>

                      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mt-6 pt-4 border-t border-[var(--ss-line)]">
                        <span className="text-xs text-[var(--ss-muted)]">Your details stay here as you continue.</span>
                        <button
                          type="submit"
                          className="ss-primary"
                          data-testid="signature-booking-next-btn"
                        >
                          <span>Continue to Parcel Details</span>
                          <ArrowRight size={16} />
                        </button>
                      </div>
                    </form>
                  </section>
                )}

                {/* STEP 2: PARCEL & ZONES */}
                {bookingStep === 2 && (
                  <section className="ss-card" aria-labelledby="step-2-title">
                    <div className="ss-card-header">
                      <div>
                        <h1 id="step-2-title">Parcel details</h1>
                        <p>Choose the parcel size, weight and delivery service.</p>
                      </div>
                    </div>

                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        setBookingStep(3);
                        setFeedbackMessage('');
                      }}
                    >
                      <div className="ss-form-grid">
                        <div className="ss-field-group">
                          <label htmlFor="ss-weight">
                            Package Weight (kg)
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <Scale size={16} />
                            <input
                              id="ss-weight"
                              type="number"
                              step="0.01"
                              min="0.1"
                              max="30"
                              data-testid="signature-booking-weight"
                              value={packageWeight}
                              onChange={(e) => setPackageWeight(e.target.value)}
                              required
                            />
                          </div>
                          <span className="ss-field-helper">Standard courier limit: 30.0 kg</span>
                        </div>

                        <div className="ss-field-group">
                          <label htmlFor="ss-dims">
                            Dimensions (L x W x H cm)
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <Maximize2 size={16} />
                            <input
                              id="ss-dims"
                              data-testid="signature-booking-dims"
                              value={packageDims}
                              onChange={(e) => setPackageDims(e.target.value)}
                              placeholder="25 x 20 x 15 cm"
                              required
                            />
                          </div>
                          <span className="ss-field-helper">Used for volumetric weight check</span>
                        </div>

                        <div className="ss-field-group">
                          <label htmlFor="ss-pickup-zone">
                            Pickup Zone
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <select
                              id="ss-pickup-zone"
                              data-testid="signature-booking-zone-select"
                              value={pickupZone}
                              onChange={(e) => setPickupZone(e.target.value)}
                            >
                              <option value="WESTERN_COLOMBO_01">Western - Colombo Central (01)</option>
                              <option value="WESTERN_GAMPAHA_02">Western - Gampaha &amp; Negombo (02)</option>
                              <option value="CENTRAL_KANDY_01">Central - Kandy Metro (01)</option>
                            </select>
                          </div>
                        </div>

                        <div className="ss-field-group">
                          <label htmlFor="ss-dest-zone">
                            Destination Delivery Zone
                            <span className="ss-required">*</span>
                          </label>
                          <div className="ss-input-wrap">
                            <select
                              id="ss-dest-zone"
                              value={destZone}
                              onChange={(e) => setDestZone(e.target.value)}
                            >
                              <option value="CENTRAL_KANDY_01">Central - Kandy Metro (01)</option>
                              <option value="WESTERN_COLOMBO_01">Western - Colombo Central (01)</option>
                              <option value="SOUTHERN_GALLE_01">Southern - Galle Metro (01)</option>
                              <option value="NORTHERN_JAFFNA_01">Northern - Jaffna Town (01)</option>
                            </select>
                          </div>
                        </div>

                        <div className="ss-field-group full-width">
                          <label>Delivery Service Priority</label>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-1">
                            <div
                              onClick={() => setServiceLevel('standard')}
                              className={`p-3 rounded-lg border cursor-pointer transition flex items-center justify-between ${
                                serviceLevel === 'standard'
                                  ? 'border-[var(--ss-primary)] bg-[var(--ss-mint)] font-semibold'
                                  : 'border-[var(--ss-line)] bg-[var(--ss-surface)]'
                              }`}
                            >
                              <div>
                                <div className="text-sm">Standard Delivery</div>
                                <div className="text-xs text-[var(--ss-muted)]">2–3 Business Days</div>
                              </div>
                              {serviceLevel === 'standard' && <Check size={18} className="text-[var(--ss-primary)]" />}
                            </div>

                            <div
                              onClick={() => setServiceLevel('express')}
                              className={`p-3 rounded-lg border cursor-pointer transition flex items-center justify-between ${
                                serviceLevel === 'express'
                                  ? 'border-[var(--ss-primary)] bg-[var(--ss-mint)] font-semibold'
                                  : 'border-[var(--ss-line)] bg-[var(--ss-surface)]'
                              }`}
                            >
                              <div>
                                <div className="text-sm">Express Priority</div>
                                <div className="text-xs text-[var(--ss-muted)]">Priority delivery</div>
                              </div>
                              {serviceLevel === 'express' && <Check size={18} className="text-[var(--ss-primary)]" />}
                            </div>
                          </div>
                        </div>

                        <div className="ss-field-group full-width">
                          <label className="ss-checkbox-label">
                            <input
                              type="checkbox"
                              checked={isFragile}
                              onChange={(e) => setIsFragile(e.target.checked)}
                              data-testid="signature-booking-fragile-check"
                            />
                            <span>Fragile item — requires gentle courier handling and shock protection</span>
                          </label>
                        </div>
                      </div>

                      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mt-6 pt-4 border-t border-[var(--ss-line)]">
                        <button
                          type="button"
                          className="ss-secondary"
                          onClick={() => setBookingStep(1)}
                          data-testid="signature-booking-back-btn"
                        >
                          <ArrowLeft size={16} />
                          <span>Back to Addresses</span>
                        </button>

                        <button
                          type="submit"
                          className="ss-primary"
                          data-testid="signature-booking-preview-btn"
                        >
                          <span>View quote</span>
                          <ArrowRight size={16} />
                        </button>
                      </div>
                    </form>
                  </section>
                )}

                {/* STEP 3: SERVER QUOTE */}
                {bookingStep === 3 && (
                  <section className="ss-card" aria-labelledby="step-3-title">
                    <div className="ss-card-header">
                      <div>
                        <h1 id="step-3-title">Delivery estimate</h1>
                        <p>Check the delivery cost before continuing.</p>
                      </div>

                      <span className="ss-badge ss-badge-teal">
                        <Clock3 size={13} />
                        <span>Sample quote</span>
                      </span>
                    </div>

                    <div className="p-4 rounded-xl bg-[var(--ss-canvas)] border border-[var(--ss-line)] space-y-4">
                      <div className="flex items-center justify-between pb-3 border-b border-[var(--ss-line)]">
                        <span className="text-sm font-semibold">Your delivery</span>
                        <div className="text-right">
                          <span className="text-xs text-[var(--ss-muted)] block">Delivery service</span>
                          <span className="text-sm font-semibold capitalize">{serviceLevel} Service</span>
                        </div>
                      </div>

                      <div className="space-y-2 text-sm">
                        <div className="flex justify-between py-1">
                          <span className="text-[var(--ss-muted)]">Delivery charge</span>
                          <span className="tabular-num font-medium">LKR 450.00</span>
                        </div>
                        <div className="flex justify-between py-1">
                          <span className="text-[var(--ss-muted)]">Weight Charge ({packageWeight} kg)</span>
                          <span className="tabular-num font-medium">LKR 150.00</span>
                        </div>
                      </div>

                      <div className="flex items-center justify-between pt-3 border-t border-[var(--ss-line)]">
                        <div>
                          <strong className="text-lg">Total Estimated Fee</strong>
                          <small className="block text-xs text-[var(--ss-muted)]">Sample price for this layout</small>
                        </div>
                        <div
                          className="text-2xl font-bold text-[var(--ss-primary)] tabular-num"
                          data-testid="signature-booking-quote-display"
                        >
                          LKR 600.00
                        </div>
                      </div>
                    </div>

                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mt-6 pt-4 border-t border-[var(--ss-line)]">
                      <button
                        type="button"
                        className="ss-secondary"
                        onClick={() => setBookingStep(2)}
                        data-testid="signature-booking-back-btn"
                      >
                        <ArrowLeft size={16} />
                        <span>Back to Parcel Specs</span>
                      </button>

                      <button
                        type="button"
                        className="ss-primary"
                        onClick={() => setBookingStep(4)}
                      >
                        <span>Review &amp; Confirm Booking</span>
                        <ArrowRight size={16} />
                      </button>
                    </div>
                  </section>
                )}

                {/* STEP 4: REVIEW & CONFIRM */}
                {bookingStep === 4 && (
                  <section className="ss-card" aria-labelledby="step-4-title">
                    <div className="ss-card-header">
                      <div>
                        <h1 id="step-4-title">Review your delivery</h1>
                        <p>Check the details before confirming.</p>
                      </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 p-4 rounded-xl bg-[var(--ss-canvas)] border border-[var(--ss-line)] text-sm">
                      <div className="space-y-1">
                        <span className="text-xs text-[var(--ss-muted)] uppercase font-semibold">Origin &amp; Sender</span>
                        <div><strong>{senderName}</strong> ({senderPhone})</div>
                        <div className="text-xs text-[var(--ss-muted)]">{pickupCity}</div>
                      </div>

                      <div className="space-y-1">
                        <span className="text-xs text-[var(--ss-muted)] uppercase font-semibold">Destination &amp; Recipient</span>
                        <div><strong>{recipientName}</strong> ({recipientPhone})</div>
                        <div className="text-xs text-[var(--ss-muted)]">{deliveryCity}</div>
                      </div>

                      <div className="space-y-1 pt-3 border-t border-[var(--ss-line)] md:border-t-0">
                        <span className="text-xs text-[var(--ss-muted)] uppercase font-semibold">Package Specs</span>
                        <div>Weight: <strong>{packageWeight} kg</strong></div>
                        <div>Dimensions: <strong>{packageDims}</strong></div>
                        <div>Service: <strong className="capitalize">{serviceLevel}</strong></div>
                      </div>
                    </div>

                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mt-6 pt-4 border-t border-[var(--ss-line)]">
                      <button
                        type="button"
                        className="ss-secondary"
                        onClick={() => setBookingStep(3)}
                        data-testid="signature-booking-back-btn"
                      >
                        <ArrowLeft size={16} />
                        <span>Back to Quote</span>
                      </button>

                      <button
                        type="button"
                        className="ss-primary"
                        data-testid="signature-booking-confirm-btn"
                        onClick={() => {
                          setFeedbackMessage('Booking is not connected yet. No parcel has been created.');
                        }}
                      >
                        <Check size={16} />
                        <span>Confirm booking</span>
                      </button>
                    </div>
                  </section>
                )}

                {/* Public Tracking Preview Card */}
                <section className="ss-card">
                  <div className="ss-card-header">
                    <div>
                      <span className="ss-eyebrow">TRACK A PARCEL</span>
                      <h3>Where’s your parcel?</h3>
                      <p>Follow your delivery with a tracking number.</p>
                    </div>
                    <Truck size={24} className="text-[var(--ss-primary)]" />
                  </div>

                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      setFeedbackMessage('Tracking search is not connected yet. The timeline below is sample data.');
                    }}
                    className="flex flex-col sm:flex-row gap-2"
                  >
                    <label htmlFor="ss-number" className="sr-only">Tracking number</label>
                    <div className="ss-input-wrap flex-1">
                      <Search size={16} />
                      <input
                        id="ss-number"
                        data-testid="signature-tracking-input"
                        placeholder="Enter tracking number (e.g. SD-DEMO-001)"
                        value={trackingNumberInput}
                        onChange={(e) => setTrackingNumberInput(e.target.value)}
                      />
                    </div>
                    <button
                      type="submit"
                      className="ss-primary"
                      data-testid="signature-tracking-search-btn"
                    >
                      <Search size={16} />
                      <span>Track Shipment</span>
                    </button>
                  </form>

                  <div className="mt-4 p-4 rounded-xl bg-[var(--ss-mint)] border border-[var(--ss-mint-dark)] space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-sm font-bold">SD-DEMO-001</span>
                      <span className="ss-badge ss-badge-teal">In Transit</span>
                    </div>

                    <div className="text-sm">
                      Destination: <strong>Kandy</strong>
                    </div>

                    <div className="space-y-2 pt-2 border-t border-[var(--ss-mint-dark)] text-xs">
                      <div className="flex items-center gap-2">
                        <Check size={14} className="text-[var(--ss-primary)]" />
                        <span><strong>09:00 AM</strong> — Booking confirmed in Colombo Central Hub</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Check size={14} className="text-[var(--ss-primary)]" />
                        <span><strong>11:45 AM</strong> — Collected by courier route #4</span>
                      </div>
                      <div className="flex items-center gap-2 text-[var(--ss-primary)] font-semibold">
                        <Truck size={14} />
                        <span><strong>02:30 PM</strong> — In transit towards Kandy Sorting Center</span>
                      </div>
                    </div>
                  </div>
                </section>
              </div>
            )}

            {/* SCREEN B: MERCHANT ORDERS & OPERATIONS */}
            {activeScreen === 'merchant-orders' && (
              <div className="space-y-6">
                {/* 1. Summary KPI Bar */}
                <div className="ss-stat-grid">
                  <div className="ss-stat-card">
                    <div className="ss-stat-icon">
                      <Package size={22} />
                    </div>
                    <div className="ss-stat-info">
                      <small>Total orders</small>
                      <strong data-testid="signature-merchant-stat-total">{SAMPLE_ORDERS.length}</strong>
                    </div>
                  </div>

                  <div className="ss-stat-card">
                    <div className="ss-stat-icon">
                      <Truck size={22} />
                    </div>
                    <div className="ss-stat-info">
                      <small>In transit</small>
                      <strong data-testid="signature-merchant-stat-transit">{SAMPLE_ORDERS.filter(order => order.status === 'IN_TRANSIT').length}</strong>
                    </div>
                  </div>

                  <div className="ss-stat-card">
                    <div className="ss-stat-icon emerald">
                      <Check size={22} />
                    </div>
                    <div className="ss-stat-info">
                      <small>Delivered</small>
                      <strong data-testid="signature-merchant-stat-delivered">{SAMPLE_ORDERS.filter(order => order.status === 'DELIVERED').length}</strong>
                    </div>
                  </div>

                  <div className="ss-stat-card">
                    <div className="ss-stat-icon amber">
                      <AlertTriangle size={22} />
                    </div>
                    <div className="ss-stat-info">
                      <small>Needs attention</small>
                      <strong data-testid="signature-merchant-stat-warning">{SAMPLE_ORDERS.filter(order => order.status === 'ATTEMPT_FAILED').length}</strong>
                    </div>
                  </div>
                </div>

                {/* 2. Controls & Filter Bar */}
                <section className="ss-card">
                  <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                    {/* Search & Status Filters */}
                    <div className="flex flex-wrap items-center gap-2 flex-1">
                      <div className="ss-input-wrap w-full sm:w-64">
                        <Search size={16} />
                        <input
                          aria-label="Search orders"
                          placeholder="Search orders or city..."
                          value={merchantSearch}
                          onChange={(e) => setMerchantSearch(e.target.value)}
                          data-testid="signature-merchant-search-input"
                        />
                      </div>

                      <div className="flex flex-wrap items-center gap-1">
                        <button
                          className={`px-3 py-2 text-xs font-semibold rounded-lg border min-h-[44px] transition ${
                            merchantStatusFilter === 'ALL'
                              ? 'bg-[var(--ss-primary)] text-white border-[var(--ss-primary)]'
                              : 'bg-[var(--ss-surface)] text-[var(--ss-ink)] border-[var(--ss-line)]'
                          }`}
                          onClick={() => setMerchantStatusFilter('ALL')}
                        >
                          All ({SAMPLE_ORDERS.length})
                        </button>

                        <button
                          className={`px-3 py-2 text-xs font-semibold rounded-lg border min-h-[44px] transition ${
                            merchantStatusFilter === 'IN_TRANSIT'
                              ? 'bg-[var(--ss-primary)] text-white border-[var(--ss-primary)]'
                              : 'bg-[var(--ss-surface)] text-[var(--ss-ink)] border-[var(--ss-line)]'
                          }`}
                          onClick={() => setMerchantStatusFilter('IN_TRANSIT')}
                        >
                          In Transit
                        </button>

                        <button
                          className={`px-3 py-2 text-xs font-semibold rounded-lg border min-h-[44px] transition ${
                            merchantStatusFilter === 'DELIVERED'
                              ? 'bg-[var(--ss-primary)] text-white border-[var(--ss-primary)]'
                              : 'bg-[var(--ss-surface)] text-[var(--ss-ink)] border-[var(--ss-line)]'
                          }`}
                          onClick={() => setMerchantStatusFilter('DELIVERED')}
                        >
                          Delivered
                        </button>

                        <button
                          className={`px-3 py-2 text-xs font-semibold rounded-lg border min-h-[44px] transition ${
                            merchantStatusFilter === 'ATTEMPT_FAILED'
                              ? 'bg-[var(--ss-amber)] text-white border-[var(--ss-amber)]'
                              : 'bg-[var(--ss-surface)] text-[var(--ss-ink)] border-[var(--ss-line)]'
                          }`}
                          onClick={() => setMerchantStatusFilter('ATTEMPT_FAILED')}
                        >
                          Action Needed
                        </button>
                      </div>
                    </div>

                    {/* Batch Actions & State Selector Tool */}
                    <div className="flex flex-wrap items-center gap-2">
                      <button
                        type="button"
                        className="ss-secondary"
                        data-testid="signature-merchant-csv-btn"
                        onClick={() => setFeedbackMessage('Order import is not connected yet. No file has been uploaded.')}
                      >
                        <Upload size={16} />
                        <span>Import orders</span>
                      </button>

                      {/* State Inspector for UI reviewers */}

                    </div>
                  </div>

                  {/* 3. Orders Table Content Depending on State */}
                  <div className="mt-4">
                    <div className="ss-table-container">
                      <table className="ss-table" data-testid="signature-merchant-orders-table">
                        <thead>
                          <tr>
                            <th>Tracking Number</th>
                            <th>Recipient &amp; Destination</th>
                            <th className="text-right">Weight</th>
                            <th>Status</th>
                            <th>Created</th>
                            <th className="text-right">Action</th>
                          </tr>
                        </thead>
                        <tbody>
                          {filteredOrders.length > 0 ? (
                            filteredOrders.map((ord) => (
                              <tr key={ord.id}>
                                <td>
                                  <strong className="font-mono text-sm tabular-num text-[var(--ss-ink)]">{ord.trackingNumber}</strong>
                                </td>
                                <td>
                                  <div className="font-medium text-[var(--ss-ink)]">{ord.recipientName}</div>
                                  <small className="text-[var(--ss-muted)]">{ord.recipientCity}</small>
                                </td>
                                <td className="text-right">
                                  <span className="tabular-num font-medium text-[var(--ss-ink)]">{ord.weightKg.replace(' kg', '')}</span>{' '}
                                  <span className="text-[var(--ss-muted)] text-xs">kg</span>
                                </td>
                                <td>{renderStatusBadge(ord.status)}</td>
                                <td className="text-[var(--ss-muted)] tabular-num text-xs whitespace-nowrap">{ord.date}</td>
                                <td className="text-right">
                                  <button
                                    className="ss-secondary ss-btn-sm"
                                    onClick={() => setFeedbackMessage(`Selected parcel ${ord.trackingNumber} for review.`)}
                                  >
                                    Details
                                  </button>
                                </td>
                              </tr>
                            ))
                          ) : (
                            <tr>
                              <td colSpan={6} className="text-center py-10 text-[var(--ss-muted)]">
                                <div className="flex flex-col items-center justify-center gap-2">
                                  <Package size={28} className="text-[var(--ss-muted)] opacity-50 mb-1" />
                                  <p className="font-medium text-sm text-[var(--ss-ink)]">No orders match your search.</p>
                                  <p className="text-xs text-[var(--ss-muted)]">Try adjusting your keywords or clearing the status filter.</p>
                                  <button
                                    className="ss-secondary ss-btn-sm mt-2"
                                    onClick={() => {
                                      setMerchantSearch('');
                                      setMerchantStatusFilter('ALL');
                                    }}
                                  >
                                    Reset filters
                                  </button>
                                </div>
                              </td>
                            </tr>
                          )}
                        </tbody>
                      </table>
                    </div>
                  </div>

                  {/* Connection Health & Live SSE Indicator */}

                </section>
              </div>
            )}

            {/* SCREEN C: PALETTE SPECIFICATIONS & M7 COORDINATION */}


            {/* Bottom Feedback / Invariant Banner */}
            {feedbackMessage && <div
              className="ss-feedback info"
              role="status"
              data-testid="signature-demo-feedback"
            >
              <Info size={16} className="shrink-0 text-[var(--ss-primary)]" />
              <span>
                {feedbackMessage}
              </span>
            </div>}
          </div>
        </div>
      </div>
    </section>
  );
};
