import { useState } from "react";
import { Outlet } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Menu, LogOut, Globe, Sun, Moon, Monitor } from "lucide-react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { Sidebar } from "./Sidebar";
import { Button } from "@/components/ui/Button";
import { useAuth } from "@/hooks/useAuth";
import { useTheme } from "@/hooks/useTheme";

export function AppLayout() {
  const { t, i18n } = useTranslation();
  const { user, logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="app-layout">
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <div className="main-content">
        <header className="top-header">
          <div className="flex items-center gap-3">
            <Button
              variant="ghost"
              size="icon"
              className="mobile-menu-btn"
              onClick={() => setSidebarOpen(true)}
              aria-label="Open menu"
            >
              <Menu size={20} />
            </Button>
            <span className="text-sm text-muted">
              {user?.display_name || user?.username}
              <span className="badge badge-neutral" style={{ marginLeft: 8 }}>
                {user?.role}
              </span>
            </span>
          </div>
          <div className="flex items-center gap-2">
            <DropdownMenu.Root>
              <DropdownMenu.Trigger asChild>
                <Button variant="ghost" size="icon" aria-label={t("app.language")}>
                  <Globe size={18} />
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content
                  className="card"
                  style={{ padding: "0.5rem", minWidth: 120 }}
                  sideOffset={4}
                >
                  <DropdownMenu.Item
                    className="nav-link"
                    style={{ cursor: "pointer" }}
                    onSelect={() => i18n.changeLanguage("en")}
                  >
                    English
                  </DropdownMenu.Item>
                  <DropdownMenu.Item
                    className="nav-link"
                    style={{ cursor: "pointer" }}
                    onSelect={() => i18n.changeLanguage("ja")}
                  >
                    日本語
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>

            <DropdownMenu.Root>
              <DropdownMenu.Trigger asChild>
                <Button variant="ghost" size="icon" aria-label={t("app.theme")}>
                  {theme === "dark" ? <Moon size={18} /> : theme === "light" ? <Sun size={18} /> : <Monitor size={18} />}
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content
                  className="card"
                  style={{ padding: "0.5rem", minWidth: 140 }}
                  sideOffset={4}
                >
                  {(["light", "dark", "system"] as const).map((t_) => (
                    <DropdownMenu.Item
                      key={t_}
                      className="nav-link"
                      style={{ cursor: "pointer" }}
                      onSelect={() => setTheme(t_)}
                    >
                      {t(`app.theme${t_.charAt(0).toUpperCase()}${t_.slice(1)}` as "app.themeLight")}
                    </DropdownMenu.Item>
                  ))}
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>

            <Button variant="ghost" size="sm" onClick={() => logout()}>
              <LogOut size={16} />
              {t("app.logout")}
            </Button>
          </div>
        </header>
        <main className="page-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
