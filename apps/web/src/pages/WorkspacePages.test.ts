import { afterEach, describe, expect, it, vi } from "vitest";
import { parseClientCSV } from "./WorkspacePages";
import { dateLabel, relativeDate } from "@/lib/format";

describe("client CSV preview", () => {
  it("reads UTF-8 BOM, semicolon fields and Windows line endings", () => {
    expect(
      parseClientCSV("\uFEFFnome;email\r\nHorizonte;ana@exemplo.com.br\r\n"),
    ).toEqual([
      ["nome", "email"],
      ["Horizonte", "ana@exemplo.com.br"],
    ]);
  });

  it("preserves quoted separators, escaped quotes and multiline company names", () => {
    expect(
      parseClientCSV(
        'nome,email\n"Horizonte, ""Sul""\nServiços",ana@exemplo.com.br',
      ),
    ).toEqual([
      ["nome", "email"],
      ['Horizonte, "Sul"\nServiços', "ana@exemplo.com.br"],
    ]);
  });

  it("keeps empty optional columns aligned and ignores blank records", () => {
    expect(
      parseClientCSV("nome;cnpj;email;telefone\n\nHorizonte;;;11999999999\n"),
    ).toEqual([
      ["nome", "cnpj", "email", "telefone"],
      ["Horizonte", "", "", "11999999999"],
    ]);
  });

  it("rejects unclosed quotation marks instead of previewing corrupted data", () => {
    expect(() =>
      parseClientCSV('nome,email\n"Horizonte,ana@exemplo.com.br'),
    ).toThrow("aspas não fechadas");
  });

  it("rejects text outside a quoted field", () => {
    expect(() =>
      parseClientCSV('nome,email\n"Horizonte"fora,ana@exemplo.com.br'),
    ).toThrow("após o fechamento de aspas");
  });
});

describe("Brasília date display", () => {
  afterEach(() => vi.useRealTimers());

  it("formats timestamps in São Paulo while preserving date-only deadlines", () => {
    expect(dateLabel("2026-10-20T01:30:00Z")).toBe("19 out");
    expect(dateLabel("2026-10-20")).toBe("20 out");
    expect(dateLabel("invalid")).toBe("—");
  });

  it("uses calendar days in Brasília for yesterday and does not call future dates now", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-20T03:15:00Z"));
    expect(relativeDate("2026-10-20T01:30:00Z")).toBe("Ontem");
    expect(relativeDate("2026-10-20T03:05:00Z")).toBe("Há 10 min");
    expect(relativeDate("2026-10-21T12:00:00Z")).toBe("21 out");
    expect(relativeDate("invalid")).toBe("—");
  });
});
