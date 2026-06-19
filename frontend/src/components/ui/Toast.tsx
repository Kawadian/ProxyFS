import * as Toast from "@radix-ui/react-toast";
import { createContext, useCallback, useContext, useState, type ReactNode } from "react";
import { X } from "lucide-react";

type ToastType = "success" | "error" | "info";

interface ToastMessage {
  id: string;
  title: string;
  description?: string;
  type: ToastType;
}

interface ToastContextValue {
  toast: (title: string, description?: string, type?: ToastType) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  const toast = useCallback((title: string, description?: string, type: ToastType = "info") => {
    const id = crypto.randomUUID();
    setToasts((prev) => [...prev, { id, title, description, type }]);
  }, []);

  const remove = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  };

  return (
    <ToastContext.Provider value={{ toast }}>
      <Toast.Provider swipeDirection="right" duration={4000}>
        {children}
        {toasts.map((t) => (
          <Toast.Root
            key={t.id}
            className="toast"
            onOpenChange={(open) => !open && remove(t.id)}
            open
          >
            <div style={{ flex: 1 }}>
              <Toast.Title style={{ fontWeight: 600, fontSize: "0.9rem" }}>{t.title}</Toast.Title>
              {t.description && (
                <Toast.Description style={{ fontSize: "0.8rem", color: "var(--text-muted)", marginTop: 2 }}>
                  {t.description}
                </Toast.Description>
              )}
            </div>
            <Toast.Close asChild>
              <button className="btn btn-ghost btn-icon" aria-label="Close">
                <X size={16} />
              </button>
            </Toast.Close>
          </Toast.Root>
        ))}
        <Toast.Viewport className="toast-viewport" />
      </Toast.Provider>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}
