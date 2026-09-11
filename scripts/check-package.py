#!/usr/bin/env python3
"""Verify the built image with disposable configuration and a retained PG fixture."""
import hashlib
import argparse
import http.cookiejar
import json
import os
import pathlib
import re
import select
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--image', default='mcpdbhub:local')
parser.add_argument('--archive', type=pathlib.Path, help='Verify an independently unpacked Linux dist instead of an image')
parser.add_argument('--otlp', action='store_true', help='Verify audit delivery through an isolated OpenTelemetry Collector')
parser.add_argument('--report', type=pathlib.Path, default=ROOT/'docs/verification/package.json')
args = parser.parse_args()
IMAGE = args.image
name = "mcpdbhub-package-" + uuid.uuid4().hex[:10]
volume = name + "-data"


def docker(*args, check=True):
    return subprocess.run(["docker", *args], text=True, capture_output=True, check=check)


def source_digest():
    paths = {p for base in ("cmd", "internal") for p in (ROOT/base).rglob("*.go") if not p.name.endswith("_test.go")}
    paths.update(p for p in (ROOT/"web/src").rglob("*") if p.is_file())
    paths.update(ROOT/p for p in ("go.mod", "go.sum", "web/package-lock.json", "Dockerfile", "internal/adapter/verified.json"))
    digest = hashlib.sha256()
    for path in sorted(paths):
        digest.update(str(path.relative_to(ROOT)).encode()+b"\0"+path.read_bytes())
    return digest.hexdigest()


client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
csrf = ""
base = ""


def request(method, path, body=None, token=None, expected=200):
    headers = {"Host": "127.0.0.1:8080", "Content-Type": "application/json", "X-CSRF-Token": csrf}
    if token:
        headers.update(Authorization="Bearer "+token, Accept="application/json, text/event-stream")
        headers["MCP-Protocol-Version"] = "2025-11-25"
    req = urllib.request.Request(base+path, method=method, headers=headers,
                                 data=json.dumps(body).encode() if body is not None else None)
    try:
        with client.open(req, timeout=15) as response:
            status, data = response.status, response.read().decode()
    except urllib.error.HTTPError as error:
        status, data = error.code, error.read().decode()
    assert status == expected, (path, status, expected)
    if expected != 200:
        return None
    if data.startswith("event:"):
        data = next(line[5:].strip() for line in data.splitlines() if line.startswith("data:"))
    return json.loads(data) if data else None


def ready():
    last_error = None
    for _ in range(40):
        try:
            if request("GET", "/healthz")["status"] == "ok":
                return
        except (OSError, AssertionError) as error:
            last_error = error
        time.sleep(.25)
    raise RuntimeError("package did not become healthy: "+str(last_error))


created = False
unpacked = None
runtime = []
binary = 'mcpdbhub'
manifest = None
collector = None


def export_view():
    return request('GET', '/api/settings/audit-export')


def await_export(predicate):
    for _ in range(120):
        view = export_view()
        if predicate(view['status']):
            return view
        time.sleep(.25)
    raise RuntimeError('Audit export did not reach the expected state: '+str(view['status']))


