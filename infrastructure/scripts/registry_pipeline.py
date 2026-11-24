"""Same-run OCI publication: no builds, deployments, tag overwrites or fake digests."""
import argparse
from datetime import datetime, timezone, date
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parents[2]
SERVICES = {'auth': 'security-and-authentication', 'user': 'user', 'course': 'course',
            'enrollment': 'enrollment', 'assessment': 'assessment', 'analytics': 'analytics'}
INVENTORY = {s: {'context': 'services/' + directory, 'file': 'services/' + directory + '/Dockerfile'}
             for s, directory in SERVICES.items()}
INVENTORY.update(frontend={'context': 'frontend', 'file': 'frontend/Dockerfile'},
                 gateway={'context': 'infrastructure/nginx', 'file': 'infrastructure/nginx/Dockerfile'},
                 **{'assessment-minio-init': {'context': 'services/assessment', 'file': 'services/assessment/MinIOInit.Dockerfile'}})
SCANNER = 'aquasec/trivy:0.76.2@sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb'
DIGEST = re.compile(r'sha256:[0-9a-f]{64}\Z')
SHA = re.compile(r'[0-9a-f]{40}\Z')


def digest(value):
    if not isinstance(value, str) or not DIGEST.fullmatch(value) or value == 'sha256:' + '0' * 64:
        raise ValueError('Invalid/placeholder digest')
    return value


def read(path):
    return json.loads(Path(path).read_text(encoding='utf-8'))


def write(path, value):
    with Path(path).open('x', encoding='utf-8') as stream:
        json.dump(value, stream, indent=2)


def clean_scan(report):
    if report.get('SchemaVersion') != 2 or not isinstance(report.get('Metadata'), dict) or not report.get('Results'):
        raise ValueError('Missing/invalid scanner result')
    for result in report['Results']:
        for finding in result.get('Vulnerabilities') or []:
            if finding['Severity'] in ('HIGH', 'CRITICAL'):
                raise ValueError('Blocking vulnerability: ' + finding['VulnerabilityID'])


def archive_info(path, service, commit, repository):
    """Read without extracting, verify OCI blobs and BuildKit attestation subjects."""
    with tarfile.open(path) as archive:
        def blob(descriptor):
            value = digest(descriptor['digest'])
            raw = archive.extractfile('blobs/sha256/' + value.split(':')[1]).read()
            if 'sha256:' + hashlib.sha256(raw).hexdigest() != value:
                raise ValueError('OCI blob digest mismatch')
            return json.loads(raw)
        index = json.load(archive.extractfile('index.json'))
        # Buildx OCI exporter may wrap the result index in an OCI-layout index.
        descriptors = index['manifests']
        if len(descriptors) == 1 and descriptors[0]['mediaType'].endswith('image.index.v1+json'):
            index = blob(descriptors[0]); descriptors = index['manifests']
        runtimes = [d for d in descriptors if d.get('platform') == {'architecture': 'amd64', 'os': 'linux'}]
        if len(runtimes) != 1:
            raise ValueError('Exactly one linux/amd64 runtime required')
        runtime = runtimes[0]; manifest = blob(runtime); config = blob(manifest['config'])
        labels = config['config'].get('Labels', {})
        if labels.get('org.opencontainers.image.revision') != commit or labels.get('org.opencontainers.image.source') != 'https://github.com/' + repository:
            raise ValueError('OCI source identity mismatch')
        if labels.get('org.opencontainers.image.title') != service:
            raise ValueError('OCI service mismatch')
        predicates = {}
        for descriptor in descriptors:
            if descriptor.get('annotations', {}).get('vnd.docker.reference.type') != 'attestation-manifest':
                continue
            if descriptor['annotations'].get('vnd.docker.reference.digest') != runtime['digest']:
                raise ValueError('Attestation runtime mismatch')
            for layer in blob(descriptor)['layers']:
                statement = blob(layer)
                if not any(s.get('digest', {}).get('sha256') == runtime['digest'].split(':')[1] for s in statement.get('subject', [])):
                    raise ValueError('Attestation subject mismatch')
                predicates[statement['predicateType']] = layer['digest']
        if 'https://spdx.dev/Document' not in predicates or not any(k.startswith('https://slsa.dev/provenance/') for k in predicates):
            raise ValueError('Missing BuildKit SBOM/provenance')
        return {'runtime_digest': runtime['digest'], 'config_digest': manifest['config']['digest'], 'attestations': predicates,
                'archive_sha256': hashlib.sha256(Path(path).read_bytes()).hexdigest()}


def eligible(policy, env):
    if env.get('GITHUB_EVENT_NAME') not in ('push', 'workflow_dispatch'):
        raise ValueError('Untrusted event cannot publish')
    if env.get('GITHUB_REPOSITORY') != policy['repository'] or env.get('GITHUB_REF') != 'refs/heads/' + policy['default_branch']:
        raise ValueError('Only the configured default branch can publish')
    if env.get('GITHUB_REF_PROTECTED') != 'true':
        raise ValueError('Protected default branch required')
    if not SHA.fullmatch(env.get('GITHUB_SHA', '')):
        raise ValueError('Exact source commit required')
    for gate in ('registry_access_verified', 'phase3_remote_verified', 'storage_release_scope_approved'):
        if policy.get(gate) is not True:
            raise ValueError('Publication prerequisite pending: ' + gate)
    if not policy.get('approval_owner') or not policy.get('review_by') or date.fromisoformat(policy['review_by']) < date.today():
        raise ValueError('Named owner and current review date required')


