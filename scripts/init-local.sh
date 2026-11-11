#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
command -v python3 >/dev/null || { printf '%s\n' 'Install Python 3 to generate local credentials.' >&2; exit 1; }
umask 077
python3 - <<'PY'
import fcntl, os, secrets
# Append new integration secrets without rotating existing passwords or tokens.
flags = os.O_CREAT | os.O_APPEND | os.O_RDWR | getattr(os, 'O_NOFOLLOW', 0)
fd = os.open('.env', flags, 0o600)
with os.fdopen(fd, 'a+') as f:
    fcntl.flock(f, fcntl.LOCK_EX)
    os.fchmod(f.fileno(), 0o600)
    f.seek(0)
    text = f.read()
    keys = {}
    for line in text.splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            keys[key] = value
    required = ('RP_DATABASE_PASSWORD', 'RP_OPERATOR_TOKEN', 'RP_VIEWER_TOKEN', 'RP_GRAFANA_PASSWORD')
    if any(key in keys and not keys[key] for key in required):
        raise SystemExit('A required .env value is empty; repair it before startup. Existing values were preserved.')
    missing = [key for key in required if key not in keys]
    f.seek(0, os.SEEK_END)
    if missing and text and not text.endswith('\n'):
        f.write('\n')
    for key in missing:
        f.write(f'{key}={secrets.token_hex(32)}\n')
    print(f'Local credentials ready; {len(missing)} missing value(s) generated; existing values preserved.')
PY
printf '%s\n' 'Credentials remain private in .env. These are local development credentials.'
