#!/usr/bin/env python3
"""Actual HTTP/PG/CLI/optional Next proxy smoke. No synthetic health reports."""
import json, os, secrets, socket, statistics, subprocess, tempfile, time
from pathlib import Path
from urllib.request import Request, build_opener, ProxyHandler
from urllib.error import HTTPError, URLError

root = Path(__file__).resolve().parents[1]
assert os.environ.get('TEST_DATABASE_URL'), 'Set TEST_DATABASE_URL to an isolated PostgreSQL database'
assert (root/'bin/target-reconciliation-service').is_file(), 'Run make build first'
op, viewer = secrets.token_hex(32), secrets.token_hex(32)
http = build_opener(ProxyHandler({}))
def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1',0)); return s.getsockname()[1]
api = f'http://127.0.0.1:{port()}'
def request(base, path, token='', data=None, method='GET'):
    req = Request(base+path, data=json.dumps(data).encode() if data is not None else None, method=method,
                  headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'})
    try:
        with http.open(req,timeout=10) as res: return res.status,res.read()
    except HTTPError as e: return e.code,e.read()
def wait_ready(base,path,process):
    for _ in range(100):
        assert process.poll() is None, 'Server stopped before readiness'
        try:
            if request(base,path)[0]==200:return
        except (URLError,ConnectionError):pass
        time.sleep(.1)
    raise AssertionError('Server readiness timeout')
env={**os.environ,'DATABASE_URL':os.environ['TEST_DATABASE_URL'],'TRS_LISTEN':api.removeprefix('http://'),
     'TRS_OPERATOR_TOKEN':op,'TRS_VIEWER_TOKEN':viewer,'TRS_KUBERNETES':'false','NO_PROXY':'localhost,127.0.0.1'}
children=[]
with tempfile.TemporaryFile() as logs:
    try:
        server=subprocess.Popen([str(root/'bin/target-reconciliation-service')],cwd=root,env=env,stdout=logs,stderr=logs);children.append(server)
        wait_ready(api,'/readyz',server)
        assert request(api,'/v1/snapshot')[0]==401
        cluster='smoke-'+secrets.token_hex(4)
        payload={'id':cluster,'environment':'test'}
        assert request(api,'/v1/clusters',viewer,payload,'POST')[0]==403
        assert request(api,'/v1/clusters',op,payload,'POST')[0]==200
        assert request(api,'/v1/clusters',op,{'id':'invalid/cluster','environment':'test'},'POST')[0]==400
        status,body=request(api,'/v1/snapshot',viewer);state=json.loads(body)
        assert status==200 and state['clusters'][cluster]['connected'] is False
        status,body=request(api,'/v1/audit?limit=1',viewer);page=json.loads(body)
        assert status==200 and len(page['items'])==1 and page['nextCursor']>=1
        assert request(api,'/v1/audit?limit=201',viewer)[0]==400
        status,metrics=request(api,'/metrics');assert status==200 and b'trs_http_requests_total' in metrics
        cli=subprocess.run([str(root/'bin/trsctl'),'snapshot'],env={**env,'TRS_API_URL':api,'TRS_TOKEN':viewer},capture_output=True,check=True)
        assert cluster in json.loads(cli.stdout)['clusters']
        measurements=[]
        for _ in range(30):
            start=time.perf_counter();assert request(api,'/v1/snapshot',viewer)[0]==200
            measurements.append((time.perf_counter()-start)*1000)
        result={'api':'PASS','postgresPersistence':'PASS','viewerMutationDenied':'PASS','unobservedClusterNotHealthy':'PASS',
                'semanticValidation':'PASS','auditPagination':'PASS','metrics':'PASS','goCLI':'PASS',
                'requestSample':{'kind':'sequential loopback HTTP snapshot, PostgreSQL-backed; not a load capacity test',
                                 'requests':len(measurements),'registeredClusters':len(state['clusters']),
                                 'observedTargets':len(state['observations']),'auditEventsShown':len(state['audit']),
                                 'medianMs':round(statistics.median(measurements),3),
                                 'p95Ms':round(sorted(measurements)[28],3)}}
        if os.environ.get('TRS_TEST_CONSOLE')=='1':
            console=f'http://127.0.0.1:{port()}'
            node=os.environ.get('CODEX_PRIMARY_RUNTIME_NODE','node')
            web=subprocess.Popen([node,str(root/'web/node_modules/next/dist/bin/next'),'start',str(root/'web'),'--hostname','127.0.0.1','--port',console.rsplit(':',1)[1]],cwd=root,env={**env,'TRS_API_URL':api,'NEXT_TELEMETRY_DISABLED':'1'},stdout=logs,stderr=logs)
            children.append(web);wait_ready(console,'/',web)
            assert request(console,'/api/snapshot')[0]==401
            status,body=request(console,'/api/snapshot',viewer)
            assert status==200 and cluster in json.loads(body)['clusters']
            result['nextToGoToPostgres']='PASS'
        else:result['nextToGoToPostgres']='NOT RUN (set TRS_TEST_CONSOLE=1 after npm ci and npm run build)'
        print(json.dumps(result,indent=2))
    finally:
        for process in reversed(children):
            process.terminate()
            try:process.wait(timeout=10)
            except subprocess.TimeoutExpired:process.kill();process.wait()
