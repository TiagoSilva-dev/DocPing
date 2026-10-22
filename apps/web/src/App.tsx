import {
  Component,
  Suspense,
  lazy,
  type ErrorInfo,
  type ReactNode,
} from "react";
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  Outlet,
  useLocation,
  Link,
} from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { MotionConfig } from "framer-motion";
import { api, ApiError, getSession, post } from "@/lib/api";
import { SessionContext } from "@/lib/session";
import { ToastProvider } from "@/lib/toast";
import { Shell } from "@/components/Shell";
import { usePageMeta } from "@/lib/seo";
import { Button, Logo, Loading, ErrorState } from "@/components/shared";
const Dashboard = lazy(() =>
  import("@/pages/Dashboard").then((m) => ({ default: m.Dashboard })),
);
const RequestsPage = lazy(() =>
  import("@/pages/Dashboard").then((m) => ({ default: m.RequestsPage })),
);
const CreateRequest = lazy(() =>
  import("@/pages/CreateRequest").then((m) => ({ default: m.CreateRequest })),
);
const RequestDetail = lazy(() =>
  import("@/pages/RequestDetail").then((m) => ({ default: m.RequestDetail })),
);
const PortalPage = lazy(() =>
  import("@/pages/Portal").then((m) => ({ default: m.PortalPage })),
);
const ClientsPage = lazy(() =>
  import("@/pages/WorkspacePages").then((m) => ({ default: m.ClientsPage })),
);
const TemplatesPage = lazy(() =>
  import("@/pages/WorkspacePages").then((m) => ({ default: m.TemplatesPage })),
);
const SchedulesPage = lazy(() =>
  import("@/pages/WorkspacePages").then((m) => ({ default: m.SchedulesPage })),
);
const SettingsPage = lazy(() =>
  import("@/pages/WorkspacePages").then((m) => ({ default: m.SettingsPage })),
);
const AuthPage = lazy(() =>
  import("@/pages/AuthPages").then((m) => ({ default: m.AuthPage })),
);
const VerifyEmailPage = lazy(() =>
  import("@/pages/AuthPages").then((m) => ({ default: m.VerifyEmailPage })),
);
const ForgotPasswordPage = lazy(() =>
  import("@/pages/AuthPages").then((m) => ({ default: m.ForgotPasswordPage })),
);
const ResetPasswordPage = lazy(() =>
  import("@/pages/AuthPages").then((m) => ({ default: m.ResetPasswordPage })),
);
const Landing = lazy(() =>
  import("@/pages/Landing").then((m) => ({ default: m.Landing })),
);
function ProtectedLayout() {
  const queryClient = useQueryClient();
  const location = useLocation();
  const {
    data: session,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ["session"],
    queryFn: getSession,
    retry: false,
    staleTime: 5 * 60000,
  });
  if (isLoading) return <Loading />;
  if (error instanceof ApiError && error.status === 401)
    return (
      <Navigate
        to={`/entrar?next=${encodeURIComponent(location.pathname)}`}
        replace
      />
    );
  if (error || !session)
    return (
      <ErrorState
        message={error?.message || "Não foi possível acessar a sessão."}
        onRetry={() => void refetch()}
      />
    );
  return (
    <SessionContext.Provider
      value={{
        session,
        onLogout: async () => {
          await post("/auth/logout");
          queryClient.clear();
          window.location.assign("/entrar");
        },
      }}
    >
      <Shell />
    </SessionContext.Provider>
  );
}
function NotFound() {
  usePageMeta({ title: "Página não encontrada — DocPing" });
  return (
    <div className="not-found-page">
      <Logo />
      <span className="mono">404</span>
      <h1>Este caminho não tem documentos.</h1>
      <p>Vamos voltar para um lugar conhecido?</p>
      <Link to="/app">
        <Button>Voltar ao início</Button>
      </Link>
    </div>
  );
}
export default function App() {
  return (
    <ErrorBoundary>
      <MotionConfig reducedMotion="user">
        <ToastProvider>
          <BrowserRouter>
            <Suspense fallback={<Loading />}>
              <Routes>
                <Route path="/" element={<Landing />} />
                <Route path="/entrar" element={<AuthPage />} />
                <Route path="/cadastro" element={<AuthPage register />} />
                <Route path="/esqueci-senha" element={<ForgotPasswordPage />} />
                <Route
                  path="/redefinir-senha"
                  element={<ResetPasswordPage />}
                />
                <Route path="/confirmar-email" element={<VerifyEmailPage />} />
                <Route path="/p/:token" element={<PortalPage />} />
                <Route path="/portal/:token" element={<PortalPage />} />
                <Route path="/app" element={<ProtectedLayout />}>
                  <Route index element={<Dashboard />} />
                  <Route path="solicitacoes" element={<RequestsPage />} />
                  <Route path="solicitacoes/nova" element={<CreateRequest />} />
                  <Route path="solicitacoes/:id" element={<RequestDetail />} />
                  <Route path="clientes" element={<ClientsPage />} />
                  <Route path="modelos" element={<TemplatesPage />} />
                  <Route path="recorrencias" element={<SchedulesPage />} />
                  <Route path="configuracoes" element={<SettingsPage />} />
                </Route>
                <Route path="*" element={<NotFound />} />
              </Routes>
            </Suspense>
          </BrowserRouter>
        </ToastProvider>
      </MotionConfig>
    </ErrorBoundary>
  );
}
class ErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  componentDidCatch(_error: Error, _info: ErrorInfo) {}
  render() {
    return this.state.failed ? (
      <div className="not-found-page">
        <Logo />
        <h1>Precisamos recarregar esta página.</h1>
        <p>Seus dados já salvos continuam aqui.</p>
        <Button onClick={() => window.location.reload()}>
          Recarregar página
        </Button>
      </div>
    ) : (
      this.props.children
    );
  }
}
