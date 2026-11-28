"""Retain fresh raw supporting-image vulnerability gates; never suppress findings."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path

import postgres_rehearsal as pg
from scan_images import SCANNER
from storage_rehearsal import VENDOR


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output-dir', type=Path, required=True)
    args = parser.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=False)
    cache = pg.ROOT / '.infra-state/trivy'
    images = {'postgres-candidate': pg.platform.read_env(pg.STATE / 'rehearsal.env')['POSTGRES_IMAGE'],
              'postgres-legacy': 'postgres:18-alpine@sha256:0115267ab5a509022df9319c77be365cef335adb71556c75f60f5360ee247e71',
              'object-storage': VENDOR}
    version = pg.run(['docker', 'run', '--rm', SCANNER, '--version']).stdout.decode().strip()
    summary = {'scanner': SCANNER, 'version': version, 'date_utc': datetime.now(timezone.utc).isoformat(), 'images': {}}
    for name, reference in images.items():
        metadata = json.loads(pg.run(['docker', 'image', 'inspect', reference]).stdout)[0]
        command = ['docker', 'run', '--rm', '--label', 'ucm.validation=remediation',
                   '--mount', f'type=bind,source={cache},target=/root/.cache/trivy',
                   '--mount', f'type=bind,source={args.output_dir.resolve()},target=/reports',
                   '--mount', 'type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock', SCANNER,
                   'image', '--no-progress', '--timeout', '5m', '--scanners', 'vuln', '--severity', 'HIGH,CRITICAL',
                   '--exit-code', '1', '--format', 'json', '--output', '/reports/' + name + '.json', reference]
        result = pg.run(command, okay=True)
        (args.output_dir / (name + '.log')).write_bytes(result.stderr)
        report = args.output_dir / (name + '.json')
        if not report.exists() or result.returncode not in (0, 1):
            raise RuntimeError('Scanner execution failed: ' + name)
        data = json.loads(report.read_text())
        if data['Metadata']['ImageID'] != metadata['Id']:
            raise RuntimeError('Scanned image identity mismatch')
        findings = [v for group in data.get('Results', []) for v in group.get('Vulnerabilities', [])]
        row = {'reference': reference, 'id': metadata['Id'], 'digests': metadata.get('RepoDigests', []),
               'raw_exit_code': result.returncode, 'high': sum(v['Severity']=='HIGH' for v in findings),
               'critical': sum(v['Severity']=='CRITICAL' for v in findings),
               'findings': [{k:v.get(k) for k in ('VulnerabilityID','Severity','PkgName','InstalledVersion','FixedVersion')} for v in findings]}
        summary['images'][name] = row
        print(f"{name}: raw gate exit={result.returncode}; HIGH={row['high']} CRITICAL={row['critical']}", flush=True)
    (args.output_dir / 'summary.json').write_text(json.dumps(summary, indent=2))
    if any(v['raw_exit_code'] for v in summary['images'].values()):
        raise SystemExit(1)


if __name__ == '__main__': main()
