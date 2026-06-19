import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AppLayout } from "@/components/layout/AppLayout";
import { ProtectedRoute, PublicRoute, SetupRoute } from "./ProtectedRoute";
import { SetupPage } from "@/pages/SetupPage";
import { LoginPage } from "@/pages/LoginPage";
import { DashboardPage } from "@/pages/DashboardPage";
import { FilesPage } from "@/pages/FilesPage";
import { NodesPage } from "@/pages/NodesPage";
import { NodeDetailPage } from "@/pages/NodeDetailPage";
import { CredentialsPage } from "@/pages/CredentialsPage";
import { KeysPage } from "@/pages/KeysPage";
import { UsersPage } from "@/pages/UsersPage";
import { TransfersPage } from "@/pages/TransfersPage";
import { ConfigPage } from "@/pages/ConfigPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { BackupPage } from "@/pages/BackupPage";

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<SetupRoute />}>
          <Route path="/setup" element={<SetupPage />} />
        </Route>

        <Route element={<PublicRoute />}>
          <Route path="/login" element={<LoginPage />} />
        </Route>

        <Route element={<ProtectedRoute />}>
          <Route element={<AppLayout />}>
            <Route index element={<DashboardPage />} />
            <Route path="files" element={<FilesPage />} />
            <Route path="nodes" element={<NodesPage />} />
            <Route path="nodes/:id" element={<NodeDetailPage />} />
            <Route path="transfers" element={<TransfersPage />} />
          </Route>
        </Route>

        <Route element={<ProtectedRoute operator />}>
          <Route element={<AppLayout />}>
            <Route path="credentials" element={<CredentialsPage />} />
          </Route>
        </Route>

        <Route element={<ProtectedRoute admin />}>
          <Route element={<AppLayout />}>
            <Route path="keys" element={<KeysPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route path="config" element={<ConfigPage />} />
            <Route path="settings" element={<SettingsPage />} />
            <Route path="backup" element={<BackupPage />} />
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
