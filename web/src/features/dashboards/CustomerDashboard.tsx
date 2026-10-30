import React, { useState } from 'react';
import { DashboardLayout } from './DashboardLayout';
import {
  Truck,
  CheckCircle2,
  Clock,
  MapPin,
  ArrowRight,
  Calculator,
  Download,
} from 'lucide-react';

export const CustomerDashboard: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'TRACK' | 'BOOK' | 'HISTORY'>('TRACK');
  const [weightKg, setWeightKg] = useState('2.5');
  const [serviceLevel, setServiceLevel] = useState<'STANDARD' | 'EXPRESS' | 'SAME_DAY'>('EXPRESS');
  const [quoteFee, setQuoteFee] = useState('LKR 650.00');

  const calculateQuote = () => {
    const weight = parseFloat(weightKg) || 1.0;
    const base = serviceLevel === 'SAME_DAY' ? 500 : serviceLevel === 'EXPRESS' ? 350 : 250;
    const perKg = 120;
    const total = base + weight * perKg;
    setQuoteFee(`LKR ${total.toFixed(2)}`);
  };

  return (
    <DashboardLayout
      activeRole="CUSTOMER"
      title="Customer Self-Service Parcel Portal"
      subtitle="Book doorstep parcel pickup, calculate delivery tariffs, track live shipment milestones, and download receipts."
    >
      {/* 4 Stat Overview Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Active Shipments</div>
            <div className="text-2xl font-bold text-blue-400 mt-1">1 En Route</div>
            <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
              <Clock className="w-3 h-3 text-amber-400" /> Out for delivery today
            </div>
          </div>
          <div className="p-3 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400">
            <Truck className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Delivered Parcels</div>
            <div className="text-2xl font-bold text-emerald-400 mt-1">14 Total</div>
            <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
              <CheckCircle2 className="w-3 h-3 text-emerald-400" /> 100% successful
            </div>
          </div>
          <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <CheckCircle2 className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Total Spent (LKR)</div>
            <div className="text-2xl font-bold text-white mt-1">LKR 9,450</div>
            <div className="text-[11px] text-slate-400 mt-1">All orders</div>
          </div>
          <div className="p-3 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400">
            <Calculator className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Saved Addresses</div>
            <div className="text-2xl font-bold text-white mt-1">3 Locations</div>
            <div className="text-[11px] text-slate-400 mt-1">Home, Office, Parents</div>
          </div>
          <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400">
            <MapPin className="w-5 h-5" />
          </div>
        </div>
      </div>

      {/* Main Tab Controls */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-5">
        <div className="flex border-b border-slate-800 pb-3 gap-2">
          <button
            onClick={() => setActiveTab('TRACK')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer ${
              activeTab === 'TRACK'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Live Shipment Tracker
          </button>
          <button
            onClick={() => setActiveTab('BOOK')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer ${
              activeTab === 'BOOK'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Book New Parcel Pickup
          </button>
          <button
            onClick={() => setActiveTab('HISTORY')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer ${
              activeTab === 'HISTORY'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Recent Shipments & Receipts
          </button>
        </div>

        {/* Tab 1: Live Tracker */}
        {activeTab === 'TRACK' && (
          <div className="space-y-4">
            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                <div>
                  <span className="text-[10px] text-slate-500 uppercase font-bold tracking-wider">
                    Tracking Number
                  </span>
                  <div className="text-lg font-mono font-bold text-blue-400">
                    SD-20261030-99801
                  </div>
                </div>
                <span className="px-2.5 py-1 rounded-full text-xs font-bold bg-blue-500/10 text-blue-400 border border-blue-500/30 w-fit">
                  OUT FOR DELIVERY
                </span>
              </div>

              {/* Progress Milestones Bar */}
              <div className="pt-2 pb-1">
                <div className="grid grid-cols-5 gap-1 text-center text-[10px] font-semibold text-slate-400">
                  <div className="text-emerald-400">1. Booked</div>
                  <div className="text-emerald-400">2. Picked Up</div>
                  <div className="text-emerald-400">3. In Transit</div>
                  <div className="text-blue-400 font-bold">4. Out for Delivery</div>
                  <div className="text-slate-600">5. Delivered</div>
                </div>
                <div className="w-full bg-slate-800 h-2 rounded-full mt-2 overflow-hidden flex">
                  <div className="bg-emerald-500 h-full w-1/5" />
                  <div className="bg-emerald-500 h-full w-1/5" />
                  <div className="bg-emerald-500 h-full w-1/5" />
                  <div className="bg-blue-500 h-full w-1/5 animate-pulse" />
                  <div className="bg-slate-800 h-full w-1/5" />
                </div>
              </div>

              <div className="text-xs text-slate-400 pt-1 flex flex-col sm:flex-row justify-between gap-1 border-t border-slate-900">
                <span>Courier Assigned: <strong className="text-slate-200">Kamal Perera (Bike)</strong></span>
                <span>Estimated Arrival: <strong className="text-emerald-400">Today, ~2:45 PM</strong></span>
              </div>
            </div>
          </div>
        )}

        {/* Tab 2: Book Parcel Wizard */}
        {activeTab === 'BOOK' && (
          <div className="space-y-4 text-xs">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="font-medium text-slate-300">Pickup Address</label>
                <input
                  type="text"
                  defaultValue="24 Peradeniya Road, Kandy"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-white text-xs"
                />
              </div>
              <div className="space-y-2">
                <label className="font-medium text-slate-300">Destination Address</label>
                <input
                  type="text"
                  defaultValue="42 Galle Road, Colombo 04"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-white text-xs"
                />
              </div>
              <div className="space-y-2">
                <label className="font-medium text-slate-300">Package Weight (kg)</label>
                <input
                  type="number"
                  value={weightKg}
                  onChange={(e) => setWeightKg(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-white text-xs"
                />
              </div>
              <div className="space-y-2">
                <label className="font-medium text-slate-300">Service Level</label>
                <select
                  value={serviceLevel}
                  onChange={(e) => setServiceLevel(e.target.value as any)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-white text-xs"
                >
                  <option value="STANDARD">Standard Delivery (2-3 Days)</option>
                  <option value="EXPRESS">Express Priority (Next Day)</option>
                  <option value="SAME_DAY">Same-Day Ultra Rush</option>
                </select>
              </div>
            </div>

            {/* Authoritative Pricing Quote Box */}
            <div className="p-4 rounded-xl bg-slate-950 border border-blue-500/30 flex items-center justify-between">
              <div>
                <span className="text-[10px] text-slate-400 uppercase font-semibold">
                  Estimated Delivery Fee
                </span>
                <div className="text-xl font-bold text-emerald-400 mt-0.5">{quoteFee}</div>
                <div className="text-[10px] text-slate-500">Includes zone base fee & weight surcharge</div>
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={calculateQuote}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg font-semibold text-xs border border-slate-700 cursor-pointer"
                >
                  Recalculate
                </button>
                <button
                  type="button"
                  onClick={() => alert("Booking submitted! Tracking number generated.")}
                  className="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-semibold text-xs flex items-center gap-1.5 shadow-sm cursor-pointer"
                >
                  Book Pickup
                  <ArrowRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Tab 3: History & Receipts */}
        {activeTab === 'HISTORY' && (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
                <tr>
                  <th className="py-2.5 px-3">Tracking #</th>
                  <th className="py-2.5 px-3">Delivered To</th>
                  <th className="py-2.5 px-3">Date</th>
                  <th className="py-2.5 px-3">Fee (LKR)</th>
                  <th className="py-2.5 px-3 text-right">Receipt</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {[
                  { id: 'SD-20261025-44211', recipient: 'Nimali Silva (Colombo)', date: '25 Oct 2026', fee: 'LKR 450.00' },
                  { id: 'SD-20261020-11204', recipient: 'Ruwan Gamage (Galle)', date: '20 Oct 2026', fee: 'LKR 820.00' },
                  { id: 'SD-20261012-99812', recipient: 'Saman Kumara (Kandy)', date: '12 Oct 2026', fee: 'LKR 650.00' },
                ].map((row) => (
                  <tr key={row.id} className="hover:bg-slate-800/30">
                    <td className="py-2.5 px-3 font-mono font-bold text-blue-400">{row.id}</td>
                    <td className="py-2.5 px-3 text-slate-200">{row.recipient}</td>
                    <td className="py-2.5 px-3 text-slate-400">{row.date}</td>
                    <td className="py-2.5 px-3 font-semibold text-emerald-400">{row.fee}</td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => alert(`Downloading receipt for ${row.id}...`)}
                        className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-slate-800 transition cursor-pointer"
                        title="Download Receipt PDF"
                      >
                        <Download className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </DashboardLayout>
  );
};