def run(args, **kwargs):
    result = subprocess.run([str(a) for a in args], capture_output=True, **kwargs)
    if result.returncode:
        raise RuntimeError(str(args[0]) + ' failed; private registry diagnostics withheld')
    return result.stdout


def prepare(directory, service, commit, repository):
    if service not in INVENTORY or not SHA.fullmatch(commit):
        raise ValueError('Unknown image or invalid source commit')
    report = read(directory / 'image-scan.json')
    clean_scan(report)
    info = archive_info(directory / 'image.tar', service, commit, repository)
    if report['Metadata'].get('ImageID') != info['config_digest']:
        raise ValueError('Scanner image identity differs from built OCI config')
    write(directory / 'build.json', dict(info, service=service, commit=commit, repository=repository,
                                       platform='linux/amd64', build_status='success', scan_status='passed'))


def unpack(directory):
    destination = directory / 'scan-layout'
    destination.mkdir(exist_ok=False)
    with tarfile.open(directory / 'image.tar') as archive:
        archive.extractall(destination, filter='data')


def validate_manifest(manifest):
    from jsonschema import Draft202012Validator, FormatChecker
    schema = read(ROOT / 'infrastructure/registry/image-manifest.schema.json')
    Draft202012Validator.check_schema(schema)
    errors = list(Draft202012Validator(schema, format_checker=FormatChecker()).iter_errors(manifest))
    if errors:
        raise ValueError('Publication manifest fails JSON Schema: ' + errors[0].message)
    if manifest.get('schema_version') != 1 or not SHA.fullmatch(manifest['source']['commit']):
        raise ValueError('Invalid source/schema')
    repository = manifest['source']['repository']
    if repository != 'kavindacena99/University-Course-Management-System':
        raise ValueError('Unexpected repository')
    if manifest['source']['ref'] != 'refs/heads/main' or manifest['workflow']['run_attempt'] < 1 or not str(manifest['workflow']['run_id']).isdigit():
        raise ValueError('Invalid workflow identity')
    if not manifest.get('published_at') or not manifest.get('publication_complete'):
        raise ValueError('Incomplete publication')
    datetime.fromisoformat(manifest['published_at'])
    if manifest['workflow'].get('url') != 'https://github.com/' + repository + '/actions/runs/' + str(manifest['workflow']['run_id']):
        raise ValueError('Workflow URL does not match identity')
    images = manifest['images']
    if len(images) != len(INVENTORY) or {r['service'] for r in images} != set(INVENTORY):
        raise ValueError('Missing, duplicate or unexpected images')
    for row in images:
        expected = 'ghcr.io/' + repository.lower() + '/' + row['service']
        if row['image'] != expected or row['tag'] != 'sha-' + manifest['source']['commit']:
            raise ValueError('Image/tag source mismatch')
        digest(row['digest']); digest(row['runtime_digest'])
        if row['platforms'] != ['linux/amd64'] or any(row.get(k) != 'passed' for k in ('scan_status', 'pull_status', 'registry_scan_status')) or row['build_status'] != 'success':
            raise ValueError('Unverified image cannot enter successful manifest')
        if not row.get('attestations') or row.get('signed_attestation') is not False:
            raise ValueError('Attestation evidence required; no signed claim supported')
        if 'https://spdx.dev/Document' not in row['attestations'] or not any(k.startswith('https://slsa.dev/provenance/') for k in row['attestations']):
            raise ValueError('SBOM and provenance required')
        for value in row['attestations'].values(): digest(value)
        for key in ('sbom_reference', 'provenance_reference'):
            if row.get(key) != expected + '@' + row['digest']:
                raise ValueError('Attestation reference differs from verified image')


