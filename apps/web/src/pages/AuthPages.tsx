import { useState } from "react";
import type { ReactNode } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import type { UseFormRegisterReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CheckCircle2,
  Eye,
  EyeOff,
  FileText,
  Mail,
} from "lucide-react";
import { Button, Field, Logo } from "@/components/shared";
import { Form, FormMessage } from "@/components/ui/form";
import { ApiError, post } from "@/lib/api";
import type { Session } from "@/lib/types";
import "./auth-pages.css";

const errorMessage = (error: unknown) =>
  error instanceof Error
    ? error.message
    : "Não foi possível continuar. Tente novamente.";
const emailSchema = z
  .string()
  .trim()
  .min(1, "Informe seu e-mail.")
  .email("Informe um e-mail válido.")
  .max(254, "Use um e-mail com até 254 caracteres.");
const passwordSchema = z
  .string()
  .min(12, "Use pelo menos 12 caracteres.")
  .max(128, "Use até 128 caracteres.")
  .refine(
    (value) => new TextEncoder().encode(value).length <= 128,
    "Essa senha é muito longa. Use uma senha menor.",
  );
const authSchema = (register: boolean) =>
  z
    .object({
      organizationName: z
        .string()
        .trim()
        .max(180, "Use até 180 caracteres.")
        .optional(),
      name: z.string().trim().max(120, "Use até 120 caracteres.").optional(),
      email: emailSchema,
      password: register
        ? passwordSchema
        : z
            .string()
            .min(1, "Informe sua senha.")
            .max(128, "Use até 128 caracteres."),
    })
    .superRefine((values, context) => {
      if (register && !values.organizationName)
        context.addIssue({
          code: "custom",
          path: ["organizationName"],
          message: "Informe o nome do escritório.",
        });
      if (register && !values.name)
        context.addIssue({
          code: "custom",
          path: ["name"],
          message: "Informe seu nome.",
        });
    });
type AuthValues = z.infer<ReturnType<typeof authSchema>>;

function AuthLayout({
  children,
  register = false,
}: {
  children: ReactNode;
  register?: boolean;
}) {
  return (
    <main className="auth-layout">
      <aside className="auth-editorial">
        <Link
          to="/"
          className="auth-brand-link"
          aria-label="DocPing — página inicial"
        >
          <Logo />
        </Link>
        <div className="auth-editorial-body">
          <span className="auth-eyebrow">MENOS COBRANÇA. MAIS CLAREZA.</span>
          <h1>
            Documentos no lugar.
            <br />
            <span>Cabeça tranquila.</span>
          </h1>
          <p>
            Peça uma vez. Acompanhe tudo.
            <br />
            Deixe o fechamento respirar.
          </p>
          <div className="auth-paper-art" aria-hidden="true">
            <div className="auth-paper auth-paper-back" />
            <div className="auth-paper auth-paper-middle" />
            <div className="auth-paper auth-paper-front">
              <FileText size={32} strokeWidth={1.2} />
              <span />
              <span />
              <span />
              <i>
                <Check size={21} strokeWidth={2} />
              </i>
            </div>
            <div className="auth-art-caption">
              <span />
              Uma rotina mais leve começa aqui.
            </div>
          </div>
        </div>
        <p className="auth-editorial-footer">
          Feito para a rotina dos escritórios contábeis.
        </p>
      </aside>
      <section className="auth-main">
        <div className="auth-topline">
          <Link
            to="/"
            className="auth-mobile-logo"
            aria-label="DocPing — início"
          >
            <Logo small />
          </Link>
          <p>
            {register ? "Já tem um espaço?" : "Ainda não tem uma conta?"}{" "}
            <Link to={register ? "/entrar" : "/cadastro"}>
              {register ? "Entrar" : "Criar escritório"}
              <ArrowRight size={13} />
            </Link>
          </p>
        </div>
        <div className="auth-content">{children}</div>
        <p className="auth-bottomline">
          Um documento de cada vez. Um fechamento mais tranquilo.
        </p>
      </section>
    </main>
  );
}

