import React, { useState } from 'react';
import { DashboardLayout } from './DashboardLayout';
import {
  Store,
  Package,
  Key,
  Webhook,
  UploadCloud,
  CheckCircle2,
  Copy,
  Clock,
  TrendingUp,
  ShieldCheck,
} from 'lucide-react';

export const MerchantDashboard: React.FC = () => {
  const [apiKeyCopied, setApiKeyCopied] = useState(false);
  const [activeTab, setActiveTab] = useState<'BATCH' | 'API' | 'WEBHOOKS'>('BATCH');

  const copyKey = () => {
    navigator.clipboard.writeText('sd_live_a1b2c3d4_9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c');
    setApiKeyCopied(true);
    setTimeout(() => setApiKeyCopied(false), 3000);
  };

  return (
    <DashboardLayout
      activeRole="MERCHANT"
      title="B2B Merchant Logistics & Shipping Console"
      subtitle="Bulk order ingestion, API integration keys, webhook event configuration, and shipping analytics."
    >
      {/* Multi-Tenant Scoping Banner */}
      <div className="p-4 rounded-xl bg-slate-900 border border-amber-500/30 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <Store className="w-5 h-5" />
          </div>
          <div>
            <div className="font-bold text-white text-sm">Colombo Tech Stores (Merchant Account)</div>
            <div className="text-slate-400">
              Tenant ID:{' '}
              <span className="font-mono text-amber-400 font-semibold">
                mrc_01HMERCHANTTENANT00001
              </span>{' '}
              • Verified Commercial Shipper
            </div>
          </div>
        </div>
        <span className="inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full bg-emerald-950/60 text-emerald-400 border border-emerald-500/30">
          <ShieldCheck className="w-3.5 h-3.5" />
          Multi-Tenant Isolation Active
        </span>
      </div>

      {/* 4 Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Shipped This Month</div>
            <div className="text-2xl font-bold text-white mt-1">842 Parcels</div>
            <div className="text-[11px] text-emerald-400 flex items-center gap-1 mt-1">
              <TrendingUp className="w-3 h-3" /> +18.4% vs last month
            </div>
          </div>
          <div className="p-3 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-400">
            <Package className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">In Transit Active</div>
            <div className="text-2xl font-bold text-sky-400 mt-1">54 Orders</div>
            <div className="text-[11px] text-slate-400 mt-1">En route to customers</div>
          </div>
          <div className="p-3 rounded-xl bg-sky-500/10 border border-sky-500/20 text-sky-400">
            <Clock className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Delivery Success</div>
            <div className="text-2xl font-bold text-emerald-400 mt-1">99.1%</div>
            <div className="text-[11px] text-emerald-400 flex items-center gap-1 mt-1">
              <CheckCircle2 className="w-3 h-3" /> Top Tier Merchant SLA
            </div>
          </div>
          <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <CheckCircle2 className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Webhooks Dispatched</div>
            <div className="text-2xl font-bold text-purple-400 mt-1">3,410</div>
            <div className="text-[11px] text-slate-400 mt-1">99.8% 200 OK delivery</div>
          </div>
          <div className="p-3 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400">
            <Webhook className="w-5 h-5" />
          </div>
        </div>
      </div>

      {/* Feature Tabs: Bulk Batch, API Keys, Webhooks */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-4">
        <div className="flex border-b border-slate-800 pb-3 gap-2">
          <button
            onClick={() => setActiveTab('BATCH')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer ${
              activeTab === 'BATCH'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Bulk Batch CSV Orders
          </button>
          <button
            onClick={() => setActiveTab('API')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer ${
              activeTab === 'API'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            B2B API Secret Keys
          </button>
          <button
            onClick={() => setActiveTab('WEBHOOKS')}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition cursor-pointer ${
              activeTab === 'WEBHOOKS'
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Webhook Events
          </button>
        </div>

        {activeTab === 'BATCH' && (
          <div className="space-y-4">
            <div className="p-6 border border-dashed border-slate-800 rounded-xl flex flex-col items-center justify-center text-center space-y-2 bg-slate-950/40">
              <UploadCloud className="w-8 h-8 text-blue-400" />
              <div className="text-xs font-semibold text-slate-200">
                Drag and drop your batch shipment CSV file
              </div>
              <div className="text-[11px] text-slate-500">
                Columns: recipient_name, phone, address_line1, city, weight_kg, service_level
              </div>
              <button className="mt-2 px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-xs font-semibold border border-slate-700 transition cursor-pointer">
                Browse CSV File
              </button>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
                  <tr>
                    <th className="py-2.5 px-3">Batch ID</th>
                    <th className="py-2.5 px-3">Uploaded</th>
                    <th className="py-2.5 px-3">Parcels</th>
                    <th className="py-2.5 px-3">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60">
                  {[
                    { id: 'batch_20261030_01', time: 'Today, 10:15 AM', count: '120 parcels', status: 'PROCESSED' },
                    { id: 'batch_20261029_02', time: 'Yesterday, 3:30 PM', count: '85 parcels', status: 'PROCESSED' },
                  ].map((b) => (
                    <tr key={b.id} className="hover:bg-slate-800/30">
                      <td className="py-2.5 px-3 font-mono font-medium text-slate-200">{b.id}</td>
                      <td className="py-2.5 px-3 text-slate-400">{b.time}</td>
                      <td className="py-2.5 px-3 text-slate-300">{b.count}</td>
                      <td className="py-2.5 px-3">
                        <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-950/60 text-emerald-400 border border-emerald-500/30">
                          {b.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {activeTab === 'API' && (
          <div className="space-y-4 text-xs">
            <p className="text-slate-400">
              Integrate your ERP, WooCommerce, or Shopify store directly via SwiftDrop REST APIs using API Keys.
            </p>

            <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
              <div className="flex items-center justify-between">
                <span className="font-semibold text-slate-300 flex items-center gap-1.5">
                  <Key className="w-4 h-4 text-amber-400" />
                  Live Production Key
                </span>
                <span className="text-[10px] text-slate-500 font-mono">Header: X-API-Key</span>
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  readOnly
                  value="sd_live_a1b2c3d4_9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c"
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-1.5 font-mono text-[11px] text-amber-300 outline-none"
                />
                <button
                  onClick={copyKey}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-xs font-semibold flex items-center gap-1 shrink-0 transition cursor-pointer"
                >
                  <Copy className="w-3.5 h-3.5" />
                  {apiKeyCopied ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div className="text-[10px] text-slate-500">
                Scopes: <code className="text-slate-400 font-mono">["parcels:create", "parcels:read", "webhooks:manage"]</code>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'WEBHOOKS' && (
          <div className="space-y-3 text-xs">
            <p className="text-slate-400">
              SwiftDrop automatically posts signed HTTP webhooks whenever parcel status changes.
            </p>
            <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
              <div className="flex items-center justify-between">
                <span className="font-mono text-emerald-400 font-semibold">
                  https://vendor.com/api/swiftdrop/webhook
                </span>
                <span className="px-2 py-0.5 rounded text-[10px] bg-emerald-950/60 text-emerald-300 border border-emerald-500/30">
                  Active
                </span>
              </div>
              <div className="text-[11px] text-slate-400">
                Subscribed Events: <span className="text-slate-200 font-mono">PARCEL_PICKED_UP, OUT_FOR_DELIVERY, DELIVERED, ATTEMPT_FAILED</span>
              </div>
              <div className="text-[10px] text-slate-500">
                Signature Header: <span className="font-mono text-slate-400">X-SwiftDrop-Signature (HMAC-SHA256)</span>
              </div>
            </div>
          </div>
        )}
      </div>
    </DashboardLayout>
  );
};
