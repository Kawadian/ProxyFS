import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { protocolsApi } from "@/api/endpoints";
import { LoadingSpinner, StatusBadge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import type { ProtocolStatus } from "@/api/types";

function ProtocolCard({
  protocol,
  onToggle,
  loading,
}: {
  protocol: ProtocolStatus;
  onToggle: (enabled: boolean) => void;
  loading: boolean;
}) {
  const { t } = useTranslation();
  const labelKey = `protocols.names.${protocol.name}` as const;

  return (
    <div className="card">
      <div className="card-header flex justify-between items-center">
        <h2 className="card-title">{t(labelKey)}</h2>
        <StatusBadge status={protocol.running ? "ok" : protocol.enabled ? "pending" : "disabled"} />
      </div>
      <div className="card-body">
        <dl className="text-sm" style={{ display: "grid", gridTemplateColumns: "120px 1fr", gap: "0.5rem", marginBottom: "1rem" }}>
          <dt className="text-muted">{t("protocols.enabled")}</dt>
          <dd>{protocol.enabled ? t("app.yes") : t("app.no")}</dd>
          <dt className="text-muted">{t("protocols.running")}</dt>
          <dd>{protocol.running ? t("app.yes") : t("app.no")}</dd>
          {protocol.port ? (
            <>
              <dt className="text-muted">{t("nodes.port")}</dt>
              <dd>{protocol.port}</dd>
            </>
          ) : null}
          {protocol.path ? (
            <>
              <dt className="text-muted">{t("protocols.path")}</dt>
              <dd className="font-mono text-sm">{protocol.path}</dd>
            </>
          ) : null}
        </dl>
        {protocol.message ? (
          <p className="text-sm" style={{ color: "var(--color-warning)", marginBottom: "1rem" }}>
            {protocol.message}
          </p>
        ) : null}
        <Button
          variant={protocol.enabled ? "secondary" : "primary"}
          onClick={() => onToggle(!protocol.enabled)}
          disabled={loading}
        >
          {protocol.enabled ? t("protocols.stop") : t("protocols.start")}
        </Button>
      </div>
    </div>
  );
}

export function ProtocolsPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["protocols"],
    queryFn: protocolsApi.get,
    refetchInterval: 10_000,
  });

  const toggleMutation = useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) =>
      protocolsApi.setEnabled(name, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["protocols"] });
      toast(t("app.success"), undefined, "success");
    },
    onError: (e: Error) => toast(t("app.error"), e.message, "error"),
  });

  if (isLoading) return <LoadingSpinner />;

  return (
    <div>
      <h1 className="page-title">{t("protocols.title")}</h1>
      <p className="text-sm text-muted mb-4">{t("protocols.description")}</p>

      <div className="form-row" style={{ alignItems: "stretch" }}>
        {(data?.protocols ?? []).map((p) => (
          <ProtocolCard
            key={p.name}
            protocol={p}
            loading={toggleMutation.isPending}
            onToggle={(enabled) => toggleMutation.mutate({ name: p.name, enabled })}
          />
        ))}
      </div>
    </div>
  );
}
