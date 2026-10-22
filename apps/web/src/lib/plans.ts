// The three plans on sale, in one place. The landing page sells them and the
// subscribe screen charges for them, so a figure edited in one and not the
// other would be a promise the product breaks.
//
// These numbers are the same ones the API enforces, in
// internal/domain/models.go: fifteen, sixty-five and two hundred active
// requests, five, thirty and a hundred and twenty GB. Changing a figure here
// without changing it there turns the pricing page back into a brochure.
export interface Plan {
  id: string;
  name: string;
  price: string;
  unit: string;
  pitch: string;
  featured: boolean;
  items: string[];
}

export const PLANS: Plan[] = [
  {
    id: "essencial",
    name: "Essencial",
    price: "97",
    unit: "R$ 6,47 por solicitação ativa",
    pitch: "Para o escritório que está organizando a coleta pela primeira vez.",
    featured: false,
    items: [
      "15 solicitações ativas",
      "5 GB de armazenamento",
      "Análise automática incluída",
      "Modelos e recorrências",
    ],
  },
  {
    id: "escritorio",
    name: "Escritório",
    price: "237",
    unit: "R$ 3,65 por solicitação ativa",
    pitch: "Para a rotina de fechamento com uma carteira inteira todo mês.",
    featured: true,
    items: [
      "65 solicitações ativas",
      "30 GB de armazenamento",
      "Análise automática incluída",
      "Equipe do mesmo domínio",
      "Sua marca no portal do cliente",
    ],
  },
  {
    id: "rede",
    name: "Rede",
    price: "487",
    unit: "R$ 2,44 por solicitação ativa",
    pitch: "Para quem atende centenas de empresas e não pode perder prazo.",
    featured: false,
    items: [
      "200 solicitações ativas",
      "120 GB de armazenamento",
      "Análise automática incluída",
      "Tudo do plano Escritório",
    ],
  },
];

// planName turns a stored slug into something a person reads. The two slugs
// that are not on sale are workspaces that never bought one: the pilot that
// predates billing and the demonstration.
export function planName(plan: string): string {
  const sold = PLANS.find((p) => p.id === plan);
  if (sold) return sold.name;
  if (plan === "founder_pilot") return "Piloto fundador";
  if (plan === "demo") return "Demonstração";
  return "Sem plano";
}

// Stripe's subscription vocabulary, said in Portuguese. The status is stored
// exactly as Stripe reports it; only the reading is translated.
export function subscriptionLabel(status: string): string {
  switch (status) {
    case "active":
      return "Ativa";
    case "trialing":
      return "Em teste";
    case "past_due":
      return "Pagamento pendente";
    case "canceled":
      return "Cancelada";
    case "unpaid":
      return "Não paga";
    case "incomplete":
    case "incomplete_expired":
      return "Pagamento não concluído";
    case "paused":
      return "Pausada";
    default:
      return "Sem assinatura";
  }
}
