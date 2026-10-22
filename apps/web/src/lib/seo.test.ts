import { describe, expect, it } from "vitest";
import { faqPageLd, softwareApplicationLd, SITE_URL } from "./seo";

describe("softwareApplicationLd", () => {
  const plans = [
    { name: "Essencial", price: "97" },
    { name: "Escritório", price: "237" },
    { name: "Rede", price: "487" },
  ];
  it("reports the range the pricing section prints", () => {
    const data = softwareApplicationLd(plans);
    expect(data.offers.lowPrice).toBe(97);
    expect(data.offers.highPrice).toBe(487);
    expect(data.offers.offerCount).toBe(3);
    expect(data.offers.priceCurrency).toBe("BRL");
    expect(data.offers.offers.map((offer) => offer.name)).toEqual([
      "Essencial",
      "Escritório",
      "Rede",
    ]);
  });
  it("claims no rating the product cannot back", () => {
    expect(softwareApplicationLd(plans)).not.toHaveProperty("aggregateRating");
    expect(softwareApplicationLd(plans)).not.toHaveProperty("review");
  });
  it("points at the canonical host", () => {
    expect(softwareApplicationLd(plans).url).toBe(`${SITE_URL}/`);
  });
});

describe("faqPageLd", () => {
  it("carries every question and its answer", () => {
    const data = faqPageLd([
      ["Meus clientes precisam criar uma conta?", "Não."],
      ["A IA aprova documentos sozinha?", "Só se o seu escritório pedir."],
    ]);
    expect(data.mainEntity).toHaveLength(2);
    expect(data.mainEntity[0].name).toBe(
      "Meus clientes precisam criar uma conta?",
    );
    expect(data.mainEntity[1].acceptedAnswer.text).toBe(
      "Só se o seu escritório pedir.",
    );
  });
});
