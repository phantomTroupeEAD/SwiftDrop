import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from '@/app/providers/AuthContext';
import { LoginPage } from '@/features/auth/LoginPage';
import { RegisterPage } from '@/features/auth/RegisterPage';
import { AdminDashboard } from '@/features/dashboards/AdminDashboard';
import { DispatcherDashboard } from '@/features/dashboards/DispatcherDashboard';
import { CourierDashboard } from '@/features/dashboards/CourierDashboard';
import { MerchantDashboard } from '@/features/dashboards/MerchantDashboard';
import { CustomerDashboard } from '@/features/dashboards/CustomerDashboard';
import { DesignPreviewPage } from '@/features/customer-merchant/preview/DesignPreviewPage';
import { BookingPage } from '@/features/customer-merchant/portal/BookingPage';
import { MerchantApplication, MerchantPage } from '@/features/customer-merchant/portal/MerchantPage';
import { RegisterPage as PortalRegisterPage } from '@/features/customer-merchant/portal/RegisterPage';
import { TrackingPage } from '@/features/customer-merchant/portal/TrackingPage';

// Protected Route Guard
const ProtectedRoute: React.FC<{ children: React.ReactNode; role?: 'CUSTOMER' | 'MERCHANT' }> = ({ children, role }) => {
  const { isAuthenticated, user } = useAuth();
  if (!isAuthenticated) {
    return <Navigate to="/auth/login" replace />;
  }
  if (role && user?.role !== role) return <Navigate to="/dashboard" replace />;
  return <>{children}</>;
};

// Automatic Role Route Redirector
const RoleRedirector: React.FC = () => {
  const { user } = useAuth();
  if (!user) return <Navigate to="/auth/login" replace />;

  switch (user.role) {
    case 'ADMIN':
      return <Navigate to="/admin/dashboard" replace />;
    case 'DISPATCHER':
      return <Navigate to="/dispatcher/dashboard" replace />;
    case 'COURIER':
      return <Navigate to="/courier/dashboard" replace />;
    case 'MERCHANT':
      return <Navigate to="/portal/merchants/orders" replace />;
    default:
      return <Navigate to="/portal/book" replace />;
  }
};

export const AppRoutes: React.FC = () => {
  return (
    <Routes>
      <Route path="/auth/login" element={<LoginPage />} />
      <Route path="/auth/register" element={<RegisterPage />} />
      <Route path="/register" element={<Navigate to="/auth/register" replace />} />
      <Route path="/portal/register" element={<PortalRegisterPage />} />
      <Route path="/track" element={<TrackingPage />} />
      <Route path="/track/:trackingNumber" element={<TrackingPage />} />
      <Route path="/portal/book" element={<ProtectedRoute role="CUSTOMER"><BookingPage /></ProtectedRoute>} />
      <Route path="/portal/merchants/apply" element={<ProtectedRoute role="CUSTOMER"><MerchantApplication /></ProtectedRoute>} />
      <Route path="/portal/merchants/orders" element={<ProtectedRoute role="MERCHANT"><MerchantPage key="orders" section="orders" /></ProtectedRoute>} />
      <Route path="/portal/merchants/api-keys" element={<ProtectedRoute role="MERCHANT"><MerchantPage key="api-keys" section="api-keys" /></ProtectedRoute>} />
      <Route path="/portal/merchants/webhooks" element={<ProtectedRoute role="MERCHANT"><MerchantPage key="webhooks" section="webhooks" /></ProtectedRoute>} />

      {/* Visual Foundation & Design Preview (Unauthenticated evaluation studio) */}
      <Route path="/design-preview" element={<DesignPreviewPage />} />
      <Route path="/preview/design" element={<DesignPreviewPage />} />

      {/* 5 Distinct Mock Pages for Each Role */}
      <Route
        path="/admin/dashboard"
        element={
          <ProtectedRoute>
            <AdminDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/dispatcher/dashboard"
        element={
          <ProtectedRoute>
            <DispatcherDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/courier/dashboard"
        element={
          <ProtectedRoute>
            <CourierDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/merchant/dashboard"
        element={
          <ProtectedRoute>
            <MerchantDashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/customer/dashboard"
        element={
          <ProtectedRoute>
            <CustomerDashboard />
          </ProtectedRoute>
        }
      />

      {/* Aliases mapping to role consoles */}
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <RoleRedirector />
          </ProtectedRoute>
        }
      />
      <Route
        path="/ops/dashboard"
        element={
          <ProtectedRoute>
            <RoleRedirector />
          </ProtectedRoute>
        }
      />
      <Route
        path="/portal/dashboard"
        element={
          <ProtectedRoute>
            <RoleRedirector />
          </ProtectedRoute>
        }
      />

      {/* Default Fallback */}
      <Route path="/" element={<Navigate to="/auth/login" replace />} />
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
