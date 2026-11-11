import test from "node:test";
import assert from "node:assert/strict";
import {fresh, clusterStatus, workloadStatus, activeIncident, parseSnapshot} from "../lib/model.ts";
const now = Date.parse("2026-11-11T12:00:00Z");
const policy = {namespaces:["rp-demo"], maxReplicas:3, maxConcurrent:1, freshnessSeconds:30, detectionSeconds:30, dryRun:true, emergencyStop:false};
const observation = {cluster:"local", namespace:"rp-demo", name:"catalog", uid:"uid", resourceVersion:"42", image:"bad", healthyImage:"good", container:"app", generation:2, observedGeneration:2, desired:2, available:2, updated:2, optIn:true, gitops:false, at:"2026-11-11T11:59:50Z"};
test("freshness rejects invalid and implausibly future timestamps", () => {
  assert.equal(fresh("invalid",30,now),false);
  assert.equal(fresh("2026-11-11T12:01:00Z",30,now),false);
  assert.equal(fresh("2026-11-11T11:59:30Z",30,now),true);
  assert.equal(fresh("2026-11-11T11:59:29Z",30,now),false);
});
test("stale connectivity never appears healthy", () => {
  const cluster = {id:"local",environment:"test",connected:true,lastSeen:"2026-11-11T11:59:00Z"};
  assert.equal(clusterStatus(cluster,policy,now),"Stale");
});
test("workload requires fresh, available, updated and observed generation evidence", () => {
  assert.equal(workloadStatus(observation,policy,now),"Healthy");
  assert.equal(workloadStatus({...observation,observedGeneration:1},policy,now),"Degraded");
  assert.equal(workloadStatus({...observation,updated:1},policy,now),"Degraded");
  assert.equal(workloadStatus({...observation,at:"2026-11-11T11:00:00Z"},policy,now),"Unobservable");
  assert.equal(workloadStatus({...observation,desired:0},policy,now),"Scaled to zero");
});
test("terminal and dry-run incidents are excluded from active count", () => {
  for (const state of ["Resolved","Failed","Cancelled","Rejected","DryRun"]) assert.equal(activeIncident({state}),false);
  assert.equal(activeIncident({state:"AwaitingApproval"}),true);
});
test("transport rejects malformed policy and accepts empty authoritative state", () => {
  const snapshot = {clusters:{},observations:{},unhealthySince:{},incidents:{},audit:[],policy};
  assert.deepEqual(parseSnapshot(snapshot),snapshot);
  assert.throws(() => parseSnapshot({...snapshot,policy:{...policy,freshnessSeconds:0}}));
  assert.throws(() => parseSnapshot({...snapshot,clusters:[]}));
  assert.throws(() => parseSnapshot({...snapshot,incidents:{bad:{state:"Approved"}}}));
});
