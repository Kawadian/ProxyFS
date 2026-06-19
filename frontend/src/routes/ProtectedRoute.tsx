import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { LoadingSpinner } from "@/components/ui/Badge";

export function ProtectedRoute({ admin, operator }: { admin?: boolean; operator?: boolean }) {
  const { state, isAdmin, isOperator } = useAuth();

  if (state === "loading") {
    return <LoadingSpinner />;
  }

  if (state === "setup") {
    return <Navigate to="/setup" replace />;
  }

  if (state === "unauthenticated") {
    return <Navigate to="/login" replace />;
  }

  if (admin && !isAdmin) {
    return <Navigate to="/" replace />;
  }

  if (operator && !isOperator) {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}

export function PublicRoute() {
  const { state } = useAuth();

  if (state === "loading") {
    return <LoadingSpinner />;
  }

  if (state === "setup") {
    return <Navigate to="/setup" replace />;
  }

  if (state === "authenticated") {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}

export function SetupRoute() {
  const { state } = useAuth();

  if (state === "loading") {
    return <LoadingSpinner />;
  }

  if (state === "authenticated") {
    return <Navigate to="/" replace />;
  }

  if (state === "unauthenticated") {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
