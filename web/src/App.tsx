import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from '@/app/providers/AuthContext';
import { LoginPage } from '@/features/auth/LoginPage';
import { AdminDashboard } from '@/features/dashboards/AdminDashboard';
import { DispatcherDashboard } from '@/features/dashboards/DispatcherDashboard';
import { CourierDashboard } from '@/features/dashboards/CourierDashboard';
import { MerchantDashboard } from '@/features/dashboards/MerchantDashboard';
import { CustomerDashboard } from '@/features/dashboards/CustomerDashboard';

// Protected Route Guard
const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) {
    return <Navigate to="/auth/login" replace />;
  }
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
      return <Navigate to="/merchant/dashboard" replace />;
    default:
      return <Navigate to="/customer/dashboard" replace />;
  }
};

export const AppRoutes: React.FC = () => {
  return (
    <Routes>
      <Route path="/auth/login" element={<LoginPage />} />

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
