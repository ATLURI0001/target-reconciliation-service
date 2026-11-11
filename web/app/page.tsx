"use client";

import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { activeIncident, clusterStatus, parseSnapshot, workloadStatus } from "@/lib/model";
import type { Snapshot } from "@/lib/model";

function Badge({value}: {value: string}) {
  const good = ["Healthy", "Connected", "Resolved"].includes(value);
  const bad = ["Failed", "Rejected", "Disconnected", "Degraded"].includes(value);
  const warn = ["Stale", "Unobservable", "AwaitingApproval", "Emergency stop", "DryRun"].includes(value);
  return <span className={`badge ${good ? "good" : bad ? "bad" : warn ? "warn" : ""}`}>{value}</span>;
}
function timestamp(value: string) {
  const date = new Date(value);
  return Number.isFinite(date.getTime()) && date.getUTCFullYear() > 1970 ? date.toISOString().replace("T", " ").slice(0, 19) + " UTC" : "Not observed";
}

export default function Home() {
  const [draftToken, setDraftToken] = useState("");
  const [token, setToken] = useState("");
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fetchedAt, setFetchedAt] = useState("");
  const [now, setNow] = useState(0);
  const activeRequest = useRef<AbortController | null>(null);
  const busyRef = useRef(false);

  const refresh = useCallback(async (accessToken: string) => {
    if (!accessToken || busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    const controller = new AbortController();
    activeRequest.current = controller;
    try {
      const response = await fetch("/api/snapshot", {
        headers: {Authorization: `Bearer ${accessToken}`}, cache: "no-store", signal: controller.signal
      });
      const result: unknown = await response.json();
      if (!response.ok) {
        const message = result && typeof result === "object" && "error" in result && typeof result.error === "string" ? result.error : "Snapshot request failed.";
        throw new Error(message);
      }
      const data = parseSnapshot(result);
      setSnapshot(data);
      setFetchedAt(new Date().toISOString());
      setNow(Date.now());
      setError("");
    } catch (cause) {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : "Could not load the management API.");
    } finally {
      busyRef.current = false;
      setBusy(false);
      activeRequest.current = null;
    }
  }, []);

  useEffect(() => {
    if (!token) return;
    void refresh(token);
    const timer = setInterval(() => void refresh(token), 10000);
    const clock = setInterval(() => setNow(Date.now()), 1000);
    return () => {clearInterval(timer); clearInterval(clock); activeRequest.current?.abort();};
  }, [token, refresh]);

  function connect(event: FormEvent) {
    event.preventDefault();
    const value = draftToken.trim();
    if (value.length < 16 || /\s/.test(value)) {setError("Enter a development token with at least 16 characters and no spaces."); return;}
    setError("");
    setToken(value);
    setDraftToken("");
  }
  function disconnect() {
    activeRequest.current?.abort();
    setToken(""); setDraftToken(""); setSnapshot(null); setFetchedAt(""); setError("");
  }

  const clusters = snapshot ? Object.values(snapshot.clusters).sort((a,b) => a.id.localeCompare(b.id)) : [];
  const workloads = snapshot ? Object.values(snapshot.observations).sort((a,b) => `${a.cluster}/${a.namespace}/${a.name}`.localeCompare(`${b.cluster}/${b.namespace}/${b.name}`)) : [];
  const incidents = snapshot ? Object.values(snapshot.incidents).sort((a,b) => Date.parse(b.created) - Date.parse(a.created)) : [];
  const audit = snapshot ? [...snapshot.audit].sort((a,b) => b.sequence - a.sequence) : [];
  const connected = snapshot ? clusters.filter(c => clusterStatus(c, snapshot.policy, now) === "Connected").length : 0;
  const healthy = snapshot ? workloads.filter(w => workloadStatus(w, snapshot.policy, now) === "Healthy").length : 0;

  return <div className="shell">
    <aside className="sidebar">
      <a className="brand" href="#overview"><span className="brand-mark" aria-hidden="true">⌁</span>ResiliencePlane</a>
      <div className="eyebrow" style={{marginTop:12}}>Reliability control plane</div>
      <nav className="nav" aria-label="Console sections">
        <a href="#overview">Overview</a><a href="#clusters">Cluster inventory</a><a href="#workloads">Workloads</a><a href="#incidents">Incidents & recovery</a><a href="#policy">Safety policy</a><a href="#audit">Audit history</a>
      </nav>
      <div className="sidebar-note"><Badge value="LOCAL DEVELOPMENT"/><p style={{marginTop:12}}>Read-only console<br/>Core workflow milestone</p><p style={{marginTop:14}}>Authenticated API data.<br/>No simulated telemetry.</p></div>
    </aside>
    <main id="overview">
      <div className="topbar"><span>CONTROL PLANE <span style={{margin:"0 9px",color:"#45546a"}}>/</span> OPERATIONS</span><span>{snapshot && !error ? <><span className="status-dot"/>API snapshot available</> : "Waiting for management API"}</span></div>
      <div className="title-row"><div><h1>Reliability overview</h1><p className="muted">Observe infrastructure. Review evidence. Govern recovery.</p></div><div className="actions"><Badge value="READ ONLY"/>{token && <><button className="button secondary" disabled={busy} onClick={() => void refresh(token)}>{busy ? "Refreshing…" : "Refresh snapshot"}</button><button className="button secondary" onClick={disconnect}>Disconnect</button></>}</div></div>
      {!token && <><div className="notice">Connect to the local Go management API with the viewer token generated during setup. The token stays in this tab’s memory and is cleared on reload. This console does not approve or execute changes.</div><form className="auth" onSubmit={connect}><div className="field"><label htmlFor="access-token">Development access token</label><input id="access-token" type="password" autoComplete="off" value={draftToken} onChange={event => setDraftToken(event.target.value)} placeholder="Paste the local viewer token" minLength={16} maxLength={4096} required/></div><button className="button" type="submit">Connect to API</button></form></>}
      {error && <div className="notice error" role="alert">{error}{snapshot && <span> Previously received data remains visible below. Freshness continues to age.</span>}</div>}
      <div role="status" aria-live="polite">{busy && !snapshot && <p className="loading">Loading the current control-plane snapshot…</p>}</div>
      {!snapshot && !busy && <section className="panel"><div className="empty">No snapshot loaded. Cluster health, workload availability, incidents, and audit events will appear after a successful API connection.</div></section>}
      {snapshot && <>
        <div className="stats" aria-label="Snapshot summary">
          <article className="stat"><div className="eyebrow">Registered clusters</div><div className="stat-value">{clusters.length}</div><p className="stat-caption">{connected} connected with fresh telemetry</p></article>
          <article className="stat"><div className="eyebrow">Observed workloads</div><div className="stat-value">{workloads.length}</div><p className="stat-caption">{healthy} healthy · freshness required</p></article>
          <article className="stat"><div className="eyebrow">Active incidents</div><div className="stat-value">{incidents.filter(activeIncident).length}</div><p className="stat-caption">{incidents.filter(i => i.state === "AwaitingApproval").length} awaiting explicit approval</p></article>
          <article className="stat"><div className="eyebrow">Audit events shown</div><div className="stat-value">{audit.length}</div><p className="stat-caption">Most recent 200 persisted events</p></article>
        </div>
        <div className="notice">{snapshot.policy.emergencyStop ? "Emergency stop is active. " : ""}{snapshot.policy.dryRun ? "Dry-run policy is active. " : "Dry-run policy is disabled. "}Observation freshness expires after {snapshot.policy.freshnessSeconds}s. Missing telemetry is treated as unobservable.</div>
        <section className="panel" id="clusters"><div className="panel-head"><div><h2>Cluster inventory</h2><p>Registration and connectivity reported by the control plane</p></div><Badge value={`${clusters.length} registered`}/></div>{clusters.length ? <div className="table-wrap"><table><thead><tr><th scope="col">Cluster</th><th scope="col">Environment</th><th scope="col">Connectivity</th><th scope="col">Last observation</th></tr></thead><tbody>{clusters.map(cluster => <tr key={cluster.id}><td><span className="mono">{cluster.id}</span></td><td>{cluster.environment}</td><td><Badge value={clusterStatus(cluster,snapshot.policy,now)}/></td><td className="mono">{timestamp(cluster.lastSeen)}</td></tr>)}</tbody></table></div> : <div className="empty">No clusters registered. Register a local test cluster through the management API.</div>}</section>
        <section className="panel" id="workloads"><div className="panel-head"><div><h2>Workload observations</h2><p>Deployment availability and revision evidence from Kubernetes</p></div><Badge value="Deployment status"/></div>{workloads.length ? <div className="table-wrap"><table><thead><tr><th scope="col">Workload</th><th scope="col">Health</th><th scope="col">Replicas</th><th scope="col">Recovery ownership</th><th scope="col">Observed</th></tr></thead><tbody>{workloads.map(workload => <tr key={`${workload.cluster}/${workload.namespace}/${workload.name}`}><td><span className="mono">{workload.name}</span><div className="subline">{workload.cluster} / {workload.namespace}</div><details className="subline"><summary>Image reference</summary><code>{workload.image}</code></details></td><td><Badge value={workloadStatus(workload,snapshot.policy,now)}/></td><td><span className="mono">{workload.available} / {workload.desired}</span><div className="subline">available / desired · {workload.updated} updated</div></td><td>{workload.gitops ? "GitOps managed" : workload.optIn ? "Explicitly opted in" : "No automation opt-in"}<div className="subline">Generation {workload.observedGeneration} / {workload.generation}</div></td><td className="mono">{timestamp(workload.at)}</td></tr>)}</tbody></table></div> : <div className="empty">No workload observations received. Configure a ReliabilityTarget in the permitted test namespace.</div>}</section>
        <section className="panel" id="incidents"><div className="panel-head"><div><h2>Incidents & recovery plans</h2><p>Deterministic diagnoses with approval and verification evidence</p></div><Badge value={`${incidents.length} total`}/></div>{incidents.length ? <div>{incidents.map(incident => <article className="incident" key={incident.id}><div className="incident-title"><div><h3 className="mono">{incident.target}</h3><div className="subline">{incident.id} · {timestamp(incident.updated)}</div></div><Badge value={incident.state}/></div><p>{incident.plan.reason}</p>{incident.error && <p style={{color:"#f5a5af"}}>Execution result: {incident.error}</p>}<details><summary>Inspect recovery plan</summary><dl><dt>Action</dt><dd>{incident.plan.action}</dd><dt>Target image</dt><dd className="mono">{incident.plan.image || "No image selected"}</dd><dt>Approval</dt><dd>{incident.approvedBy ? `Approved by ${incident.approvedBy}` : incident.plan.requiresApproval ? "Explicit approval required" : "See policy decision"}</dd><dt>Verify</dt><dd>{incident.plan.verification}</dd><dt>Abort</dt><dd>{incident.plan.abort}</dd></dl></details></article>)}</div> : <div className="empty">No incidents recorded. Healthy workloads do not generate demonstration incidents.</div>}</section>
        <section className="panel" id="policy"><div className="panel-head"><div><h2>Operational safety policy</h2><p>Current authoritative policy · changes require the management API</p></div><Badge value={snapshot.policy.emergencyStop ? "Emergency stop" : snapshot.policy.dryRun ? "DryRun" : "Live execution enabled"}/></div><div className="policy-grid"><div><div className="eyebrow">Permitted namespaces</div><p className="mono">{snapshot.policy.namespaces.join(", ")}</p></div><div><div className="eyebrow">Maximum blast radius</div><p>{snapshot.policy.maxReplicas} replicas per target</p></div><div><div className="eyebrow">Recovery concurrency</div><p>{snapshot.policy.maxConcurrent} active operation(s)</p></div><div><div className="eyebrow">Freshness limit</div><p>{snapshot.policy.freshnessSeconds} seconds</p></div><div><div className="eyebrow">Detection window</div><p>{snapshot.policy.detectionSeconds} seconds unhealthy</p></div><div><div className="eyebrow">Execution mode</div><p>{snapshot.policy.dryRun ? "Dry run" : "Approval gated"}</p></div></div></section>
        <section className="panel" id="audit"><div className="panel-head"><div><h2>Audit history</h2><p>Most recent 200 identity-linked events · full history via paginated /v1/audit</p></div><Badge value="Persisted"/></div>{audit.length ? <div className="table-wrap"><table><thead><tr><th scope="col">Sequence / time</th><th scope="col">Identity</th><th scope="col">Action / resource</th><th scope="col">Result</th></tr></thead><tbody>{audit.map(event => <tr key={event.sequence}><td><span className="mono">#{event.sequence}</span><div className="subline mono">{timestamp(event.at)}</div></td><td className="mono">{event.actor}</td><td>{event.action}<div className="subline mono">{event.resource}</div></td><td>{event.result}</td></tr>)}</tbody></table></div> : <div className="empty">No audit events in the current snapshot.</div>}</section>
      </>}
      <footer><p>ResiliencePlane / executable foundation / read-only console</p><p>{fetchedAt ? `Snapshot received ${timestamp(fetchedAt)} · refresh every 10s` : "No backend data loaded"}</p></footer>
    </main>
  </div>;
}
