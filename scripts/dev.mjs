import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../", import.meta.url));

// The README and .env.example tell people to put local values in .env, but the
// Go process only ever saw the shell environment. Read the file here so a key
// configured once is picked up by `npm run dev` as well. A variable already
// exported in the shell still wins, and the file is optional.
function readEnvFile() {
  const out = {};
  let raw;
  try {
    raw = readFileSync(`${root}.env`, "utf8");
  } catch {
    return out;
  }
  for (const line of raw.split("\n")) {
    const match = /^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/.exec(line);
    if (!match || line.trimStart().startsWith("#")) continue;
    const value = match[2].trim().replace(/^(['"])(.*)\1$/, "$2");
    if (!(match[1] in process.env)) out[match[1]] = value;
  }
  return out;
}
const fileEnv = readEnvFile();
const api = spawn("go", ["run", "./cmd/server"], {
  cwd: `${root}apps/api`,
  env: {
    ...fileEnv,
    ...process.env,
    // Development forces the safe local defaults regardless of .env: JSON
    // store, synthetic uploads and the Vite origin.
    APP_ENV: process.env.APP_ENV || "development",
    STORE_DRIVER: process.env.STORE_DRIVER || "json",
    ALLOW_UNSCANNED_UPLOADS: process.env.ALLOW_UNSCANNED_UPLOADS || "true",
    WEB_ORIGIN: process.env.WEB_ORIGIN || "http://localhost:5173",
    // The extraction services publish on loopback for this mode; inside the
    // Compose network they answer on docling:5001 and paddleocr:8080.
    DOCLING_URL:
      process.env.DOCLING_URL || fileEnv.DOCLING_URL || "http://127.0.0.1:5001",
    PADDLEOCR_URL:
      process.env.PADDLEOCR_URL ||
      fileEnv.PADDLEOCR_URL ||
      "http://127.0.0.1:8081",
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
