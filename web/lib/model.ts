export type Cluster = {id: string; environment: string; lastSeen: string; connected: boolean};
export type Observation = {
  cluster: string; namespace: string; name: string; uid: string; resourceVersion: string;
  image: string; healthyImage: string; container: string; generation: number;
  observedGeneration: number; desired: number; available: number; updated: number; replicas: number;
  optIn: boolean; gitops: boolean; at: string; executionKey?: string;
};
export type Policy = {namespaces: string[]; maxReplicas: number; maxConcurrent: number;
  freshnessSeconds: number; detectionSeconds: number; dryRun: boolean; emergencyStop: boolean};
export type Incident = {id: string; target: string; state: string; created: string; updated: string;
  approvedBy?: string; error?: string; deadline?: string; executionKey?: string;
  plan: {action: string; reason: string; target: Observation; image: string;
    requiresApproval: boolean; verification: string; abort: string}};
export type Audit = {sequence: number; at: string; actor: string; action: string; resource: string; result: string};
export type Snapshot = {clusters: Record<string, Cluster>; observations: Record<string, Observation>;
  unhealthySince: Record<string, string>; incidents: Record<string, Incident>; audit: Audit[]; policy: Policy};

export function fresh(at: string, freshnessSeconds: number, now: number): boolean {
  const time = Date.parse(at);
  return Number.isFinite(time) && time <= now + 5000 && now - time <= freshnessSeconds * 1000;
}
export function clusterStatus(cluster: Cluster, policy: Policy, now: number): string {
  if (!fresh(cluster.lastSeen, policy.freshnessSeconds, now)) return "Stale";
  return cluster.connected ? "Connected" : "Disconnected";
}
export function workloadStatus(observation: Observation, policy: Policy, now: number): string {
  if (!fresh(observation.at, policy.freshnessSeconds, now)) return "Unobservable";
  if (observation.desired === 0) return "Scaled to zero";
  return observation.replicas === observation.desired && observation.available >= observation.desired && observation.updated === observation.desired &&
    observation.observedGeneration >= observation.generation ? "Healthy" : "Degraded";
}
export function activeIncident(incident: Incident): boolean {
  return !["Resolved", "Cancelled", "Failed", "Rejected", "DryRun"].includes(incident.state);
}
// Guard the transport boundary before treating a response as an API snapshot.
export function parseSnapshot(input: unknown): Snapshot {
  if (!input || typeof input !== "object") throw new Error("Invalid snapshot response");
  const value = input as Record<string, unknown>;
  for (const key of ["clusters", "observations", "unhealthySince", "incidents"]) {
    if (!value[key] || typeof value[key] !== "object" || Array.isArray(value[key])) throw new Error(`Invalid ${key} data`);
  }
  if (!Array.isArray(value.audit) || !value.policy || typeof value.policy !== "object") throw new Error("Invalid policy or audit data");
  const policy = value.policy as Policy;
  if (!Array.isArray(policy.namespaces) || !policy.namespaces.every(n => typeof n === "string") ||
      !Number.isFinite(policy.freshnessSeconds) || policy.freshnessSeconds <= 0 ||
      !Number.isFinite(policy.maxReplicas) || !Number.isFinite(policy.maxConcurrent) ||
      !Number.isFinite(policy.detectionSeconds) || typeof policy.dryRun !== "boolean" ||
      typeof policy.emergencyStop !== "boolean") throw new Error("Invalid policy data");
  for (const cluster of Object.values(value.clusters as Record<string, Cluster>)) {
    if (!cluster || typeof cluster.id !== "string" || typeof cluster.lastSeen !== "string" || typeof cluster.connected !== "boolean") throw new Error("Invalid cluster data");
  }
  for (const observation of Object.values(value.observations as Record<string, Observation>)) {
    if (!observation || typeof observation.name !== "string" || typeof observation.at !== "string" ||
      ![observation.desired, observation.available, observation.updated, observation.replicas, observation.generation, observation.observedGeneration].every(Number.isFinite)) throw new Error("Invalid workload data");
  }
  for (const incident of Object.values(value.incidents as Record<string, Incident>)) {
    if (!incident || typeof incident.id !== "string" || typeof incident.state !== "string" || typeof incident.target !== "string" ||
      !incident.plan || typeof incident.plan.reason !== "string" || typeof incident.plan.verification !== "string" || typeof incident.plan.abort !== "string") throw new Error("Invalid incident data");
  }
  for (const event of value.audit as Audit[]) {
    if (!event || !Number.isInteger(event.sequence) || typeof event.at !== "string" || typeof event.actor !== "string" || typeof event.action !== "string" || typeof event.result !== "string") throw new Error("Invalid audit data");
  }
  return value as Snapshot;
}
