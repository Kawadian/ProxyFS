import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";

export function SetupPage() {
  const { t, i18n } = useTranslation();
  const { setup } = useAuth();
  const { toast } = useToast();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (password !== confirmPassword) {
      setError(t("setup.passwordMismatch"));
      return;
    }
    setLoading(true);
    try {
      await setup({ username, password, display_name: displayName || undefined });
      toast(t("app.success"), undefined, "success");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("errors.generic"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1>{t("setup.title")}</h1>
        <p className="auth-subtitle">{t("setup.subtitle")}</p>
        <form onSubmit={handleSubmit}>
          <FormGroup label={t("setup.language")}>
            <Select
              value={i18n.language.startsWith("ja") ? "ja" : "en"}
              onChange={(e) => i18n.changeLanguage(e.target.value)}
            >
              <option value="en">English</option>
              <option value="ja">日本語</option>
            </Select>
          </FormGroup>
          <FormGroup label={t("setup.displayName")}>
            <Input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="LXC File Hub"
            />
          </FormGroup>
          <FormGroup label={t("setup.username")} htmlFor="setup-username">
            <Input
              id="setup-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="username"
            />
          </FormGroup>
          <FormGroup label={t("setup.password")} htmlFor="setup-password">
            <Input
              id="setup-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
            />
          </FormGroup>
          <FormGroup label={t("setup.confirmPassword")} htmlFor="setup-confirm">
            <Input
              id="setup-confirm"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
            />
          </FormGroup>
          {error && <p className="form-error" role="alert">{error}</p>}
          <Button type="submit" className="w-full mt-4" disabled={loading}>
            {loading ? t("app.loading") : t("setup.submit")}
          </Button>
        </form>
      </div>
    </div>
  );
}