function PasswordInput({
  registerField,
  error,
  autoComplete = "current-password",
  id = "password",
}: {
  registerField: UseFormRegisterReturn;
  error?: string;
  autoComplete?: string;
  id?: string;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="auth-password-input">
      <input
        id={id}
        className="input"
        type={visible ? "text" : "password"}
        autoComplete={autoComplete}
        maxLength={128}
        aria-invalid={!!error}
        aria-describedby={
          error
            ? `${id}-error`
            : autoComplete === "new-password"
              ? "password-hint"
              : undefined
        }
        {...registerField}
      />
      <button
        type="button"
        aria-label={visible ? "Ocultar senha" : "Mostrar senha"}
        aria-pressed={visible}
        onClick={() => setVisible((value) => !value)}
      >
        {visible ? <EyeOff size={17} /> : <Eye size={17} />}
      </button>
    </div>
  );
}

export function AuthPage({ register = false }: { register?: boolean }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [demoPending, setDemoPending] = useState(false);
  const {
    register: registerField,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<AuthValues>({
    resolver: zodResolver(authSchema(register)),
    defaultValues: { organizationName: "", name: "", email: "", password: "" },
  });
  function enter(session: Session, target: string) {
    queryClient.clear();
    queryClient.setQueryData(["session"], session);
    navigate(target, { replace: true });
  }
  async function submit(values: AuthValues) {
    try {
      const session = await post<Session>(
        register ? "/auth/register" : "/auth/login",
        register ? values : { email: values.email, password: values.password },
      );
      enter(session, register ? "/app/solicitacoes/nova" : "/app");
    } catch (error) {
      setError("root", { message: errorMessage(error) });
    }
  }
  async function demo() {
    setDemoPending(true);
    try {
      enter(await post<Session>("/auth/demo"), "/app");
    } catch (error) {
      setError("root", { message: errorMessage(error) });
    } finally {
      setDemoPending(false);
    }
  }
  return (
    <AuthLayout register={register}>
      <div className="auth-form-heading">
        <span className="auth-eyebrow">
          {register ? "SEU PRIMEIRO PASSO" : "SEU ESPAÇO DE TRABALHO"}
        </span>
        <h2>
          {register
            ? "Um fechamento mais leve começa aqui."
            : "Bom ter você de volta."}
        </h2>
        <p>
          {register
            ? "Crie o espaço do seu escritório. Em seguida, vamos preparar sua primeira solicitação."
            : "Entre para ver o que chegou e o que ainda falta."}
        </p>
      </div>
      <Form className="auth-form" onSubmit={handleSubmit(submit)}>
        {register && (
          <>
            <div>
              <Field label="Nome do escritório">
                <input
                  className="input"
                  autoFocus
                  autoComplete="organization"
                  maxLength={180}
                  placeholder="Ex.: Aurora Contabilidade"
                  aria-invalid={!!errors.organizationName}
                  aria-describedby={
                    errors.organizationName ? "organization-error" : undefined
                  }
                  {...registerField("organizationName")}
                />
              </Field>
              <FormMessage
                id="organization-error"
                className="auth-field-error"
                error={errors.organizationName?.message}
              />
            </div>
            <div>
              <Field label="Seu nome">
                <input
                  className="input"
                  autoComplete="name"
                  maxLength={120}
                  placeholder="Como podemos chamar você?"
                  aria-invalid={!!errors.name}
                  aria-describedby={errors.name ? "name-error" : undefined}
                  {...registerField("name")}
                />
              </Field>
              <FormMessage
                id="name-error"
                className="auth-field-error"
                error={errors.name?.message}
              />
            </div>
          </>
        )}
        <div>
          <Field label="E-mail de trabalho">
            <input
              className="input"
              type="email"
              autoFocus={!register}
              autoComplete="email"
              inputMode="email"
              maxLength={254}
              placeholder="voce@escritorio.com.br"
              aria-invalid={!!errors.email}
              aria-describedby={errors.email ? "email-error" : undefined}
              {...registerField("email")}
            />
          </Field>
          <FormMessage
            id="email-error"
            className="auth-field-error"
            error={errors.email?.message}
          />
        </div>
        <div>
          <div className="auth-password-label">
            <label htmlFor="password" className="field-label">
              {register ? "Crie uma senha" : "Senha"}
            </label>
            {!register && <Link to="/esqueci-senha">Esqueci minha senha</Link>}
          </div>
          <PasswordInput
            registerField={registerField("password")}
            autoComplete={register ? "new-password" : "current-password"}
            error={errors.password?.message}
          />
          {register && (
            <p className="auth-password-hint" id="password-hint">
              Pelo menos 12 caracteres. Uma frase curta também funciona.
            </p>
          )}
          <FormMessage
            id="password-error"
            className="auth-field-error"
            error={errors.password?.message}
          />
        </div>
        {errors.root?.message && (
          <p className="inline-error" role="alert">
            {errors.root.message}
          </p>
        )}
        <Button
          className="auth-submit"
          type="submit"
          loading={isSubmitting}
          disabled={demoPending}
        >
          {register ? "Criar meu escritório" : "Entrar no meu espaço"}
          <ArrowRight size={16} />
        </Button>
      </Form>
      <div className="auth-demo-divider">
        <span />
        ou conheça primeiro
        <span />
      </div>
      <Button
        className="auth-demo-button"
        variant="secondary"
        disabled={isSubmitting}
        loading={demoPending}
        onClick={() => void demo()}
      >
        Explorar demonstração
        <ArrowRight size={15} />
      </Button>
      <p className="auth-demo-note">
        Um escritório de exemplo, com clientes e arquivos fictícios. Você pode
        explorar antes de cadastrar seus dados.
      </p>
      {register && (
        <p className="auth-register-note">
          Sem dados de cobrança para começar. Você prepara a primeira
          solicitação e conhece o fluxo no seu ritmo.
        </p>
      )}
    </AuthLayout>
  );
}

const forgotSchema = z.object({ email: emailSchema });
type ForgotValues = z.infer<typeof forgotSchema>;
export function ForgotPasswordPage() {
  const [complete, setComplete] = useState(false);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ForgotValues>({
    resolver: zodResolver(forgotSchema),
    defaultValues: { email: "" },
  });
  async function submit(values: ForgotValues) {
    try {
      await post("/auth/forgot-password", values);
      setComplete(true);
    } catch (error) {
      setError("root", { message: errorMessage(error) });
    }
  }
  return (
    <AuthLayout>
      <Link to="/entrar" className="auth-back-link">
        <ArrowLeft size={15} />
        Voltar para entrar
      </Link>
      {complete ? (
        <div className="auth-confirmation" role="status">
          <span className="auth-confirmation-icon">
            <Mail size={26} strokeWidth={1.5} />
          </span>
          <h2>Confira seu e-mail.</h2>
          <p>
            Se o e-mail informado estiver cadastrado, enviaremos as instruções
            para redefinir sua senha. O link é válido por 30 minutos.
          </p>
          <p className="auth-muted-copy">
            Confira também a pasta de spam. Se precisar, tente com outro e-mail.
          </p>
          <Button variant="secondary" onClick={() => setComplete(false)}>
            Usar outro e-mail
            <ArrowRight size={15} />
          </Button>
        </div>
      ) : (
        <>
          <div className="auth-form-heading">
            <span className="auth-eyebrow">VAMOS RESOLVER</span>
            <h2>Esqueceu sua senha?</h2>
            <p>
              Acontece. Informe seu e-mail e solicite um link para criar uma
              nova senha.
            </p>
          </div>
          <Form className="auth-form" onSubmit={handleSubmit(submit)}>
            <div>
              <Field label="E-mail de trabalho">
                <input
                  className="input"
                  type="email"
                  autoFocus
                  autoComplete="email"
                  maxLength={254}
                  placeholder="voce@escritorio.com.br"
                  aria-invalid={!!errors.email}
                  aria-describedby={
                    errors.email ? "forgot-email-error" : undefined
                  }
                  {...register("email")}
                />
              </Field>
              <FormMessage
                id="forgot-email-error"
                className="auth-field-error"
                error={errors.email?.message}
              />
            </div>
            {errors.root?.message && (
              <p className="inline-error" role="alert">
                {errors.root.message}
              </p>
            )}
            <Button
              className="auth-submit"
              type="submit"
              loading={isSubmitting}
            >
              Solicitar link de recuperação
              <ArrowRight size={16} />
            </Button>
          </Form>
        </>
      )}
    </AuthLayout>
  );
}

const resetSchema = z
  .object({
    password: passwordSchema,
    confirmPassword: z.string().min(1, "Confirme sua nova senha."),
  })
  .refine((values) => values.password === values.confirmPassword, {
    message: "As senhas precisam ser iguais.",
    path: ["confirmPassword"],
  });
type ResetValues = z.infer<typeof resetSchema>;
export function ResetPasswordPage() {
  const [params] = useSearchParams();
  const token = params.get("token") ?? "";
  const queryClient = useQueryClient();
  const [complete, setComplete] = useState(false);
  const [expired, setExpired] = useState(!token);
  const [visible, setVisible] = useState(false);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ResetValues>({
    resolver: zodResolver(resetSchema),
    defaultValues: { password: "", confirmPassword: "" },
  });
  async function submit(values: ResetValues) {
    try {
      await post("/auth/reset-password", { token, password: values.password });
      queryClient.clear();
      setComplete(true);
    } catch (error) {
      if (
        error instanceof ApiError &&
        (error.status === 410 || error.code === "reset_expired")
      )
        setExpired(true);
      else setError("root", { message: errorMessage(error) });
    }
  }
  return (
    <AuthLayout>
      <Link to="/entrar" className="auth-back-link">
        <ArrowLeft size={15} />
        Voltar para entrar
      </Link>
      {complete ? (
        <div className="auth-confirmation" role="status">
          <span className="auth-confirmation-icon">
            <CheckCircle2 size={28} strokeWidth={1.5} />
          </span>
          <h2>Senha atualizada.</h2>
          <p>
            Está tudo pronto. Entre com sua nova senha para continuar de onde
            parou.
          </p>
          <Link className="auth-primary-link" to="/entrar">
            Entrar no meu espaço
            <ArrowRight size={16} />
          </Link>
        </div>
      ) : expired ? (
        <div className="auth-confirmation">
          <span className="auth-confirmation-icon">
            <Mail size={27} strokeWidth={1.5} />
          </span>
          <h2>Precisamos de um novo link.</h2>
          <p>
            Este link está incompleto, expirou ou já foi utilizado. Solicite
            outro para redefinir sua senha.
          </p>
          <Link className="auth-primary-link" to="/esqueci-senha">
            Solicitar novo link
            <ArrowRight size={16} />
          </Link>
        </div>
      ) : (
        <>
          <div className="auth-form-heading">
            <span className="auth-eyebrow">UM NOVO COMEÇO</span>
            <h2>Crie sua nova senha.</h2>
            <p>Escolha uma senha que só você conhece.</p>
          </div>
          <Form className="auth-form" onSubmit={handleSubmit(submit)}>
            <div>
              <label htmlFor="new-password" className="field-label">
                Nova senha
              </label>
              <div className="auth-password-input">
                <input
                  id="new-password"
                  className="input"
                  autoFocus
                  type={visible ? "text" : "password"}
                  autoComplete="new-password"
                  maxLength={128}
                  aria-invalid={!!errors.password}
                  aria-describedby="password-hint new-password-error"
                  {...register("password")}
                />
                <button
                  type="button"
                  aria-label={visible ? "Ocultar senha" : "Mostrar senha"}
                  aria-pressed={visible}
                  onClick={() => setVisible((value) => !value)}
                >
                  {visible ? <EyeOff size={17} /> : <Eye size={17} />}
                </button>
              </div>
              <p className="auth-password-hint" id="password-hint">
                Pelo menos 12 caracteres. Use até 128 caracteres.
              </p>
              <FormMessage
                id="new-password-error"
                className="auth-field-error"
                error={errors.password?.message}
              />
            </div>
            <div>
              <Field label="Confirme a nova senha">
                <input
                  className="input"
                  type={visible ? "text" : "password"}
                  autoComplete="new-password"
                  maxLength={128}
                  aria-invalid={!!errors.confirmPassword}
                  aria-describedby={
                    errors.confirmPassword
                      ? "confirm-password-error"
                      : undefined
                  }
                  {...register("confirmPassword")}
                />
              </Field>
              <FormMessage
                id="confirm-password-error"
                className="auth-field-error"
                error={errors.confirmPassword?.message}
              />
            </div>
            {errors.root?.message && (
              <p className="inline-error" role="alert">
                {errors.root.message}
              </p>
            )}
            <Button
              className="auth-submit"
              type="submit"
              loading={isSubmitting}
            >
              Salvar nova senha
              <ArrowRight size={16} />
            </Button>
          </Form>
        </>
      )}
    </AuthLayout>
  );
}
