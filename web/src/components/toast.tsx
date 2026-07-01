import React, { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";

type ToastTone = "success" | "danger" | "info" | "warn";
type Toast = { id: number; tone: ToastTone; message: string };

type ToastAPI = {
  notify: (message: string, tone?: ToastTone) => void;
  success: (message: string) => void;
  error: (message: string) => void;
};

const ToastContext = createContext<ToastAPI | null>(null);

export function useToast(): ToastAPI {
  const api = useContext(ToastContext);
  if (!api) throw new Error("useToast must be used inside ToastProvider");
  return api;
}

export function ToastProvider(props: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextID = useRef(1);

  const notify = useCallback((message: string, tone: ToastTone = "info") => {
    const id = nextID.current++;
    setToasts((current) => [...current.slice(-3), { id, tone, message }]);
    window.setTimeout(() => {
      setToasts((current) => current.filter((toast) => toast.id !== id));
    }, tone === "danger" ? 8000 : 4500);
  }, []);

  const api = useMemo<ToastAPI>(
    () => ({
      notify,
      success: (message) => notify(message, "success"),
      error: (message) => notify(message, "danger")
    }),
    [notify]
  );

  return (
    <ToastContext.Provider value={api}>
      {props.children}
      <div className="toast-stack" aria-live="polite">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast tone-${toast.tone}`}>
            <span>{toast.message}</span>
            <button
              type="button"
              aria-label="Dismiss"
              onClick={() => setToasts((current) => current.filter((item) => item.id !== toast.id))}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
