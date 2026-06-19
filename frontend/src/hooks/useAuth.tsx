import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/api/endpoints";
import { setCsrfToken, checkSetupStatus } from "@/api/client";
import type { User } from "@/api/types";

type AuthState = "loading" | "setup" | "unauthenticated" | "authenticated";

interface AuthContextValue {
  state: AuthState;
  user: User | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setup: (data: { username: string; password: string; display_name?: string }) => Promise<void>;
  isAdmin: boolean;
  isOperator: boolean;
  canWrite: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [state, setState] = useState<AuthState>("loading");
  const [user, setUser] = useState<User | null>(null);

  const refresh = useCallback(async () => {
    try {
      const res = await authApi.me();
      setCsrfToken(res.csrf_token);
      setUser(res.user);
      setState("authenticated");
      return true;
    } catch {
      const setupStatus = await checkSetupStatus();
      setCsrfToken(null);
      setUser(null);
      setState(setupStatus === "needs_setup" ? "setup" : "unauthenticated");
      return false;
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const login = async (username: string, password: string) => {
    const res = await authApi.login({ username, password });
    setCsrfToken(res.csrf_token);
    setUser(res.user);
    setState("authenticated");
    queryClient.clear();
  };

  const logout = async () => {
    try {
      await authApi.logout();
    } finally {
      setCsrfToken(null);
      setUser(null);
      setState("unauthenticated");
      queryClient.clear();
    }
  };

  const setup = async (data: { username: string; password: string; display_name?: string }) => {
    const res = await authApi.setup(data);
    setCsrfToken(res.csrf_token);
    setUser(res.user);
    setState("authenticated");
    queryClient.clear();
  };

  const value = useMemo(
    () => ({
      state,
      user,
      login,
      logout,
      setup,
      isAdmin: user?.role === "admin",
      isOperator: user?.role === "admin" || user?.role === "operator",
      canWrite: user?.role === "admin" || user?.role === "operator",
    }),
    [state, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function useCurrentUser() {
  return useQuery({
    queryKey: ["auth", "me"],
    queryFn: authApi.me,
    staleTime: 60_000,
    retry: false,
  });
}
