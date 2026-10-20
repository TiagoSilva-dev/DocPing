import { describe, it, expect } from "vitest";
import { latestFailures, type Delivery } from "./notifications";
const delivery = (
  requestId: string,
  status: Delivery["status"],
  createdAt: string,
  channel = "email",
): Delivery => ({
  id: crypto.randomUUID(),
  requestId,
  status,
  createdAt,
  channel,
  kind: "manual",
});
describe("falhas de comunicação visíveis no painel", () => {
  it("remove uma falha antiga quando o provedor aceitou uma tentativa posterior", () => {
    expect(
      latestFailures([
        delivery("r", "accepted", "2026-10-19T15:01:00Z"),
        delivery("r", "failed", "2026-10-19T15:00:00Z"),
      ]),
    ).toEqual([]);
  });
  it("mantém somente a tentativa mais recente de cada solicitação", () => {
    const latest = delivery("r", "failed", "2026-10-19T15:02:00Z");
    expect(
      latestFailures([delivery("r", "failed", "2026-10-19T15:00:00Z"), latest]),
    ).toEqual([latest]);
  });
  it("abrir o WhatsApp não confirma nem apaga uma falha de e-mail", () => {
    const failed = delivery("r", "failed", "2026-10-19T15:00:00Z");
    expect(
      latestFailures([
        failed,
        delivery("r", "opened", "2026-10-19T15:01:00Z", "whatsapp"),
      ]),
    ).toEqual([failed]);
  });
});
