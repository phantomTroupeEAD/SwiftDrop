/**
 * SwiftDrop Canonical Test Data & Seed Constants
 * Standardized across all E2E test journeys and isolated contexts.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */

export const TEST_USERS = {
  CUSTOMER: {
    email: 'customer@swiftdrop.local',
    password: 'Customer123!',
    role: 'CUSTOMER',
    name: 'Dilshan Silva',
    phone: '+94771234567',
  },
  MERCHANT_ALPHA: {
    email: 'alpha@vendor.local',
    password: 'Merchant123!',
    role: 'MERCHANT',
    tenantId: 'mrc_01HMERCHANTALPHA0001',
    name: 'Merchant Alpha Store',
    apiKey: 'sk_test_alpha_01',
    webhookSecret: 'whsec_alpha_super_secret_test_key_01',
  },
  MERCHANT_BETA: {
    email: 'beta@vendor.local',
    password: 'Merchant123!',
    role: 'MERCHANT',
    tenantId: 'mrc_01HMERCHANTBETA00002',
    name: 'Merchant Beta Supplies',
    apiKey: 'sk_test_beta_02',
    webhookSecret: 'whsec_beta_super_secret_test_key_02',
  },
  DISPATCHER: {
    email: 'dispatcher@swiftdrop.local',
    password: 'Dispatch123!',
    role: 'DISPATCHER',
    name: 'Kamal Perera',
  },
  COURIER: {
    email: 'courier@swiftdrop.local',
    password: 'Courier123!',
    role: 'COURIER',
    name: 'Sunil Fernando',
    vehicle: 'Motorcycle (WP BCD-1234)',
    phone: '+94719876543',
  },
  ADMIN: {
    email: 'admin@swiftdrop.local',
    password: 'Admin123!',
    role: 'ADMIN',
    name: 'System Administrator',
  },
} as const;

export const TEST_LOCATIONS = {
  COLOMBO_CENTRAL: {
    zone: 'WESTERN',
    postalCode: '00100',
    address: 'No. 45, Galle Road, Colombo 03',
    city: 'Colombo',
  },
  KANDY_HUB: {
    zone: 'CENTRAL',
    postalCode: '20000',
    address: 'No. 12, Peradeniya Road, Kandy',
    city: 'Kandy',
  },
  GALLE_BRANCH: {
    zone: 'SOUTHERN',
    postalCode: '80000',
    address: 'No. 88, Matara Road, Galle',
    city: 'Galle',
  },
} as const;

export const TEST_PARCELS = {
  STANDARD_BOX: {
    weightKg: 2.5,
    lengthCm: 25,
    widthCm: 15,
    heightCm: 10,
    description: 'Electronics & Accessories',
    expectedBaseFeeLKR: 450,
  },
  HEAVY_CRATE: {
    weightKg: 18.0,
    lengthCm: 50,
    widthCm: 50,
    heightCm: 50,
    description: 'Machinery Spare Parts',
    expectedBaseFeeLKR: 2800,
  },
} as const;

export const TEST_ENDPOINTS = {
  GATEWAY_URL: process.env.GATEWAY_URL || 'http://localhost:80',
  WEB_PORTAL_URL: process.env.BASE_URL || 'http://localhost:3000',
  MOCK_WEBHOOK_URL: process.env.MOCK_WEBHOOK_URL || 'http://localhost:9999',
  AI_ASSISTANT_URL: process.env.AI_ASSISTANT_URL || 'http://localhost:8000',
  TRACKING_API_URL: process.env.TRACKING_API_URL || 'http://localhost:8081',
} as const;
