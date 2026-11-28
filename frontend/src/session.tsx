import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import type { UserSummary } from "./types/api";
import { getCurrentUser, login, logout } from "./services/auth";
import { onSessionExpired } from "./services/api";
type Session = {
  user: UserSummary | null;
  loading: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};
const Context = createContext<Session | null>(null);
export function useSession() {
  const value = useContext(Context);
  if (!value) throw new Error("Session provider missing");
  return value;
}
export function hasRole(user: UserSummary, role: string) {
  return (user.roles || []).includes(role);
}
export function hasPermission(user: UserSummary, permission: string) {
  return (user.permissions || []).includes(permission);
}
export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserSummary | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let active = true;
    onSessionExpired(() => setUser(null));
    getCurrentUser()
      .then((value) => {
        if (active) setUser(value);
      })
      .catch(() => {})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      onSessionExpired(null);
    };
  }, []);
  async function signIn(email: string, password: string) {
    const result = await login(email, password);
    setUser(result.user);
  }
  async function signOut() {
    await logout();
    setUser(null);
  }
  return <Context.Provider value={{ user, loading, signIn, signOut }}>{children}</Context.Provider>;
}
export function Protected({
  children,
  role,
  permission,
}: {
  children: ReactNode;
  role?: string;
  permission?: string;
}) {
  const { user, loading } = useSession();
  const location = useLocation();
  if (loading) return <main role="status">Restoring your session…</main>;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  if (role && !hasRole(user, role)) return <main role="alert">Access denied.</main>;
  if (permission && !hasPermission(user, permission))
    return <main role="alert">Access denied.</main>;
  return <>{children}</>;
}
