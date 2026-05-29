import { callApi } from '$lib/api/client';
import type {
  CurrentUser,
  GenerateKeyResponse,
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
  ValidateResponse,
} from '$lib/types/auth';
import { setToken } from '$lib/auth/session';

export async function login(req: LoginRequest): Promise<LoginResponse> {
  const res = await callApi<LoginResponse>('/auth/login', {
    method: 'POST',
    auth: false,
    body: JSON.stringify(req),
  });
  setToken(res.token);
  return res;
}

export async function register(req: RegisterRequest): Promise<RegisterResponse> {
  return callApi<RegisterResponse>('/auth/register', {
    method: 'POST',
    auth: false,
    body: JSON.stringify(req),
  });
}

export async function validateToken(): Promise<ValidateResponse> {
  return callApi<ValidateResponse>('/auth/validate', { method: 'GET' });
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return callApi<CurrentUser>('/users/me', { method: 'GET' });
}

export async function generateRegistrationKey(): Promise<GenerateKeyResponse> {
  return callApi<GenerateKeyResponse>('/keys/generate', { method: 'POST' });
}
