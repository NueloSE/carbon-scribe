import type { AuthState } from "./auth.types";

export const selectUser = (s: AuthState) => s.user;
export const selectToken = (s: AuthState) => s.token;
export const selectIsAuthenticated = (s: AuthState) => s.isAuthenticated;
export const selectAuthError = (s: AuthState) => s.error;
export const selectIsHydrated = (s: AuthState) => s.isHydrated;

export const selectUserName = (s: AuthState) => s.user?.full_name ?? "";
export const selectUserRole = (s: AuthState) => s.user?.role ?? "farmer";
