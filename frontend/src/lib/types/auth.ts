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
};

export type ValidateResponse = {
  valid: boolean;
  user_id?: number;
  username?: string;
};
