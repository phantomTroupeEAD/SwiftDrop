export type Role = 'ADMIN' | 'DISPATCHER' | 'COURIER' | 'MERCHANT' | 'CUSTOMER';
export type UserStatus = 'ACTIVE' | 'SUSPENDED';

export interface User {
  id: string;
  email: string;
  first_name?: string;
  last_name?: string;
  display_name?: string;
  phone?: string;
  role: Role;
  tenant_id: string | null;
  status?: UserStatus;
  email_verified?: boolean;
  requires_phone?: boolean;
  created_at?: string;
}


export interface LoginResponseData {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}

export interface GoogleLoginResponseData extends LoginResponseData {
  is_new_user: boolean;
  requires_phone: boolean;
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
