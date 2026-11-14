import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

// Mock accounts for local frontend development before Go / Postgres is running
const MOCK_USERS: Record<string, { role: string; name: string; tenant_id: string | null }> = {
  'admin@swiftdrop.local': { role: 'ADMIN', name: 'System Administrator', tenant_id: null },
  'dispatcher@swiftdrop.local': { role: 'DISPATCHER', name: 'Central Dispatcher', tenant_id: null },
  'courier@swiftdrop.local': { role: 'COURIER', name: 'Kamal Perera (Courier)', tenant_id: null },
  'merchant@vendor.com': { role: 'MERCHANT', name: 'Colombo Tech Stores', tenant_id: 'mrc_01HMERCHANTTENANT00001' },
  'customer@swiftdrop.local': { role: 'CUSTOMER', name: 'Nimal Fernando', tenant_id: null },
};

export default defineConfig({
  plugins: [
    react(),
    {
      name: 'dev-fallback-auth-middleware',
      configureServer(server) {
        // Runs before proxy: if backend is not running, seamlessly provides dev mock auth
        server.middlewares.use(async (req, res, next) => {
          if (req.url === '/api/v1/auth/login' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              // Try forwarding to real Go backend first if it's running
              try {
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/login', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                res.writeHead(backendRes.status, { 'Content-Type': 'application/json' });
                res.end(data);
                return;
              } catch (_) {
                // Backend not running (e.g. Go not installed yet) -> Seamless dev mock response!
                try {
                  const parsed = JSON.parse(rawBody);
                  const email = (parsed.email || '').trim().toLowerCase();
                  const mock = MOCK_USERS[email] || {
                    role: 'CUSTOMER',
                    name: 'Demo User',
                    tenant_id: null,
                  };

                  const response = {
                    data: {
                      access_token: `mock_jwt_token_for_${mock.role}_${Date.now()}`,
                      token_type: 'Bearer',
                      expires_in: 900,
                      user: {
                        id: `usr_${mock.role.toLowerCase()}_001`,
                        email: email || 'customer@swiftdrop.local',
                        display_name: mock.name,
                        role: mock.role,
                        tenant_id: mock.tenant_id,
                      },
                    },
                  };

                  res.writeHead(200, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify(response));
                } catch (e) {
                  res.writeHead(400, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify({ error: { message: 'Invalid JSON payload' } }));
                }
              }
            });
            return;
          }

          if (req.url === '/api/v1/auth/forgot-password/send-otp' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              try {
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/forgot-password/send-otp', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                res.writeHead(backendRes.status, { 'Content-Type': 'application/json' });
                res.end(data);
                return;
              } catch (_) {
                res.writeHead(503, { 'Content-Type': 'application/json' });
                res.end(
                  JSON.stringify({
                    error: {
                      message:
                        'Core API backend is not running on port 8080. Please start api.exe in core-api to send real emails.',
                    },
                  })
                );
              }
            });
            return;
          }

          if (req.url === '/api/v1/auth/forgot-password/reset' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              try {
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/forgot-password/reset', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                res.writeHead(backendRes.status, { 'Content-Type': 'application/json' });
                res.end(data);
                return;
              } catch (_) {
                res.writeHead(503, { 'Content-Type': 'application/json' });
                res.end(
                  JSON.stringify({
                    error: {
                      message:
                        'Core API backend is not running on port 8080. Please start api.exe in core-api to verify OTP.',
                    },
                  })
                );
              }
            });
            return;
          }

          if (req.url === '/api/v1/auth/register/send-otp' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              try {
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/register/send-otp', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                res.writeHead(backendRes.status, { 'Content-Type': 'application/json' });
                res.end(data);
                return;
              } catch (_) {
                try {
                  const parsed = JSON.parse(rawBody);
                  const email = (parsed.email || '').trim();
                  console.log(`\x1b[33m[DEV AUTH MOCK]\x1b[0m Registration OTP for ${email}: 123456`);
                  const response = {
                    data: {
                      message: 'Verification OTP has been sent to your email',
                      email: email,
                      expires_in: 600,
                      cooldown_seconds: 60,
                    },
                  };
                  res.writeHead(200, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify(response));
                } catch (e) {
                  res.writeHead(400, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify({ error: { message: 'Invalid JSON payload' } }));
                }
              }
            });
            return;
          }

          if (req.url === '/api/v1/auth/register' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              try {
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/register', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                res.writeHead(backendRes.status, { 'Content-Type': 'application/json' });
                res.end(data);
                return;
              } catch (_) {
                try {
                  const parsed = JSON.parse(rawBody);
                  const email = (parsed.email || '').trim();
                  const response = {
                    data: {
                      user: {
                        id: `usr_customer_${Date.now()}`,
                        email: email,
                        display_name: `${parsed.first_name || ''} ${parsed.last_name || ''}`.trim() || 'New User',
                        role: 'CUSTOMER',
                        status: 'ACTIVE',
                      },
                      is_new_user: true,
                    },
                  };
                  res.writeHead(201, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify(response));
                } catch (e) {
                  res.writeHead(400, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify({ error: { message: 'Invalid JSON payload' } }));
                }
              }
            });
            return;
          }


          if (req.url === '/api/v1/auth/google' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              try {
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/google', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                res.writeHead(backendRes.status, { 'Content-Type': 'application/json' });
                res.end(data);
                return;
              } catch (_) {
                // Backend not running -> Seamless dev mock response for Google login
                try {
                  const parsed = JSON.parse(rawBody);
                  let email = 'customer.google@swiftdrop.local';
                  let name = 'Google Customer';
                  if (parsed.id_token) {
                    try {
                      const payloadBase64 = parsed.id_token.split('.')[1];
                      if (payloadBase64) {
                        const decoded = JSON.parse(Buffer.from(payloadBase64, 'base64').toString());
                        if (decoded.email) email = decoded.email;
                        if (decoded.name) name = decoded.name;
                      }
                    } catch {}
                  }

                  const response = {
                    data: {
                      access_token: `mock_jwt_token_for_CUSTOMER_${Date.now()}`,
                      token_type: 'Bearer',
                      expires_in: 900,
                      user: {
                        id: `usr_customer_google_${Date.now()}`,
                        email: email,
                        display_name: name,
                        role: 'CUSTOMER',
                        status: 'ACTIVE',
                        requires_phone: false,
                        tenant_id: null,
                      },
                      is_new_user: false,
                      requires_phone: false,
                    },
                  };

                  res.writeHead(200, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify(response));
                } catch (e) {
                  res.writeHead(400, { 'Content-Type': 'application/json' });
                  res.end(JSON.stringify({ error: { message: 'Invalid JSON payload' } }));
                }
              }
            });
            return;
          }

          next();
        });
      },
    },
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
});
