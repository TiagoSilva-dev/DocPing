import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../", import.meta.url));
const api = spawn("go", ["run", "./cmd/server"], {
  cwd: `${root}apps/api`,
  env: {
    ...process.env,
    APP_ENV: process.env.APP_ENV || "development",
    STORE_DRIVER: process.env.STORE_DRIVER || "json",
    ALLOW_UNSCANNED_UPLOADS: process.env.ALLOW_UNSCANNED_UPLOADS || "true",
    WEB_ORIGIN: process.env.WEB_ORIGIN || "http://localhost:5173",
  },
  stdio: "inherit",
  detached: process.platform !== "win32",
});
const web = spawn("npm", ["run", "dev:web"], {
  cwd: root,
  stdio: "inherit",
  detached: process.platform !== "win32",
  env: {
    ...process.env,
    API_PROXY_TARGET:
      process.env.API_PROXY_TARGET ||
      `http://127.0.0.1:${process.env.PORT || "8080"}`,
  },
});
let stopping = false;
function stop(signal = "SIGTERM") {
  stopping = true;
  for (const child of [api, web]) {
    try {
      if (process.platform !== "win32" && child.pid)
        process.kill(-child.pid, signal);
      else child.kill(signal);
    } catch (error) {
      if (error.code !== "ESRCH")
        console.error(
          "Não foi possível encerrar um processo de desenvolvimento.",
        );
    }
  }
}
process.on("SIGINT", () => stop("SIGINT"));
process.on("SIGTERM", () => stop());
api.on("exit", (code) => {
  if (code && !stopping) {
    console.error("API encerrou com erro.");
    stop();
    process.exitCode = code;
  }
});
web.on("exit", (code) => {
  if (code && !stopping) {
    stop();
    process.exitCode = code;
  }
});
