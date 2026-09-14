#!/usr/bin/env python3
"""Verify one shared Customer/Order ontology against isolated PostgreSQL and MongoDB."""
import argparse
import datetime
import json
import os
from matrix import ROOT, OUT, docker, provision, seed, implementation_digest

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--reuse', action='store_true', help='Reuse existing matrix-owned PostgreSQL/MongoDB fixtures')
parser.add_argument('--keep', action='store_true', help='Keep newly created fixtures for UI or package verification')
args = parser.parse_args()
created = []
try:
    fixtures = []
    images = {}
    for kind in ('postgres', 'mongodb'):
        name = 'mcpdbhub-it-' + kind
        manifest = OUT / (kind + '-fixture.json')
        if args.reuse:
            label = docker('inspect', '--format', '{{index .Config.Labels "com.mcpdbhub.fixture"}}', name).stdout.strip()
            if label != 'true':
                raise RuntimeError('Refusing a fixture not owned by this project: ' + name)
            fixture = json.loads(manifest.read_text())[0]
            if fixture['source'].get('host') != name:
                raise RuntimeError('Fixture manifest host does not match ' + name)
        else:
            provision(kind, name)
            created.append(name)
            fixture = seed(kind, name)
            manifest.write_text(json.dumps([fixture], indent=2) + '\n')
        fixtures.append(fixture)
        images[kind] = docker('inspect', '--format', '{{.Image}}', name).stdout.strip()
    docker('exec', '-i', 'mcpdbhub-it-postgres', 'psql', '-U', 'postgres', '-d', 'hubtest', '-v', 'ON_ERROR_STOP=1', data="""
CREATE TABLE IF NOT EXISTS crm_customers(customer_key TEXT PRIMARY KEY,full_name TEXT);
CREATE TABLE IF NOT EXISTS sales_orders(order_key TEXT PRIMARY KEY,customer_key TEXT REFERENCES crm_customers(customer_key),total_decimal DECIMAL(20,2));
INSERT INTO crm_customers VALUES('C-100','Ada 客户') ON CONFLICT DO NOTHING;
INSERT INTO sales_orders VALUES('O-100','C-100',12.50),('O-101','C-100',20.25) ON CONFLICT DO NOTHING;
GRANT SELECT ON crm_customers,sales_orders TO reader;
""")
    docker('exec', 'mcpdbhub-it-mongodb', 'mongosh', '--quiet', '-u', 'admin', '-p', 'fixture-admin-only', '--authenticationDatabase', 'admin', '--eval', """
db=db.getSiblingDB('hubtest');
db.buyers.updateOne({_id:'C-100'},{$set:{profile:{displayName:'Ada 客户'}}},{upsert:true});
db.purchases.updateOne({_id:'O-100'},{$set:{buyer:'C-100',amount:Decimal128('12.50')}},{upsert:true});
db.purchases.updateOne({_id:'O-101'},{$set:{buyer:'C-100',amount:Decimal128('20.25')}},{upsert:true});
db.buyers.createIndex({'profile.displayName':1}); db.purchases.createIndex({buyer:1});
""")
    (OUT / 'ontology-fixtures.json').write_text(json.dumps(fixtures, indent=2) + '\n')
    digest = implementation_digest()
    environment = ["-e", "MCPDBHUB_TEST_DATABASE_URL="+os.environ["MCPDBHUB_TEST_DATABASE_URL"]] if os.getenv("MCPDBHUB_TEST_DATABASE_URL") else []
    result = docker('exec', *environment, '-w', '/work', '-e', 'MCPDBHUB_ONTOLOGY_FIXTURES=/work/artifacts/matrix/ontology-fixtures.json',
                    'mcpdbhub-dev', 'go', 'test', './internal/server', './internal/engine', '-run', 'Ontology', '-count=1', '-v', check=False)
    (OUT / 'ontology.log').write_text(result.stdout + result.stderr)
    print(result.stdout + result.stderr)
    if result.returncode or digest != implementation_digest():
        raise RuntimeError('Ontology acceptance failed or implementation changed during verification')
    report = {
        'result': 'passed', 'environment': 'OrbStack Linux arm64',
        'verified_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'implementation_sha256': digest, 'images': images,
        'checks': ['shared_customer_order_postgres_mongodb', 'native_join_and_aggregation_equivalence',
                   'exact_decimal_results', 'empty_results', 'parameter_injection_rejected',
                   'mapped_subset_and_identity_key_isolation', 'draft_isolation', 'agent_grants',
                   'immutable_versions_and_explicit_adoption', 'optimistic_conflicts',
                   'referenced_version_deletion_protection', 'archived_binding_protection',
                   'declaration_and_discovery_distinction', 'definition_validation',
                   'encrypted_restart_recovery', 'request_start_ontology_context',
                   'semantic_and_native_cursor_invalidation', 'ontology_audit_correlation'],
        'scope': 'Declarations and metadata only; no claim of full database constraint validation or OWL/SHACL conformance.'
    }
    path = ROOT / 'docs/verification/ontology.json'
    path.write_text(json.dumps(report, indent=2) + '\n')
finally:
    if not args.keep:
        for name in created:
            docker('rm', '-f', name, check=False)
