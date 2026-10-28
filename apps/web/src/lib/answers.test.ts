import { describe, expect, it } from "vitest";
import {
  BOUNDARIES,
  CRITERIA,
  QUESTIONS,
  SITE_URL,
  STEPS,
  answerSourceHtml,
  howToLd,
  landingLd,
  llmsTxt,
} from "./answers";
import { PLANS } from "./plans";

describe("llmsTxt", () => {
  const text = llmsTxt();
  it("opens the way an llms.txt is read: one H1 and a summary", () => {
    const lines = text.split("\n");
    expect(lines[0]).toBe("# DocPing");
    expect(lines[2].startsWith("> ")).toBe(true);
    expect(text.match(/^# /gm)).toHaveLength(1);
  });
  it("prints every plan at the price the pricing section charges", () => {
    for (const plan of PLANS) {
      expect(text).toContain(`| ${plan.name} | R$ ${plan.price} |`);
    }
  });
  it("carries every question and every criterion", () => {
    for (const [question] of QUESTIONS)
      expect(text).toContain(`### ${question}`);
    for (const [name] of CRITERIA) expect(text).toContain(`**${name}**`);
  });
  it("says what the product does not do, not only what it does", () => {
    for (const [title] of BOUNDARIES) expect(text).toContain(`**${title}**`);
    expect(text).toContain("não foram calibrados");
  });
  it("links the public pages and no private one", () => {
    expect(text).toContain(
      `[Política de Privacidade](${SITE_URL}/privacidade)`,
    );
    for (const path of ["/app", "/p/", "/entrar", "/assinar"]) {
      expect(text).not.toContain(`${SITE_URL}${path}`);
    }
  });
});

describe("answerSourceHtml", () => {
  const html = answerSourceHtml();
  it("is a single element with one heading of its own", () => {
    expect(html.startsWith('<div id="answer-source"')).toBe(true);
    expect(html.endsWith("</div>")).toBe(true);
    expect(html.match(/<h1>/g)).toHaveLength(1);
  });
  it("renders the same facts the page renders", () => {
    for (const step of STEPS) expect(html).toContain(step.text);
    for (const [question] of QUESTIONS)
      expect(html).toContain(`<h3>${question}`);
    for (const plan of PLANS)
      expect(html).toContain(`R$ ${plan.price} por mês`);
  });
  it("cannot close the document it is written into", () => {
    expect(html).not.toContain("</script");
    expect(html.includes("<script")).toBe(false);
  });
});

describe("landingLd", () => {
  const graph = landingLd(PLANS)["@graph"];
  it("describes the product, the FAQ and the steps, once each", () => {
    expect(graph.map((node) => node["@type"])).toEqual([
      "SoftwareApplication",
      "FAQPage",
      "HowTo",
    ]);
  });
  it("numbers the steps in the order the page shows them", () => {
    const steps = howToLd().step;
    expect(steps.map((step) => step.position)).toEqual([1, 2, 3]);
    expect(steps[0].text).toBe(STEPS[0].text);
  });
});
