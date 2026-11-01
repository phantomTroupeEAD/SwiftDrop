import React, { useState } from 'react';
import { DashboardLayout } from './DashboardLayout';
import {
  Package,
  Truck,
  CheckCircle2,
  Clock,
  ArrowRight,
  MapPin,
} from 'lucide-react';

interface UnassignedParcel {
  trackingNumber: string;
  sender: string;
  recipientCity: string;
  weight: string;
  serviceLevel: 'EXPRESS' | 'STANDARD' | 'SAME_DAY';
  createdAt: string;
}

interface CourierRoster {
  id: string;
  name: string;
  vehicle: 'BIKE' | 'VAN' | 'TRUCK';
  zone: string;
  activeDeliveries: number;
  capacityLoad: number; // percentage
  status: 'AVAILABLE' | 'BUSY' | 'OFF_DUTY';
}

export const DispatcherDashboard: React.FC = () => {
  const [unassignedParcels, setUnassignedParcels] = useState<UnassignedParcel[]>([
    {
      trackingNumber: 'SD-20261030-99801',
      sender: 'Colombo Tech Stores',
      recipientCity: 'Dehiwala',
      weight: '2.5 kg',
      serviceLevel: 'EXPRESS',
      createdAt: '10 mins ago',
    },
    {
      trackingNumber: 'SD-20261030-99802',
      sender: 'Nimal Fernando',
      recipientCity: 'Colombo 03',
      weight: '0.8 kg',
      serviceLevel: 'SAME_DAY',
      createdAt: '18 mins ago',
    },
    {
      trackingNumber: 'SD-20261030-99803',
      sender: 'Apex Gadgets',
      recipientCity: 'Kandy Urban',
      weight: '4.2 kg',
      serviceLevel: 'STANDARD',
      createdAt: '35 mins ago',
    },
  ]);

  const [couriers] = useState<CourierRoster[]>([
    {
      id: 'cr_01',
      name: 'Kamal Perera',
      vehicle: 'BIKE',
      zone: 'ZONE_COLOMBO_01',
      activeDeliveries: 4,
      capacityLoad: 65,
      status: 'AVAILABLE',
    },
    {
      id: 'cr_02',
      name: 'Sunil Wickrama',
      vehicle: 'VAN',
      zone: 'ZONE_COLOMBO_02',
      activeDeliveries: 9,
      capacityLoad: 85,
      status: 'BUSY',
    },
    {
      id: 'cr_03',
      name: 'Dinesh Silva',
      vehicle: 'BIKE',
      zone: 'ZONE_COLOMBO_01',
      activeDeliveries: 1,
      capacityLoad: 20,
      status: 'AVAILABLE',
    },
    {
      id: 'cr_04',
      name: 'Anura Bandara',
      vehicle: 'TRUCK',
      zone: 'ZONE_NEGOMBO_01',
      activeDeliveries: 6,
      capacityLoad: 50,
      status: 'AVAILABLE',
    },
  ]);

  const [assignedNotice, setAssignedNotice] = useState<string | null>(null);

  const handleAssign = (tracking: string) => {
    setUnassignedParcels((prev) => prev.filter((p) => p.trackingNumber !== tracking));
    setAssignedNotice(`Parcel ${tracking} successfully assigned to Kamal Perera (Bike).`);
    setTimeout(() => setAssignedNotice(null), 4000);
  };

  return (
    <DashboardLayout
      activeRole="DISPATCHER"
      title="Central Dispatch & Courier Operations Board"
      subtitle="Real-time parcel queue, courier fleet capacity roster, zone balancing, and automated dispatch assignment."
    >
      {/* 4 Stat Overview Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Unassigned Queue</div>
            <div className="text-2xl font-bold text-amber-400 mt-1">
              {unassignedParcels.length} Bookings
            </div>
            <div className="text-[11px] text-amber-400/80 flex items-center gap-1 mt-1">
              <Clock className="w-3 h-3" /> Awaiting Courier Match
            </div>
          </div>
          <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400">
            <Package className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Couriers on Route</div>
            <div className="text-2xl font-bold text-sky-400 mt-1">16 Active</div>
            <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
              <Truck className="w-3 h-3" /> Fleet capacity 68%
            </div>
          </div>
          <div className="p-3 rounded-xl bg-sky-500/10 border border-sky-500/20 text-sky-400">
            <Truck className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">In Transit Today</div>
            <div className="text-2xl font-bold text-white mt-1">84 Parcels</div>
            <div className="text-[11px] text-emerald-400 flex items-center gap-1 mt-1">
              <CheckCircle2 className="w-3 h-3" /> Average Drop: 28m
            </div>
          </div>
          <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <Clock className="w-5 h-5" />
          </div>
        </div>

        <div className="p-5 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400 font-medium">Completed Drops</div>
            <div className="text-2xl font-bold text-white mt-1">210</div>
            <div className="text-[11px] text-slate-400 flex items-center gap-1 mt-1">
              <CheckCircle2 className="w-3 h-3 text-emerald-400" /> 98.4% SLA Target Met
            </div>
          </div>
          <div className="p-3 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400">
            <CheckCircle2 className="w-5 h-5" />
          </div>
        </div>
      </div>

      {assignedNotice && (
        <div className="p-3.5 rounded-xl bg-emerald-950/60 border border-emerald-500/40 text-emerald-300 text-xs flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{assignedNotice}</span>
        </div>
      )}

      {/* Main Grid: Unassigned Queue + Fleet Roster */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Unassigned Parcels Board */}
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold text-white">Unassigned Parcel Queue</h2>
              <p className="text-xs text-slate-400">
                Booked parcels requiring courier route allocation
              </p>
            </div>
            <span className="text-xs bg-amber-500/10 text-amber-400 border border-amber-500/20 px-2.5 py-1 rounded-lg">
              {unassignedParcels.length} Pending
            </span>
          </div>

          {unassignedParcels.length > 0 ? (
            <div className="space-y-3">
              {unassignedParcels.map((p) => (
                <div
                  key={p.trackingNumber}
                  className="p-3.5 rounded-xl bg-slate-950 border border-slate-800/80 flex items-center justify-between gap-3 text-xs"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono font-bold text-blue-400">
                        {p.trackingNumber}
                      </span>
                      <span
                        className={`text-[9px] font-bold px-1.5 py-0.5 rounded border ${
                          p.serviceLevel === 'SAME_DAY'
                            ? 'bg-rose-950/60 text-rose-300 border-rose-800/40'
                            : p.serviceLevel === 'EXPRESS'
                            ? 'bg-amber-950/60 text-amber-300 border-amber-800/40'
                            : 'bg-slate-800 text-slate-300 border-slate-700'
                        }`}
                      >
                        {p.serviceLevel}
                      </span>
                    </div>
                    <div className="text-slate-400 text-[11px]">
                      From: <span className="text-slate-200">{p.sender}</span> &rarr; Dest:{' '}
                      <span className="text-slate-200">{p.recipientCity}</span> ({p.weight})
                    </div>
                  </div>

                  <button
                    onClick={() => handleAssign(p.trackingNumber)}
                    className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-semibold text-xs transition flex items-center gap-1.5 shrink-0 shadow-sm cursor-pointer"
                  >
                    Assign
                    <ArrowRight className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-8 text-center text-xs text-slate-500 border border-dashed border-slate-800 rounded-xl">
              All bookings currently assigned to couriers.
            </div>
          )}
        </div>

        {/* Courier Fleet Roster */}
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold text-white">Courier Fleet Roster</h2>
              <p className="text-xs text-slate-400">
                Live driver availability, zone, and capacity load
              </p>
            </div>
            <span className="text-xs bg-slate-800 text-slate-300 px-2 py-0.5 rounded border border-slate-700">
              {couriers.length} Drivers
            </span>
          </div>

          <div className="space-y-2.5">
            {couriers.map((c) => (
              <div
                key={c.id}
                className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between text-xs"
              >
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-slate-800 flex items-center justify-center text-slate-300 font-bold">
                    <Truck className="w-4 h-4 text-sky-400" />
                  </div>
                  <div>
                    <div className="font-semibold text-slate-200 flex items-center gap-2">
                      {c.name}
                      <span className="text-[10px] text-slate-500 font-mono">({c.vehicle})</span>
                    </div>
                    <div className="text-[11px] text-slate-400 flex items-center gap-1">
                      <MapPin className="w-3 h-3 text-slate-500" />
                      {c.zone}
                    </div>
                  </div>
                </div>

                <div className="text-right space-y-1">
                  <span
                    className={`inline-block text-[10px] font-bold px-2 py-0.5 rounded border ${
                      c.status === 'AVAILABLE'
                        ? 'bg-emerald-950/60 text-emerald-300 border-emerald-800/40'
                        : 'bg-amber-950/60 text-amber-300 border-amber-800/40'
                    }`}
                  >
                    {c.status}
                  </span>
                  <div className="text-[10px] text-slate-400">
                    Load: <span className="font-semibold text-slate-200">{c.capacityLoad}%</span> ({c.activeDeliveries} drops)
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
};
