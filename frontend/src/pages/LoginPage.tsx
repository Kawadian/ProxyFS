import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/Button";
import { FormGroup, Input, Select } from "@/components/ui/Input";
import { ApiError } from "@/api/client";

export function LoginPage() {
  const { t, i18n } = useTranslation();
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 401
          ? t("login.failed")
          : err instanceof Error
            ? err.message
            : t("errors.generic"),
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1>{t("login.title")}</h1>
        <p className="auth-subtitle">{t("login.subtitle")}</p>
        <form onSubmit={handleSubmit}>
          <FormGroup label={t("app.language")}>
            <Select
              value={i18n.language.startsWith("ja") ? "ja" : "en"}
              onChange={(e) => i18n.changeLanguage(e.target.value)}
            >
              <option value="en">English</option>
              <option value="ja">日本語</option>
            </Select>
          </FormGroup>
          <FormGroup label={t("login.username")} htmlFor="login-username">
            <Input
              id="login-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="username"
            />
          </FormGroup>
          <FormGroup label={t("login.password")} htmlFor="login-password">
            <Input
              id="login-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="current-password"
            />
          </FormGroup>
          {error && <p className="form-error" role="alert">{error}</p>}
          <Button type="submit" className="w-full mt-4" disabled={loading}>
            {loading ? t("app.loading") : t("login.submit")}
          </Button>
        </form>
      </div>
    </div>
  );
}
