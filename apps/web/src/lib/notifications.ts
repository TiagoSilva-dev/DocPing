export interface Delivery {
  id: string;
  requestId: string;
  channel: string;
  kind: string;
  status: "pending" | "accepted" | "failed" | "opened";
  error?: string;
  createdAt: string;
}
export function latestFailures(deliveries: Delivery[]) {
  const latest = new Map<string, Delivery>();
  for (const delivery of deliveries) {
    if (
      delivery.channel === "email" &&
      (!latest.has(delivery.requestId) ||
        latest.get(delivery.requestId)!.createdAt <= delivery.createdAt)
    )
      latest.set(delivery.requestId, delivery);
  }
  return [...latest.values()].filter((d) => d.status === "failed");
}
