import { describe, expect, it } from "vitest";
import { PLANS, planName, subscriptionLabel } from "./plans";

describe("planos", () => {
  // The id is not a label: it is the slug posted to /api/billing/checkout and
  // the one the API maps to a Stripe price. A typo here is a checkout that
  // refuses every plan, and nothing on screen would say why.
  it("usa exatamente os slugs que a API vende", () => {
    expect(PLANS.map((plan) => plan.id)).toEqual([
      "essencial",
      "escritorio",
      "rede",
    ]);
  });

  // Volume is not what is sold. The ceilings in domain.entitlements protect
  // the infrastructure; they are not a product boundary, and printing one
  // here would turn it back into a promise -- and into a refusal the day an
  // office reaches it in the middle of a closing.
  it("não vende volume: nenhum card anuncia quantidade ou espaço", () => {
    const sold = PLANS.map((plan) => `${plan.pitch} ${plan.items.join(" ")}`);
    for (const card of sold) {
      expect(card).not.toMatch(/\bGB\b/);
      expect(card).not.toMatch(/\d+\s*solicitações/);
      expect(card).not.toMatch(/\bpor solicitação\b/);
    }
  });

  it("tem um único plano em destaque", () => {
    expect(PLANS.filter((plan) => plan.featured)).toHaveLength(1);
  });

  it("nomeia também os espaços que nunca compraram um plano", () => {
    expect(planName("escritorio")).toBe("Escritório");
    expect(planName("founder_pilot")).toBe("Piloto fundador");
    expect(planName("demo")).toBe("Demonstração");
    expect(planName("")).toBe("Sem plano");
    expect(planName("inventado")).toBe("Sem plano");
  });

  // Stripe's vocabulary is stored verbatim and only read in Portuguese. An
  // unknown status must read as no subscription rather than as an empty
  // space that looks like a loading state.
  it("traduz o estado da assinatura sem inventar um ativo", () => {
    expect(subscriptionLabel("active")).toBe("Ativa");
    expect(subscriptionLabel("past_due")).toBe("Pagamento pendente");
    expect(subscriptionLabel("canceled")).toBe("Cancelada");
    expect(subscriptionLabel("")).toBe("Sem assinatura");
    expect(subscriptionLabel("algo_novo_da_stripe")).toBe("Sem assinatura");
  });
});
