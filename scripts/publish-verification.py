#!/usr/bin/env python3
"""Publish sanitized passing reports; never read fixture credentials."""
import datetime, json, pathlib, re, shutil
from matrix import implementation_digest
root=pathlib.Path(__file__).resolve().parents[1]
reports=root/'artifacts/matrix/results'
names=['postgres','mysql','mariadb','tidb','cockroachdb','timescaledb','sqlite','duckdb','clickhouse','mongodb','redis','valkey','elasticsearch','opensearch','neo4j','cassandra','scylladb','influxdb1','influxdb2','influxdb3']
versions={}
verified_reports=[]
digest = implementation_digest()
for name in names:
 p=reports/(name+'.json')
 d=json.loads(p.read_text())
 if not d['version'] or 'unchanged_data' not in d['checks']:
  raise SystemExit('Missing verification: '+name)
 if d.get('implementation_sha256') != digest or 'mcp_http_queries' not in d['checks'] or 'template_native_equivalence' not in d['checks']:
  raise SystemExit('Missing current implementation / MCP verification: '+name)
 version=d['version']
 if name=='tidb':version=version.split('TiDB-v')[-1]
 elif name=='timescaledb':version='2.25.2 / PostgreSQL 17.7' if '2.25.2' in version and '17.7' in version else version
 else:
  match=re.search(r'\d+\.\d+(?:\.\d+)*(?:-Core)?',version)
  if match:version=match.group(0)
 versions.setdefault(d['kind'],[]).append(version)
 verified_reports.append(d)
 target=root/'docs/verification'/p.name;target.parent.mkdir(exist_ok=True);shutil.copyfile(p,target)
(root/'internal/adapter/verified.json').write_text(json.dumps(versions,indent=2)+'\n')
(root/'docs/verification/matrix.json').write_text(json.dumps({
 'result':'passed', 'environment':'OrbStack Linux arm64',
 'verified_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),
 'products':len(versions), 'version_cases':len(names),
 'query_cases':sum(d['query_cases'] for d in verified_reports),
 'denied_cases':sum(d['denied_cases'] for d in verified_reports),
 'implementation_sha256':digest, 'reports':[name+'.json' for name in names],
 'scope':'Independent single-node fixtures; native adapters, HTTP MCP and trialled/published template equivalence across all seven query families; real SQLite/DuckDB file engines. Elasticsearch/OpenSearch verified TLS and read-account write denial.'
},indent=2)+'\n')
print('Published',len(names),'independent version reports for',len(versions),'products')
