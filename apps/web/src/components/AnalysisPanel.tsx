import {
  AlertTriangle,
  BrainCircuit,
  Check,
  Loader2,
  RotateCw,
  X,
} from "lucide-react";
import type { Analysis, AnalysisSignal, AnalysisVerdict } from "@/lib/types";
import "./analysis-panel.css";

const verdictCopy: Record<
  AnalysisVerdict,
  { title: string; icon: typeof Check }
> = {
  approved: { title: "Aprovado na análise", icon: Check },
  review: { title: "Revisão humana recomendada", icon: AlertTriangle },
  rejected: { title: "Reprovado na análise", icon: X },
};

/** The bar length is the model's confidence, which on its own is misleading:
 *  "4% with 97% confidence" would draw the same long bar as "95%". The tone
 *  says whether the answer itself is favourable, so a confidently bad signal
 *  reads as a long red bar rather than a long neutral one. */
const goodAnswers = new Set(["correta", "confere", "nenhum"]);
const unknownAnswers = new Set(["ausente", "nao_informada", "nao_informado"]);

function signalTone(signal: AnalysisSignal): "ok" | "bad" | "neutral" {
  if (signal.kind === "choice") {
    const answer = (signal.answer || "").toLowerCase();
    if (!answer || unknownAnswers.has(answer)) return "neutral";
    return goodAnswers.has(answer) ? "ok" : "bad";
  }
  if (signal.kind === "score") {
    if (signal.value >= 2) return "ok";
    return signal.value < 1 ? "bad" : "neutral";
  }
  if (signal.value >= 0.85) return "ok";
  return signal.value <= 0.3 ? "bad" : "neutral";
}

/** Signals are shown as the model answered them, not as a single number: a
 *  verdict the office cannot interrogate is one it will not trust. */
function signalValue(signal: AnalysisSignal) {
  if (signal.kind === "choice") return signal.answer?.replace(/_/g, " ") || "—";
  if (signal.kind === "score")
    return `${signal.value.toFixed(1)} de 3,0`;
  return `${Math.round(signal.value * 100)}%`;
}

export function AnalysisPanel({
  analysis,
  onRetry,
  retrying,
}: {
  analysis: Analysis;
  onRetry?: () => void;
  retrying?: boolean;
}) {
  if (analysis.status === "disabled") return null;
  if (analysis.status === "queued" || analysis.status === "running") {
    return (
      <div className="analysis-panel pending">
        <Loader2 size={15} className="spin" />
        <div>
          <strong>Analisando o documento…</strong>
          <p>
            Extraindo o texto e conferindo os critérios. Você pode revisar
            manualmente a qualquer momento.
          </p>
        </div>
      </div>
    );
  }
  if (analysis.status === "failed") {
    return (
      <div className="analysis-panel failed">
        <AlertTriangle size={15} />
        <div>
          <strong>A análise automática não foi concluída</strong>
          <p>{analysis.error || "O serviço de análise não respondeu."}</p>
        </div>
        {onRetry && (
          <button type="button" onClick={onRetry} disabled={retrying}>
            <RotateCw size={13} className={retrying ? "spin" : undefined} />
            Tentar de novo
          </button>
        )}
      </div>
    );
  }
  const verdict = analysis.verdict ? verdictCopy[analysis.verdict] : null;
  if (!verdict) return null;
  const Icon = verdict.icon;
  return (
    <div className={`analysis-panel ${analysis.verdict}`}>
      <div className="analysis-head">
        <span className="analysis-mark">
          <Icon size={14} />
        </span>
        <div>
          <strong>
            {verdict.title}
            {analysis.applied && <em>aplicado automaticamente</em>}
          </strong>
          <p>{analysis.reason}</p>
        </div>
        <span className="analysis-confidence" title="Confiança do modelo">
          {Math.round(analysis.confidence * 100)}%
        </span>
      </div>
      {analysis.signals && analysis.signals.length > 0 && (
        <dl className="analysis-signals">
          {analysis.signals.map((signal) => (
            <div key={signal.key} className={signalTone(signal)}>
              <dt>{signal.label}</dt>
              <dd>
                <span>{signalValue(signal)}</span>
                <i
                  aria-hidden="true"
                  style={{ width: `${Math.round(signal.confidence * 100)}%` }}
                />
                <small>
                  {Math.round(signal.confidence * 100)}% de confiança
                </small>
              </dd>
            </div>
          ))}
        </dl>
      )}
      <div className="analysis-foot">
        <BrainCircuit size={12} />
        <span>
          {[
            analysis.extractor === "paddleocr"
              ? "PaddleOCR"
              : analysis.extractor === "docling"
                ? "Docling"
                : null,
            analysis.model,
            analysis.characters
              ? `${analysis.characters.toLocaleString("pt-BR")} caracteres`
              : null,
          ]
            .filter(Boolean)
            .join(" · ")}
        </span>
        {onRetry && (
          <button type="button" onClick={onRetry} disabled={retrying}>
            <RotateCw size={12} className={retrying ? "spin" : undefined} />
            Analisar de novo
          </button>
        )}
      </div>
    </div>
  );
}
