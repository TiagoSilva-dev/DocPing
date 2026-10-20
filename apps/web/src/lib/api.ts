import type { Session } from "./types";
let csrfToken = "";
export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...options,
    credentials: "same-origin",
    headers: {
      ...(options.body instanceof FormData
        ? {}
        : { "Content-Type": "application/json" }),
      ...(csrfToken ? { "X-CSRF-Token": csrfToken } : {}),
      ...options.headers,
    },
  });
  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new ApiError(
      data?.error?.code || "network_error",
      data?.error?.message || "Não foi possível concluir. Tente novamente.",
      response.status,
    );
  }
  if (response.status === 204) return undefined as T;
  const result = await response.json();
  if (result?.csrfToken) csrfToken = result.csrfToken;
  return result as T;
}
export function post<T>(path: string, body?: unknown) {
  return api<T>(path, {
    method: "POST",
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}
export function put<T>(path: string, body: unknown) {
  return api<T>(path, { method: "PUT", body: JSON.stringify(body) });
}
export function getSession() {
  return api<Session>("/session");
}
export function uploadFile(
  token: string,
  itemId: string,
  file: File,
  onProgress: (percent: number) => void,
): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const data = new FormData();
    data.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open(
      "POST",
      `/api/portal/${encodeURIComponent(token)}/items/${encodeURIComponent(itemId)}/upload`,
    );
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable)
        onProgress(Math.min(99, Math.round((e.loaded / e.total) * 100)));
    };
    xhr.onerror = () =>
      reject(
        new Error(
          "A conexão falhou. Seu progresso anterior está salvo. Tente novamente.",
        ),
      );
    xhr.onload = () => {
      let result;
      try {
        result = JSON.parse(xhr.responseText);
      } catch {
        result = null;
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress(100);
        resolve(result);
      } else
        reject(
          new Error(
            result?.error?.message ||
              "Não foi possível enviar este arquivo. Tente novamente.",
          ),
        );
    };
    xhr.send(data);
  });
}
