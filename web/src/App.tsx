import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { AuthProvider, useAuth } from '@/app/providers/AuthContext';
import { LoginPage } from '@/features/auth/LoginPage';
import { RegisterPage } from '@/features/auth/RegisterPage';
import { LandingPage } from '@/features/landing/LandingPage';
import { AdminDashboard } from '@/features/dashboards/AdminDashboard';
import { DispatcherDashboard } from '@/features/dashboards/DispatcherDashboard';
import { CourierDashboard } from '@/features/dashboards/CourierDashboard';
import { RoleHome } from '@/features/dashboards/RoleHome';
import type { Role } from '@/shared/types/auth';
import { BookingPage } from '@/features/customer-merchant/portal/BookingPage';
import { MerchantApplication, MerchantPage } from '@/features/customer-merchant/portal/MerchantPage';
import { TrackingPage } from '@/features/customer-merchant/portal/TrackingPage';
import { DesignSystemPage } from '@/features/design-system/DesignSystemPage';

function CustomerBookingAlias() {
  const { search } = useLocation();
  return <Navigate to={{ pathname: '/portal/book', search }} replace />;
}

// Protected Route Guard
function SignedInHome() {
  const { user } = useAuth();
  switch (user?.role) {
    case 'ADMIN': return <Navigate to="/admin/dashboard" replace />;
    case 'DISPATCHER': return <Navigate to="/dispatcher/dashboard" replace />;
    case 'COURIER': return <Navigate to="/courier/dashboard" replace />;
    default: return <RoleHome />;
  }
}

function MerchantSessionGate({ children }: { children: React.ReactNode }) {
  const { user, refreshSession } = useAuth();
  const [attempted, setAttempted] = React.useState(false);

  React.useEffect(() => {
    if (user?.role !== 'CUSTOMER') return;
    let active = true;
    void refreshSession().finally(() => { if (active) setAttempted(true); });
    return () => { active = false; };
  }, [user?.id, user?.role, refreshSession]);

  if (user?.role === 'MERCHANT') return <>{children}</>;
  if (user?.role === 'CUSTOMER' && !attempted) return <p role="status">Checking merchant access…</p>;
  return <Navigate to="/dashboard" replace />;
}

const ProtectedRoute: React.FC<{ children: React.ReactNode; role?: Role }> = ({ children, role }) => {
  const { isAuthenticated, isRestoringSession, user } = useAuth();
  if (isRestoringSession) return <p role="status">Restoring your session…</p>;
  if (!isAuthenticated) {
    return <Navigate to="/auth/login" replace />;
  }
  if (role === 'MERCHANT') return <MerchantSessionGate>{children}</MerchantSessionGate>;
  if (role && user?.role !== role) return <Navigate to="/dashboard" replace />;
  return <>{children}</>;
};

export const AppRoutes: React.FC = () => {
  return (
    <Routes>
      <Route path="/auth/login" element={<LoginPage />} />
      <Route path="/auth/register" element={<RegisterPage />} />
      <Route path="/register" element={<Navigate to="/auth/register" replace />} />
      <Route path="/portal/register" element={<Navigate to="/auth/register" replace />} />
      <Route path="/track" element={<TrackingPage />} />
      <Route path="/track/:trackingNumber" element={<TrackingPage />} />
      <Route path="/portal/tracking/:trackingNumber?" element={<ProtectedRoute><TrackingPage key="private" privateView /></ProtectedRoute>} />
      <Route path="/portal/book" element={<ProtectedRoute role="CUSTOMER"><BookingPage /></ProtectedRoute>} />
      <Route path="/customer/book" element={<CustomerBookingAlias />} />
      <Route path="/portal/merchants/apply" element={<ProtectedRoute role="CUSTOMER"><MerchantApplication /></ProtectedRoute>} />
      <Route path="/portal/merchants/orders" element={<ProtectedRoute role="MERCHANT"><MerchantPage key="orders" section="orders" /></ProtectedRoute>} />
      <Route path="/portal/merchants/api-keys" element={<ProtectedRoute role="MERCHANT"><MerchantPage key="api-keys" section="api-keys" /></ProtectedRoute>} />
      <Route path="/portal/merchants/webhooks" element={<ProtectedRoute role="MERCHANT"><MerchantPage key="webhooks" section="webhooks" /></ProtectedRoute>} />

      {/* Dev-Only Design System Parity Studio (Gated out of production) */}
      {import.meta.env.DEV && (
        <Route path="/design-system" element={<DesignSystemPage />} />
      )}

      {/* Operations workspaces */}
      <Route
        path="/admin/dashboard"
        element={
          <ProtectedRoute role="ADMIN">
            <AdminDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dispatcher/dashboard"
        element={
          <ProtectedRoute role="DISPATCHER">
            <DispatcherDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/courier/dashboard"
        element={
          <ProtectedRoute role="COURIER">
            <CourierDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/merchant/dashboard"
        element={
          <ProtectedRoute role="MERCHANT">
            <Navigate to="/dashboard" replace />
          </ProtectedRoute>
        }
      />
      <Route
        path="/customer/dashboard"
        element={
          <ProtectedRoute role="CUSTOMER">
            <Navigate to="/dashboard" replace />
          </ProtectedRoute>
        }
      />

      {/* Aliases mapping to role consoles */}
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <SignedInHome />
          </ProtectedRoute>
        }
      />
      <Route
        path="/ops/dashboard"
        element={
          <ProtectedRoute>
            <Navigate to="/dashboard" replace />
          </ProtectedRoute>
        }
      />
      <Route
        path="/portal/dashboard"
        element={
          <ProtectedRoute>
            <Navigate to="/dashboard" replace />
          </ProtectedRoute>
        }
      />

      {/* Public Landing & Default Fallback */}
      <Route path="/" element={<LandingPage />} />
      <Route path="*" element={<Navigate to="/auth/login" replace />} />
    </Routes>
  );
};

export const App: React.FC = () => {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </AuthProvider>
  );
};

export default App;