try:
    assert docker("inspect", "--format", '{{index .Config.Labels "com.mcpdbhub.fixture"}}', "mcpdbhub-it-postgres").stdout.strip() == "true"
    if args.archive:
        unpacked = tempfile.TemporaryDirectory(prefix='mcpdbhub-unpacked-', dir=ROOT/'artifacts')
        with tarfile.open(args.archive) as archive:
            members = archive.getmembers()
            for member in members:
                path = pathlib.PurePosixPath(member.name)
                assert not path.is_absolute() and '..' not in path.parts
                assert member.isfile() or member.isdir(), 'unexpected archive link or device'
            archive.extractall(unpacked.name)
        roots = list(pathlib.Path(unpacked.name).iterdir())
        assert len(roots) == 1
        package = roots[0]
        manifest = json.loads((package/'BUILD.json').read_text())
        assert manifest['arch'] in ('arm64', 'amd64') and manifest['os'] == 'linux'
        assert hashlib.sha256((package/'libexec/mcpdbhub').read_bytes()).hexdigest() == manifest['binary_sha256']
        assert all((package/p).exists() for p in ('README.md', 'README.en.md', 'docs/install.md', 'docs/install.en.md', 'third_party/licenses'))
        assert all((package/p).exists() for p in ('docs/semantics.md', 'docs/semantics.zh-CN.md', 'examples/semantics/sql-postgres.json'))
        IMAGE = 'debian:bookworm-slim'
        binary = '/opt/mcpdbhub/mcpdbhub'
        runtime = ['--platform', 'linux/'+manifest['arch'], '--user', '10001:10001',
                   '-v', str(package)+':/opt/mcpdbhub:ro', '--entrypoint', binary,
                   '-e', 'MCPDBHUB_DATA_DIR=/data', '-e', 'MCPDBHUB_LISTEN=0.0.0.0:8080']
        docker('run', '--rm', '--platform', 'linux/'+manifest['arch'], '-v', volume+':/data', IMAGE, 'chown', '10001:10001', '/data')
    docker("run", "-d", "--name", name, "--label", "com.mcpdbhub.fixture=true", "--network", "mcpdbhub-test",
           "--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m", "--cap-drop", "ALL",
           "--security-opt", "no-new-privileges", "-v", volume+":/data", "-p", "127.0.0.1::8080", *runtime, IMAGE, 'serve')
    created = True
    port = docker("port", name, "8080/tcp").stdout.strip().rsplit(":", 1)[1]
    base = "http://127.0.0.1:"+port
    ready()
    setup = re.search(r"First-time setup token: (\S+)", docker("logs", name).stderr).group(1)
    initial_password = uuid.uuid4().hex
    csrf = request("POST", "/api/setup", {"token": setup, "password": initial_password})["csrf"]
    assert docker("exec", name, "id", "-u").stdout.strip() == "10001"
    assert docker("exec", name, "sh", "-c", "command -v node", check=False).returncode != 0
    version_output = docker('exec', name, binary, 'version').stdout.strip()
    runtime_version = re.fullmatch(r'mcpdbhub (\d+\.\d+\.\d+) \(commit ([^)]+)\)', version_output)
    assert runtime_version, 'missing runtime version'
    if manifest:
        assert version_output == 'mcpdbhub '+manifest['version']+' (commit '+manifest['commit']+')'
    fixture = json.loads((ROOT/"artifacts/matrix/postgres-fixture.json").read_text())[0]["source"]
    fixture.update(name="Package PostgreSQL fixture", enabled=True)
    source = request("POST", "/api/sources", fixture)
    agent = request("POST", "/api/agents", {"name": "Package reader", "sources": [source["id"]], "enabled": True})
    if args.otlp:
        assert export_view()['config']['enabled'] is False
        collector = name+'-collector'
        docker('run', '-d', '--name', collector, '--network', 'mcpdbhub-test', '--cpus', '1', '--memory', '256m',
               '-v', str(ROOT/'examples/otel-collector.yaml')+':/etc/otelcol/config.yaml:ro',
               'otel/opentelemetry-collector:0.160.0', '--config=/etc/otelcol/config.yaml')
        for _ in range(40):
            logs = docker('logs', collector)
            if 'Everything is ready' in logs.stdout+logs.stderr:
                break
            time.sleep(.25)
        else:
            raise RuntimeError('Collector did not become ready')
        config = export_view()['config']
        config.update(enabled=True, endpoint='http://'+collector+':4318/v1/logs', headers={'Authorization': 'Bearer package-test-only'})
        assert request('POST', '/api/settings/audit-export/test', config)['accepted'] is True
        saved = request('PUT', '/api/settings/audit-export', config)
        assert saved['headers_configured'] is True and 'package-test-only' not in json.dumps(saved)
    query = {"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "query_sql", "arguments": {"source_id": source["id"], "query": "SELECT count(*) FROM events"}}}
    result = request("POST", "/mcp", query, agent["token"])
    assert result["result"]["structuredContent"]["data"] == [["3"]], result
    if args.otlp:
        config = await_export(lambda status: status['accepted'] == 1 and status['pending'] == 0)['config']
        config.update(protocol='grpc', endpoint='http://'+collector+':4317')
        assert request('POST', '/api/settings/audit-export/test', config)['accepted'] is True
        request('PUT', '/api/settings/audit-export', config)
        assert request('POST', '/mcp', query, agent['token'])['result']['structuredContent']['data'] == [['3']]
        await_export(lambda status: status['accepted'] == 2 and status['pending'] == 0)
        docker('stop', collector)
        assert request('POST', '/mcp', query, agent['token'])['result']['structuredContent']['data'] == [['3']]
        await_export(lambda status: status['state'] == 'retrying' and status['pending'] == 1)
    bridge = subprocess.Popen(['docker', 'exec', '-i', '-e', 'MCPDBHUB_TOKEN', name, binary,
                               'stdio', '--url', 'http://127.0.0.1:8080/mcp'],
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              text=True, env={**os.environ, 'MCPDBHUB_TOKEN': agent['token']})
    def bridge_call(body):
        bridge.stdin.write(json.dumps(body)+'\n')
        bridge.stdin.flush()
        assert select.select([bridge.stdout], [], [], 20)[0], 'stdio response timed out'
        return json.loads(bridge.stdout.readline())
    try:
        initialized = bridge_call({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {
            'protocolVersion': '2025-11-25', 'capabilities': {}, 'clientInfo': {'name': 'dist-verification', 'version': '1'}}})
        assert initialized['result']['serverInfo']['version'] == runtime_version.group(1)
        bridge.stdin.write(json.dumps({'jsonrpc': '2.0', 'method': 'notifications/initialized'})+'\n')
        tools = bridge_call({'jsonrpc': '2.0', 'id': 2, 'method': 'tools/list'})
        assert len(tools['result']['tools']) == 14
        assert bridge_call({**query, 'id': 3})['result']['structuredContent']['data'] == [['3']]
    finally:
        bridge.stdin.close()
        try:
            bridge.wait(timeout=10)
        except subprocess.TimeoutExpired:
            bridge.kill()
            bridge.wait()
        bridge.stdout.close()
        bridge.stderr.close()
    docker("restart", name)
    # Docker may reassign an automatically published host port on restart.
    port = docker("port", name, "8080/tcp").stdout.strip().rsplit(":", 1)[1]
    base = "http://127.0.0.1:"+port
    ready()
    assert any(s["id"] == source["id"] for s in request("GET", "/api/sources"))
    assert request("POST", "/mcp", query, agent["token"])["result"]["structuredContent"]["data"] == [["3"]]
    if args.otlp:
        saved = export_view()
        assert saved['config']['protocol'] == 'grpc' and saved['headers_configured'] is True
        assert saved['status']['pending'] == 3 and saved['status']['accepted'] == 2
        docker('start', collector)
        await_export(lambda status: status['pending'] == 0 and status['accepted'] == 5)
        logs = docker('logs', collector)
        wire = logs.stdout+logs.stderr
        assert source['id'] in wire and 'event.name: Str(mcpdbhub.audit)' in wire
        assert 'package-test-only' not in wire and 'SELECT count(*)' not in wire and agent['token'] not in wire
    semantic_path = '/api/sources/'+source['id']+'/semantics'
    snapshot = {'format_version': 1, 'overview': 'Release package semantics', 'entries': [{
        'id': 'event-count', 'kind': 'template', 'name': 'Event count', 'template': {
            'enabled': True, 'tool': 'query_sql',
            'query_json': json.dumps({'query': 'SELECT count(*) FROM events WHERE id >= $1', 'params': [0]}),
            'parameters': [{'name': 'minimum_id', 'type': 'integer', 'required': True, 'pointers': ['/params/0'], 'minimum': '1'}],
            'example_json': '{"minimum_id":1}'}}]}
    draft = request('PUT', semantic_path, {'revision': '0', 'snapshot': snapshot})
    request('POST', semantic_path+'/publish', {'revision': draft['revision']}, expected=400)
    request('POST', semantic_path+'/trial', {'revision': draft['revision'], 'template_id': 'event-count'})
    published = request('POST', semantic_path+'/publish', {'revision': draft['revision']})
    source = next(s for s in request('GET', '/api/sources') if s['id'] == source['id'])
    source['query_access_mode'] = 'templates_only'
    request('PUT', '/api/sources/'+source['id'], source)
    native_query = query
    assert request('POST', '/mcp', native_query, agent['token'])['result']['isError'] is True
    query = {'jsonrpc': '2.0', 'id': 10, 'method': 'tools/call', 'params': {
        'name': 'execute_query_template', 'arguments': {'source_id': source['id'], 'template_id': 'event-count',
        'execution_version': published['published_version'], 'parameters': {'minimum_id': 1}}}}
    content = request('POST', '/mcp', query, agent['token'])['result']['structuredContent']
    assert content['data'] == [['3']] and content['template_id'] == 'event-count' and content['template_version'] == '1'
    bridge = subprocess.Popen(['docker', 'exec', '-i', '-e', 'MCPDBHUB_TOKEN', name, binary,
                               'stdio', '--url', 'http://127.0.0.1:8080/mcp'],
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              text=True, env={**os.environ, 'MCPDBHUB_TOKEN': agent['token']})
    try:
        bridge_call({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {
            'protocolVersion': '2025-11-25', 'capabilities': {}, 'clientInfo': {'name': 'template-dist-verification', 'version': '1'}}})
        bridge.stdin.write(json.dumps({'jsonrpc': '2.0', 'method': 'notifications/initialized'})+'\n')
        assert bridge_call({**native_query, 'id': 2})['result']['isError'] is True
        assert bridge_call({**query, 'id': 3})['result']['structuredContent']['data'] == [['3']]
    finally:
        bridge.stdin.close()
        try:
            bridge.wait(timeout=10)
        except subprocess.TimeoutExpired:
            bridge.kill()
            bridge.wait()
        bridge.stdout.close()
        bridge.stderr.close()
    if args.otlp:
        await_export(lambda status: status['pending'] == 0 and status['accepted'] >= 10)
        logs = docker('logs', collector)
        wire = logs.stdout+logs.stderr
        assert 'mcpdbhub.audit.template_id: Str(event-count)' in wire
        assert 'mcpdbhub.audit.template_version: Str(1)' in wire
        assert 'minimum_id' not in wire and 'SELECT count(*)' not in wire
    docker("stop", name)
    replacement_password = uuid.uuid4().hex
    recovery = subprocess.run(["docker", "run", "--rm", "-i", "--read-only", "--cap-drop", "ALL",
                               "-v", volume+":/data", *runtime, IMAGE, "reset-password", "--password-stdin"],
                              input=replacement_password, text=True, capture_output=True, check=True)
    assert replacement_password not in recovery.stdout + recovery.stderr
    docker("start", name)
    port = docker("port", name, "8080/tcp").stdout.strip().rsplit(":", 1)[1]
    base = "http://127.0.0.1:"+port
    ready()
    request("GET", "/api/sources", expected=401)
    request("POST", "/api/login", {"password": initial_password}, expected=401)
    csrf = request("POST", "/api/login", {"password": replacement_password})["csrf"]
    assert any(s["id"] == source["id"] for s in request("GET", "/api/sources"))
    assert request("POST", "/mcp", query, agent["token"])["result"]["structuredContent"]["data"] == [["3"]]
    assert request('POST', '/mcp', native_query, agent['token'])['result']['isError'] is True
    request("DELETE", "/api/agents/"+agent["agent"]["id"])
    request("POST", "/mcp", query, agent["token"], expected=401)
    image = json.loads(docker("image", "inspect", IMAGE).stdout)[0]
    result = {"result": "passed", "image_id": image["Id"], "architecture": image["Architecture"],
              "version": runtime_version.group(1),
              "source_sha256": source_digest(), "verified_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "checks": ["nonroot_uid_10001", "readonly_root_filesystem", "no_node_runtime", "bootstrap",
                         "source_persistence", "HTTP_MCP_query", "stdio_initialize_discovery_and_query", "restart_session_agent_and_source_recovery", "revoked_agent_denied",
                         "password_recovery_cli", "old_password_and_admin_sessions_rejected", "recovery_preserves_database_credentials_and_agent_token"]}
    result['checks'].extend(['template_trial_required_for_publication', 'HTTP_and_stdio_template_query',
                             'HTTP_and_stdio_templates_only_native_denial', 'published_template_and_evidence_restart_recovery'])
    if args.otlp:
        result['collector_version'] = '0.160.0'
        result['checks'].extend(['OTLP_HTTP_protobuf', 'OTLP_gRPC', 'OTLP_header_and_payload_redaction',
                                 'OTLP_receiver_outage_query_isolation', 'OTLP_pending_restart_and_recovery'])
        result['checks'].append('OTLP_template_correlation_and_redaction')
    if manifest:
        result.update(architecture=manifest['arch'], commit=manifest['commit'], image_id=manifest['image_id'],
                      binary_sha256=manifest['binary_sha256'], archive=args.archive.name,
                      archive_sha256=hashlib.sha256(args.archive.read_bytes()).hexdigest())
        result['checks'].extend(['independent_archive_extraction', 'private_runtime_libraries', 'version_matches_source', 'bilingual_installation_guides'])
        result['checks'].append('bilingual_semantics_guides_and_examples')
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(result, indent=2)+"\n")
    print(json.dumps(result))
except Exception:
    if created:
        logs = docker("logs", name, check=False)
        safe = re.sub(r"First-time setup token: \S+", "First-time setup token: [redacted]", logs.stdout+logs.stderr)
        (ROOT/"artifacts/matrix/package-server.log").write_text(safe)
    raise
finally:
    if created:
        docker("rm", "-f", name, check=False)
    if collector:
        docker('rm', '-f', collector, check=False)
    docker("volume", "rm", volume, check=False)
    if unpacked:
        unpacked.cleanup()
