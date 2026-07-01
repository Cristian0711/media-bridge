const TOKEN_KEY = 'media_bridge_token';
/** Cookie read by nginx for API + SPA route protection (matches nginx/lua/auth.lua). */
export const AUTH_COOKIE_NAME = 'auth_token';

/** 90 days — aligned with backend JWT TTL. */
const COOKIE_MAX_AGE_SEC = 90 * 24 * 60 * 60;

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

function writeCookie(name: string, value: string, maxAgeSec: number): void {
  if (typeof document === 'undefined') return;
  const secure = typeof location !== 'undefined' && location.protocol === 'https:' ? '; Secure' : '';
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${maxAgeSec}; SameSite=Lax${secure}`;
}

function clearCookie(name: string): void {
  if (typeof document === 'undefined') return;
  const secure = typeof location !== 'undefined' && location.protocol === 'https:' ? '; Secure' : '';
  document.cookie = `${name}=; path=/; max-age=0; SameSite=Lax${secure}`;
}

/** Sync cookie → localStorage when nginx set cookie or storage was cleared. */
export function syncTokenFromCookie(): void {
  if (typeof localStorage === 'undefined') return;
  if (localStorage.getItem(TOKEN_KEY)) return;
  const fromCookie = readCookie(AUTH_COOKIE_NAME);
  if (fromCookie) {
    localStorage.setItem(TOKEN_KEY, fromCookie);
  }
}

export function getToken(): string | null {
  if (typeof localStorage === 'undefined') return null;
  const stored = localStorage.getItem(TOKEN_KEY);
  if (stored) return stored;
  const fromCookie = readCookie(AUTH_COOKIE_NAME);
  if (fromCookie) {
    localStorage.setItem(TOKEN_KEY, fromCookie);
    return fromCookie;
  }
  return null;
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
  writeCookie(AUTH_COOKIE_NAME, token, COOKIE_MAX_AGE_SEC);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
  clearCookie(AUTH_COOKIE_NAME);
}

export function isAuthenticated(): boolean {
  return !!getToken();
}
