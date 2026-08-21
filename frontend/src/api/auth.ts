import { api } from "./client";
import type { AuthResponse, LoginInput, SignupInput, User } from "./types";

// Signup and login both start a session (the backend returns an access token
// and sets the refresh cookie in one shot), so their callers share a shape.
export async function signup(input: SignupInput): Promise<AuthResponse> {
  const res = await api.post<AuthResponse>("/auth/signup", input, { withCredentials: true });
  return res.data;
}

export async function login(input: LoginInput): Promise<AuthResponse> {
  const res = await api.post<AuthResponse>("/auth/login", input, { withCredentials: true });
  return res.data;
}

// Ends every session server-side and expires the refresh cookie. The access
// token is stateless, so dropping it locally is the caller's job.
export async function logout(): Promise<void> {
  await api.post("/auth/logout", undefined, { withCredentials: true });
}

// Exchanges the HttpOnly cookie for a fresh session. Used on page load when no
// access token exists; the 401 interceptor uses its own single-flight version
// (see client.ts) so concurrent failures don't fire this twice at once.
export async function refresh(): Promise<AuthResponse> {
  const res = await api.post<AuthResponse>("/auth/refresh", undefined, { withCredentials: true });
  return res.data;
}

// Identity behind the current access token. Cheaper than refreshing on page
// load: it answers "who am I" without rotating a perfectly good session.
export async function getCurrentUser(): Promise<User> {
  const res = await api.get<User>("/auth/me");
  return res.data;
}
