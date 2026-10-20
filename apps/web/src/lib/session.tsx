import { createContext, useContext } from "react";
import type { Session } from "./types";
export const SessionContext = createContext<{
  session: Session;
  onLogout: () => Promise<void>;
} | null>(null);
export function useSession() {
  const context = useContext(SessionContext);
  if (!context) throw new Error("Sessão indisponível.");
  return context;
}
