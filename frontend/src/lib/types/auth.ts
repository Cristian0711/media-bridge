export type LoginRequest = {
  username: string;
  password: string;
};

export type LoginResponse = {
  token: string;
};

export type RegisterRequest = {
  username: string;
  password: string;
  key: string;
};

export type RegisterResponse = {
  id: number;
  username: string;
  role?: string;
};

export type ValidateResponse = {
  valid: boolean;
  user_id?: number;
  username?: string;
  role?: string;
};

export type UserRole = 'admin' | 'user';

export type CurrentUser = {
  id: number;
  username: string;
  role: UserRole;
};

export type GenerateKeyResponse = {
  key: string;
};

export type InviteKeyStatus = 'available' | 'used';

export type InviteKey = {
  value: string;
  is_active: boolean;
  status: InviteKeyStatus;
  used_at?: string;
  created_at: string;
};

export type ListKeysResponse = {
  keys: InviteKey[];
};
