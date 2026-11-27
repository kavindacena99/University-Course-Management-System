"""Security boundaries and complete-manifest contracts; never contacts a registry."""
from datetime import date, timedelta
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
from unittest.mock import patch
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infrastructure/scripts'))
import registry_pipeline as registry
import platformctl


class PublicationContract(unittest.TestCase):
    def setUp(self):
        self.policy = dict(repository='kavindacena99/University-Course-Management-System', default_branch='main',
                           registry_access_verified=True, phase3_remote_verified=True, storage_release_scope_approved=True,
                           approval_owner='test-owner', review_by=(date.today() + timedelta(days=1)).isoformat())
        self.env = dict(GITHUB_EVENT_NAME='push', GITHUB_REPOSITORY=self.policy['repository'], GITHUB_REF='refs/heads/main',
                        GITHUB_REF_PROTECTED='true', GITHUB_SHA='a' * 40)

    def test_only_protected_default_branch_events_eligible(self):
        registry.eligible(self.policy, self.env)
        for key, value in [('GITHUB_EVENT_NAME', 'pull_request'), ('GITHUB_EVENT_NAME', 'workflow_run'),
                           ('GITHUB_REF', 'refs/heads/feature'), ('GITHUB_REF_PROTECTED', 'false'),
                           ('GITHUB_REPOSITORY', 'attacker/fork'), ('GITHUB_SHA', 'short')]:
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                registry.eligible(self.policy, dict(self.env, **{key:value}))

    def test_all_prerequisites_and_review_are_required(self):
        for key, value in [('registry_access_verified', False), ('phase3_remote_verified', False),
                           ('storage_release_scope_approved', False), ('approval_owner', None), ('review_by', '2020-01-01')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                registry.eligible(dict(self.policy, **{key:value}), self.env)

    def test_registry_bootstrap_requires_the_explicit_mode(self):
        with self.assertRaises(ValueError):
            registry.eligible(dict(self.policy, registry_access_verified=False), self.env)
        registry.eligible(dict(self.policy, registry_access_verified=False,
                               registry_bootstrap_mode='first_verified_application_upload'), self.env)

    def test_repository_policy_currently_blocks_publication(self):
        with self.assertRaises(ValueError):
            registry.eligible(registry.read(ROOT / 'infrastructure/registry/publication-policy.json'), self.env)

    def test_scan_failures_are_not_clean(self):
        good = dict(SchemaVersion=2, Metadata={}, Results=[{'Target':'test', 'Vulnerabilities':[]}])
        registry.clean_scan(good)
        for report in ({}, dict(good, Results=[]), dict(good, Results=[{'Vulnerabilities':[{'Severity':'HIGH', 'VulnerabilityID':'test'}]}]),
                       dict(good, Results=[{'Vulnerabilities':[{'Severity':'CRITICAL', 'VulnerabilityID':'test'}]}])):
            with self.assertRaises(ValueError): registry.clean_scan(report)

    def manifest(self):
        rows=[]
        for service in registry.INVENTORY:
            image='ghcr.io/' + self.policy['repository'].lower() + '/' + service
            rows.append(dict(service=service, image=image, tag='sha-'+'a'*40, digest='sha256:'+'1'*64,
                             runtime_digest='sha256:'+'2'*64, platforms=['linux/amd64'], build_status='success',
                             scan_status='passed', registry_scan_status='passed', pull_status='passed',
                             attestations={'https://spdx.dev/Document':'sha256:'+'3'*64, 'https://slsa.dev/provenance/v0.2':'sha256:'+'4'*64},
                             signed_attestation=False, sbom_reference=image+'@sha256:'+'1'*64, provenance_reference=image+'@sha256:'+'1'*64))
        return dict(schema_version=1, registry_access_verified=True,
                    source=dict(repository=self.policy['repository'], commit='a'*40, ref='refs/heads/main'),
                    workflow=dict(run_id='1', run_attempt=1, url='https://github.com/'+self.policy['repository']+'/actions/runs/1'),
                    published_at='2026-11-24T00:00:00Z', publication_complete=True, images=rows)

    def test_manifest_requires_complete_verified_inventory(self):
        registry.validate_manifest(self.manifest())
        for field, value in [('digest','sha256:'+'0'*64), ('digest','<digest>'), ('scan_status','failed'),
                             ('pull_status','pending'), ('tag','latest'), ('signed_attestation',True), ('attestations',{})]:
            manifest=self.manifest(); manifest['images'][0][field]=value
            with self.subTest(field=field), self.assertRaises(ValueError): registry.validate_manifest(manifest)
        for images in (self.manifest()['images'][:-1], [self.manifest()['images'][0]]*9):
            manifest=self.manifest(); manifest['images']=images
            with self.assertRaises(ValueError): registry.validate_manifest(manifest)
        manifest=self.manifest(); manifest['registry_access_verified']=False
        with self.assertRaises(ValueError): registry.validate_manifest(manifest)

    def test_first_publish_and_identical_rerun_tag_states(self):
        raw=b'{"schemaVersion":2,"manifests":[]}'
        expected='sha256:'+hashlib.sha256(raw).hexdigest()
        image='ghcr.io/example/product/auth'
        with patch.object(registry.subprocess, 'run', return_value=subprocess.CompletedProcess([],1,b'',b'manifest unknown')):
            self.assertIsNone(registry.existing_tag_digest(image,'sha-'+'a'*40,'authfile',expected))
        with patch.object(registry.subprocess, 'run', return_value=subprocess.CompletedProcess([],0,raw,b'')) as command:
            self.assertEqual(registry.existing_tag_digest(image,'sha-'+'a'*40,'authfile',expected), expected)
            self.assertIn('--raw',command.call_args.args[0])

    def test_partial_publication_rerun_reuses_only_matching_tags(self):
        raw=b'{"schemaVersion":2,"manifests":[1]}'
        expected='sha256:'+hashlib.sha256(raw).hexdigest()
        outcomes=[subprocess.CompletedProcess([],0,raw,b''),
                  subprocess.CompletedProcess([],1,b'',b'manifest unknown')]
        with patch.object(registry.subprocess, 'run', side_effect=outcomes):
            states=[registry.existing_tag_digest('ghcr.io/example/product/'+s,'sha-'+'a'*40,'authfile',expected)
                    for s in ('auth','user')]
        self.assertEqual(states,[expected,None])

    def test_conflicting_tag_or_registry_auth_failure_is_not_absence(self):
        expected='sha256:'+hashlib.sha256(b'expected').hexdigest()
        for result, error in [(subprocess.CompletedProcess([],0,b'other',b''),ValueError),
                              (subprocess.CompletedProcess([],1,b'',b'HTTP 403 denied: manifest unknown'),RuntimeError)]:
            with patch.object(registry.subprocess, 'run', return_value=result), self.assertRaises(error):
                registry.existing_tag_digest('ghcr.io/example/product/auth','sha-'+'a'*40,'authfile',expected)

    def test_upload_digest_must_match_scanned_index(self):
        expected='sha256:'+'a'*64
        self.assertEqual(registry.verified_published_digest(expected+'\n',expected),expected)
        for value in ('sha256:'+'b'*64,'<digest>'):
            with self.assertRaises(ValueError): registry.verified_published_digest(value,expected)

    def test_inventory_excludes_supporting_images(self):
        self.assertEqual(len(registry.INVENTORY),9)
        self.assertNotIn('postgres',registry.INVENTORY)
        self.assertNotIn('minio',registry.INVENTORY)
        for definition in registry.INVENTORY.values(): self.assertTrue((ROOT / definition['file']).is_file())

    def test_failed_prerequisite_cannot_contact_registry(self):
        with patch.dict(registry.os.environ, self.env, clear=True), patch.object(registry.subprocess, 'run') as command:
            with self.assertRaises(ValueError):
                registry.publish(Path('nonexistent'), dict(self.policy, storage_release_scope_approved=False))
            command.assert_not_called()

    def test_workflow_permissions_and_dependency_boundaries(self):
        workflow=yaml.load((ROOT/'.github/workflows/ci.yml').read_text(), Loader=yaml.BaseLoader)
        jobs=workflow['jobs']
        self.assertNotIn('workflow_run',workflow['on'])
        self.assertEqual(set(jobs['publish']['needs']), {'backend','frontend','infrastructure','images','publication-prerequisites'})
        for name,job in jobs.items():
            if name!='publish': self.assertNotEqual(job.get('permissions',{}).get('packages'),'write')
        self.assertEqual(jobs['publish']['permissions'], {'contents':'read','packages':'write'})
        condition=jobs['publish']['if']
        for boundary in ('github.ref_protected', 'github.event_name', 'github.event.repository.default_branch'):
            self.assertIn(boundary,condition)
        build=next(s for s in jobs['images']['steps'] if s.get('uses','').startswith('docker/build-push-action@'))
        self.assertEqual(build['with']['push'],'false')
        self.assertNotIn('load',build['with'])
        self.assertIn('type=oci',build['with']['outputs'])
        # Publisher may copy and pull artifacts, but must never build again.
        self.assertFalse(any(s.get('uses','').startswith('docker/build-push-action@') for s in jobs['publish']['steps']))
        self.assertTrue(all(len(s['uses'].split('@')[1])==40 for job in jobs.values() for s in job['steps'] if 'uses' in s))

    def test_manual_entry_reuses_protected_ci_pipeline(self):
        workflow=yaml.load((ROOT/'.github/workflows/publish-images.yml').read_text(), Loader=yaml.BaseLoader)
        self.assertIn('workflow_dispatch', workflow['on'])
        self.assertEqual(len(workflow['jobs']), 1)
        job=workflow['jobs']['validated-publication']
        self.assertEqual(job['uses'], './.github/workflows/ci.yml')
        self.assertNotIn('with', job)
        self.assertNotIn('secrets', job)
        self.assertIn('github.ref_protected', job['if'])
        self.assertIn('github.event.repository.default_branch', job['if'])
        self.assertEqual(job['permissions'], {'contents':'read', 'packages':'write'})

    def test_digest_output_matches_existing_compose_contract(self):
        with tempfile.TemporaryDirectory() as temporary:
            path=Path(temporary)/'images.env'
            rows=self.manifest()['images']
            path.write_text(''.join(row['service'].upper().replace('-','_')+'_IMAGE='+row['image']+'@'+row['digest']+'\n' for row in rows))
            self.assertEqual(platformctl.validate_release(path),path.resolve())
            path.write_text(path.read_text().replace('@sha256:'+'1'*64,':sha-'+'a'*40))
            with self.assertRaises(RuntimeError): platformctl.validate_release(path)


if __name__ == '__main__': unittest.main()
