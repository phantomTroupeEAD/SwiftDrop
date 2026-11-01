export type Role = 'ADMIN' | 'DISPATCHER' | 'COURIER' | 'MERCHANT' | 'CUSTOMER';

export interface User {
  id: string;
  email: string;
  display_name?: string;
  role: Role;
  tenant_id: string | null;
}

export interface LoginResponseData {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}

export interface ApiEnvelope<T> {
  data: T;
}

export interface ApiErrorEnvelope {
  error: {
    code: string;
    message: string;
    timestamp: string;
  };
}
