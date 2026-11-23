"""Run the strict CI vulnerability gate locally and retain concise JSON evidence."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess
import sys

import platformctl as local

SCANNER = 'aquasec/trivy:0.76.2@sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tag', default='phase345')
    parser.add_argument('--skip-db-update', action='store_true')
    args = parser.parse_args()
    cache = local.ROOT / '.infra-state/trivy'
    reports = local.ROOT / '.infra-state/security'
    reports.mkdir(parents=True, exist_ok=True)
    summary = {}
    run_id = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    for service in local.IMAGES:
        output = service + '-' + run_id + '-vulnerabilities.json'
        image = f'ucm-{service}:{args.tag}'
        command = ['docker', 'run', '--rm', '--mount',
                   f'type=bind,source={cache},target=/root/.cache/trivy', '--mount',
                   f'type=bind,source={reports},target=/reports', '--mount',
                   'type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock',
                   SCANNER, 'image', '--no-progress', '--timeout', '5m', '--scanners', 'vuln',
                   '--severity', 'HIGH,CRITICAL', '--exit-code', '1', '--format', 'json',
                   '--output', '/reports/' + output]
        if args.skip_db_update:
            command.append('--skip-db-update')
        result = subprocess.run(command + [image], capture_output=True, text=True)
        path = reports / output
        if result.returncode not in (0, 1) or not path.exists():
            summary[service] = {'status': 'scanner-error', 'exit_code': result.returncode}
        else:
            data = json.loads(path.read_text())
            found = [item for group in data.get('Results', []) for item in group.get('Vulnerabilities', [])]
            summary[service] = {
                'status': 'failed-vulnerability-gate' if found else 'passed',
                'report': output,
                'high': sum(v['Severity'] == 'HIGH' for v in found),
                'critical': sum(v['Severity'] == 'CRITICAL' for v in found),
                'unique_cves': sorted({v['VulnerabilityID'] for v in found}),
                'remediation': sorted({(v['PkgName'], v.get('FixedVersion', 'unavailable')) for v in found}),
            }
        print(f'{service}: {summary[service]["status"]}; HIGH={summary[service].get("high", "unknown")} CRITICAL={summary[service].get("critical", "unknown")}', flush=True)
    (reports / 'image-scan-summary.json').write_text(json.dumps(summary, indent=2), encoding='utf-8')
    if any(v['status'] != 'passed' for v in summary.values()):
        raise SystemExit(1)


if __name__ == '__main__':
    main()
