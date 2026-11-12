"""Run against an already-started local production console; no browser required."""
import json
import os
import urllib.error
import urllib.request

origin = os.environ.get("TRS_CONSOLE_URL", "http://127.0.0.1:3000")
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
results = []
for path, method, expected in [("/", "GET", 200), ("/api/snapshot", "GET", 401), ("/api/snapshot", "POST", 405)]:
    try:
        with opener.open(urllib.request.Request(origin + path, method=method), timeout=5) as response:
            status, body = response.status, response.read().decode()
    except urllib.error.HTTPError as error:
        status, body = error.code, error.read().decode()
    assert status == expected, (path, status, expected)
    if path == "/":
        assert "Reliability overview" in body and "No snapshot loaded" in body
        assert "TargetReconciliationService" in body and "TRS" in body
    results.append({"path": path, "method": method, "status": status, "expected": expected, "passed": True})
print(json.dumps(results, indent=2))
