"""Create and validate immutable semantic release records from verified publication."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import shutil

from jsonschema import Draft202012Validator, FormatChecker
import registry_pipeline

ROOT = Path(__file__).resolve().parents[2]
SCHEMA = ROOT / 'infrastructure/registry/release-manifest.schema.json'
RC = re.compile(r'^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)-rc\.([1-9]\d*)$')
STABLE = re.compile(r'^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')


def read(path):
    return json.loads(Path(path).read_text(encoding='utf-8'))


def validate(manifest, path=None):
    Draft202012Validator(read(SCHEMA), format_checker=FormatChecker()).validate(manifest)
    match = RC.fullmatch(manifest['version']) if manifest['channel'] == 'rc' else STABLE.fullmatch(manifest['version'])
    if not match or manifest['tag'] != 'v' + manifest['version']:
        raise ValueError('Version, tag and channel mismatch')
    if manifest['rc_number'] != (int(match.group(4)) if manifest['channel'] == 'rc' else None):
        raise ValueError('RC number mismatch')
    if manifest['channel'] == 'stable' and manifest['qa_validation']['status'] != 'passed':
        raise ValueError('Stable release requires passed QA')
    if manifest['qa_validation']['status'] == 'passed' and not manifest['qa_validation']['reference']:
        raise ValueError('Passed QA requires evidence reference')
    if manifest['channel'] == 'stable':
        qa_path = (ROOT / manifest['qa_validation']['reference']).resolve()
        if not qa_path.is_relative_to(ROOT / 'release-manifests') or not qa_path.is_file():
            raise ValueError('Stable release requires retained QA acceptance evidence')
        qa = read(qa_path)
        if (qa.get('verdict') != 'passed' or qa.get('source_commit') != manifest['source_commit'] or
                qa.get('images') != {row['service']: row['digest'] for row in manifest['images']}):
            raise ValueError('QA acceptance does not match stable source and digests')
    publication_path = ROOT / manifest['publication']['image_manifest_path']
    if publication_path.resolve() != publication_path or not publication_path.is_relative_to(ROOT / 'release-manifests/phase-4'):
        raise ValueError('Publication path must be an in-repository Phase 4 manifest')
    raw = publication_path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != manifest['publication']['image_manifest_sha256']:
        raise ValueError('Publication manifest checksum mismatch')
    source = json.loads(raw)
    registry_pipeline.validate_manifest(source)
    if not source['publication_complete'] or not source['registry_access_verified']:
        raise ValueError('Publication incomplete')
    if (manifest['repository'] != source['source']['repository'] or
            manifest['source_commit'] != source['source']['commit'] or
            source['source']['ref'] != 'refs/heads/' + manifest['source_branch'] or
            manifest['publication']['workflow_run_id'] != source['workflow']['run_id'] or
            manifest['publication']['workflow_url'] != source['workflow']['url'] or
            manifest['publication']['published_at'] != source['published_at'] or
            manifest['images'] != source['images']):
        raise ValueError('Release identity differs from verified publication')
    if path:
        images_env = Path(path).parent / 'images.env'
        expected = ''.join(r['service'].upper().replace('-', '_') + '_IMAGE=' + r['image'] + '@' + r['digest'] + '\n'
                           for r in source['images'])
        if images_env.read_text(encoding='utf-8').replace('\r\n', '\n') != expected:
            raise ValueError('Release images.env differs from verified publication')
    return source


def create_rc(version, publication, manifest_artifact_url, evidence_artifact_url):
    match = RC.fullmatch(version)
    if not match:
        raise ValueError('Expected MAJOR.MINOR.PATCH-rc.N')
    source = read(publication)
    registry_pipeline.validate_manifest(source)
    directory = ROOT / 'release-manifests' / ('v' + version)
    if directory.exists():
        raise FileExistsError('Release directory already exists; RC overwrite refused')
    relative = Path(publication).resolve().relative_to(ROOT)
    manifest = {
        'schema_version': 1, 'version': version, 'tag': 'v' + version,
        'channel': 'rc', 'rc_number': int(match.group(4)),
        'repository': source['source']['repository'],
        'source_commit': source['source']['commit'], 'source_branch': 'main',
        'publication': {
            'workflow_run_id': source['workflow']['run_id'],
            'workflow_url': source['workflow']['url'],
            'image_manifest_path': relative.as_posix(),
            'image_manifest_sha256': hashlib.sha256(Path(publication).read_bytes()).hexdigest(),
            'manifest_artifact_url': manifest_artifact_url,
            'evidence_artifact_url': evidence_artifact_url,
            'published_at': source['published_at'],
        },
        'images': source['images'],
        'qa_validation': {'status': 'pending', 'reference': None},
        'created_at': datetime.now(timezone.utc).isoformat(),
    }
    validate(manifest)
    directory.mkdir(parents=True)
    (directory / 'release-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    shutil.copyfile(Path(publication).parent / 'images.env', directory / 'images.env')
    validate(manifest, directory / 'release-manifest.json')
    print(directory)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['create-rc', 'validate'])
    parser.add_argument('--version')
    parser.add_argument('--publication', type=Path)
    parser.add_argument('--manifest-artifact-url')
    parser.add_argument('--evidence-artifact-url')
    parser.add_argument('--manifest', type=Path)
    args = parser.parse_args()
    if args.action == 'create-rc':
        create_rc(args.version, args.publication, args.manifest_artifact_url, args.evidence_artifact_url)
    else:
        validate(read(args.manifest), args.manifest)


if __name__ == '__main__':
    main()