def publish(directory, policy):
    env = os.environ
    eligible(policy, env)  # Before registry access, credential use or any publication.
    commit, repository = env['GITHUB_SHA'], env['GITHUB_REPOSITORY']
    plans = []
    for service in INVENTORY:
        bundle = directory / ('image-' + service)
        row = read(bundle / 'build.json')
        if row['commit'] != commit or row['repository'] != repository or row['service'] != service:
            raise ValueError('Artifact source mismatch')
        report = read(bundle / 'image-scan.json')
        clean_scan(report)
        info = archive_info(bundle / 'image.tar', service, commit, repository)
        if report['Metadata'].get('ImageID') != info['config_digest']:
            raise ValueError('Scanner/image identity mismatch')
        if any(row.get(k) != v for k, v in info.items()):
            raise ValueError('Artifact integrity mismatch')
        image = 'ghcr.io/' + repository.lower() + '/' + service
        tag = 'sha-' + commit
        # Refuse all existing SHA tags; never silently overwrite or rebuild on rerun.
        result = subprocess.run(['skopeo', 'inspect', '--authfile', env['REGISTRY_AUTH_FILE'], 'docker://' + image + ':' + tag], capture_output=True)
        if result.returncode == 0:
            raise ValueError('Immutable tag already exists: ' + service)
        if not any(code in result.stderr.lower() for code in (b'manifest unknown', b'name unknown')):
            raise RuntimeError('Registry preflight failed; verify repository package access')
        plans.append((service, bundle, row, image, tag))
    rows = []
    for service, bundle, build, image, tag in plans:
        digestfile = bundle / 'published-digest.txt'
        run(['skopeo', 'copy', '--all', '--preserve-digests', '--authfile', env['REGISTRY_AUTH_FILE'],
             '--digestfile', digestfile, 'oci-archive:' + str(bundle / 'image.tar'), 'docker://' + image + ':' + tag])
        published = digest(digestfile.read_text().strip())  # Only the publication operation supplies this.
        immutable = image + '@' + published
        raw = run(['skopeo', 'inspect', '--raw', '--authfile', env['REGISTRY_AUTH_FILE'], 'docker://' + immutable])
        if 'sha256:' + hashlib.sha256(raw).hexdigest() != published:
            raise ValueError('Registry digest mismatch')
        registry_index = json.loads(raw)
        required = {build['runtime_digest'], *[d['digest'] for d in registry_index.get('manifests', []) if d.get('annotations', {}).get('vnd.docker.reference.type') == 'attestation-manifest']}
        if build['runtime_digest'] not in {d['digest'] for d in registry_index.get('manifests', [])} or len(required) < 2:
            raise ValueError('Registry lost runtime/attestation index')
        # Download complete OCI index and attestations and recheck all subjects locally.
        run(['skopeo', 'copy', '--all', '--preserve-digests', '--authfile', env['REGISTRY_AUTH_FILE'], 'docker://' + immutable, 'oci-archive:' + str(bundle / 'pulled.tar')])
        restored = archive_info(bundle / 'pulled.tar', service, commit, repository)
        if restored['attestations'] != build['attestations'] or restored['runtime_digest'] != build['runtime_digest']:
            raise ValueError('Published attestation mismatch')
        run(['docker', 'pull', '--platform', 'linux/amd64', immutable])
        metadata = json.loads(run(['docker', 'image', 'inspect', immutable]))[0]
        if (metadata['Os'], metadata['Architecture']) != ('linux', 'amd64'):
            raise ValueError('Pulled platform mismatch')
        run(['docker', 'run', '--rm', '--env', 'TRIVY_USERNAME', '--env', 'TRIVY_PASSWORD',
             '-v', str(bundle.resolve()) + ':/reports', SCANNER, 'image', '--scanners', 'vuln', '--severity', 'HIGH,CRITICAL',
             '--exit-code', '1', '--format', 'json', '--output', '/reports/registry-scan.json', immutable])
        clean_scan(read(bundle / 'registry-scan.json'))
        if read(bundle / 'registry-scan.json')['Metadata'].get('ImageID') != build['config_digest']:
            raise ValueError('Registry scan did not resolve the built runtime')
        row = dict(service=service, image=image, tag=tag, digest=published, runtime_digest=build['runtime_digest'],
                   platforms=['linux/amd64'], build_status='success', scan_status='passed', registry_scan_status='passed',
                   pull_status='passed', attestations=build['attestations'], signed_attestation=False,
                   sbom_reference=immutable, provenance_reference=immutable)
        rows.append(row)
        write(bundle / 'publication.json', row)  # Retained even if a later image fails.
    manifest = dict(schema_version=1, publication_complete=True,
                    source=dict(repository=repository, commit=commit, ref=env['GITHUB_REF']),
                    workflow=dict(run_id=env['GITHUB_RUN_ID'], run_attempt=int(env['GITHUB_RUN_ATTEMPT']),
                                  url=env['GITHUB_SERVER_URL'] + '/' + repository + '/actions/runs/' + env['GITHUB_RUN_ID']),
                    published_at=datetime.now(timezone.utc).isoformat(), images=rows)
    validate_manifest(manifest)
    write(directory / 'image-manifest.json', manifest)
    # Compatible with existing nine-image validate_release(); generation does not deploy.
    with (directory / 'images.env').open('x', encoding='utf-8') as stream:
        stream.write(''.join(r['service'].upper().replace('-', '_') + '_IMAGE=' + r['image'] + '@' + r['digest'] + '\n' for r in rows))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['prepare', 'publish', 'validate', 'gate', 'unpack'])
    p.add_argument('--directory', type=Path)
    p.add_argument('--service', choices=INVENTORY)
    args = p.parse_args()
    policy = read(ROOT / 'infrastructure/registry/publication-policy.json')
    if args.action == 'gate': eligible(policy, os.environ)
    elif args.action == 'unpack': unpack(args.directory)
    elif args.action == 'prepare': prepare(args.directory, args.service, os.environ['GITHUB_SHA'], os.environ['GITHUB_REPOSITORY'])
    elif args.action == 'validate': validate_manifest(read(args.directory / 'image-manifest.json'))
    else: publish(args.directory, policy)


if __name__ == '__main__': main()
