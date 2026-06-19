import { NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  LayoutDashboard,
  FolderOpen,
  Server,
  KeyRound,
  Shield,
  Users,
  ArrowLeftRight,
  FileCode,
  Settings,
  Database,
} from "lucide-react";
import { useAuth } from "@/hooks/useAuth";

const navItems = [
  { to: "/", icon: LayoutDashboard, labelKey: "nav.dashboard", end: true },
  { to: "/files", icon: FolderOpen, labelKey: "nav.files" },
  { to: "/nodes", icon: Server, labelKey: "nav.nodes" },
  { to: "/credentials", icon: KeyRound, labelKey: "nav.credentials", operator: true },
  { to: "/keys", icon: Shield, labelKey: "nav.keys", admin: true },
  { to: "/users", icon: Users, labelKey: "nav.users", admin: true },
  { to: "/transfers", icon: ArrowLeftRight, labelKey: "nav.transfers" },
  { to: "/config", icon: FileCode, labelKey: "nav.config", admin: true },
  { to: "/settings", icon: Settings, labelKey: "nav.settings", admin: true },
  { to: "/backup", icon: Database, labelKey: "nav.backup", admin: true },
];

interface SidebarProps {
  open: boolean;
  onClose: () => void;
}

export function Sidebar({ open, onClose }: SidebarProps) {
  const { t } = useTranslation();
  const { isAdmin, isOperator } = useAuth();

  const visible = navItems.filter((item) => {
    if (item.admin && !isAdmin) return false;
    if (item.operator && !isOperator) return false;
    return true;
  });

  return (
    <>
      {open && <div className="sidebar-backdrop" onClick={onClose} aria-hidden />}
      <aside className={`sidebar ${open ? "open" : ""}`} aria-label="Main navigation">
        <div className="sidebar-header">
          <div className="sidebar-logo" aria-hidden>LX</div>
          <span className="sidebar-title">{t("app.name")}</span>
        </div>
        <nav className="sidebar-nav">
          {visible.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => `nav-link ${isActive ? "active" : ""}`}
              onClick={onClose}
            >
              <item.icon size={18} aria-hidden />
              {t(item.labelKey)}
            </NavLink>
          ))}
        </nav>
      </aside>
    </>
  );
}
