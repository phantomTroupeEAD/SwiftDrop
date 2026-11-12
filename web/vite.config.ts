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
          if (process.env.SWIFTDROP_DEMO_AUTH === '1' && req.url === '/api/v1/auth/refresh' && req.method === 'POST') {
            res.writeHead(401, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: { message: 'Demo sessions end on reload.' } }));
            return;
          }
          if (req.url === '/api/v1/auth/login' && req.method === 'POST') {
            let rawBody = '';
            req.on('data', (chunk) => {
              rawBody += chunk;
            });
            req.on('end', async () => {
              // Try forwarding to real Go backend first if it's running
              try {
                if (process.env.SWIFTDROP_DEMO_AUTH === '1') {
                  throw new Error('Local demo sign-in is enabled');
                }
                const backendRes = await fetch('http://127.0.0.1:8080/api/v1/auth/login', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: rawBody,
                });
                const data = await backendRes.text();
                const cookies = backendRes.headers.getSetCookie();
                if (cookies.length) res.setHeader('Set-Cookie', cookies);
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
      '/api/v1/tracking': {
        target: 'http://127.0.0.1:8081',
        changeOrigin: true,
      },
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
});
