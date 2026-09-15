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
parser.add_argument('--image', default='contextgate:local')
parser.add_argument('--archive', type=pathlib.Path, help='Verify an independently unpacked Linux dist instead of an image')
parser.add_argument('--otlp', action='store_true', help='Verify audit delivery through an isolated OpenTelemetry Collector')
parser.add_argument('--report', type=pathlib.Path, default=ROOT/'docs/verification/package.json')
args = parser.parse_args()
IMAGE = args.image
name = "mcpdbhub-package-" + uuid.uuid4().hex[:10]
volume = name + "-data"


def docker(*args, check=True):
    result = subprocess.run(["docker", *args], text=True, capture_output=True)
    if check and result.returncode:
        raise RuntimeError("Docker " + str(args[0]) + " failed: " + result.stderr[-2000:])
    return result


def source_digest():
    paths = {p for base in ("cmd", "internal") for p in (ROOT/base).rglob("*") if p.suffix in (".go", ".sql") and not p.name.endswith("_test.go")}
    paths.update(p for p in (ROOT/"web/src").rglob("*") if p.is_file())
    paths.update(ROOT/p for p in ("go.mod", "go.sum", "web/package-lock.json", "Dockerfile", "internal/adapter/verified.json", "LICENSE", "NOTICE"))
    digest = hashlib.sha256()
    for path in sorted(paths):
        digest.update(str(path.relative_to(ROOT)).encode()+b"\0"+path.read_bytes())
    return digest.hexdigest()


client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
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
binary = 'contextgate'
manifest = None
collector = None
metadata = None
http_fixture = None


def export_view():
    return request('GET', '/api/settings/audit-export')


def await_export(predicate):
    for _ in range(120):
        view = export_view()
        if predicate(view['status']):
            return view
        time.sleep(.25)
    raise RuntimeError('Audit export did not reach the expected state: '+str(view['status']))


def check_administrator_otlp(source_id, owner_configuration):
    global client, csrf
    owner_client, owner_csrf = client, csrf
    owner = request('GET', '/api/session')['administrator']
    colleague = request('POST', '/api/administrators', {
        'username': 'package-operator', 'display_name': 'Package operator', 'role': 'admin'})
    password = uuid.uuid4().hex
    try:
        client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        csrf = request('POST', '/api/login', {'username': 'package-operator', 'password': colleague['temporary_password']})['csrf']
        request('GET', '/api/sources', expected=403)
        request('POST', '/api/password', {'current_password': colleague['temporary_password'], 'password': password})
        personal = request('POST', '/api/configuration-agents', {})
        colleague_client, colleague_csrf = client, csrf
        for protocol, port, suffix in [('http/protobuf', 4318, '/v1/logs'), ('grpc', 4317, '')]:
            client, csrf = owner_client, owner_csrf
            config = export_view()['config']
            config.update(protocol=protocol, endpoint='http://'+collector+':'+str(port)+suffix)
            request('PUT', '/api/settings/audit-export', config)
            expected_records = []
            for actor, browser, session_csrf, identity in [
                (owner, owner_client, owner_csrf, owner_configuration),
                (colleague['administrator'], colleague_client, colleague_csrf, personal),
            ]:
                client, csrf = browser, session_csrf
                request('POST', '/api/sources/'+source_id+'/test', {})
                reply = request('POST', '/mcp/config', {'jsonrpc': '2.0', 'id': 900, 'method': 'tools/call',
                    'params': {'name': 'get_configuration_guide', 'arguments': {}}}, identity['token'])['result']
                assert not reply.get('isError')
                for channel in ['ui', 'configuration_mcp']:
                    events = request('GET', '/api/audit?administrator_id='+actor['id']+'&channel='+channel)
                    event = next(e for e in events if not e.get('error_code'))
                    expected_records.append((actor, channel, identity['id'], event['request_id']))
            client, csrf = owner_client, owner_csrf
            await_export(lambda status: status['pending'] == 0)
            logs = docker('logs', collector)
            wire = logs.stdout+logs.stderr
            records = re.split(r'LogRecord #\d+', wire)
            for actor, channel, identity_id, request_id in expected_records:
                attributes = [
                    'mcpdbhub.audit.request_id: Str('+request_id+')',
                    'mcpdbhub.audit.administrator_id: Str('+actor['id']+')',
                    'mcpdbhub.audit.administrator_username: Str('+actor['username']+')',
                    'mcpdbhub.audit.channel: Str('+channel+')',
                ]
                if channel == 'configuration_mcp':
                    attributes.append('mcpdbhub.audit.configuration_agent_id: Str('+identity_id+')')
                assert any(all(attribute in record for attribute in attributes) for record in records), 'OTLP lost per-record administrator attribution: '+protocol
            for secret in [password, colleague['temporary_password'], personal['token'], owner_configuration['token']]:
                assert secret not in wire, 'OTLP exposed an administrator credential'
    finally:
        client, csrf = owner_client, owner_csrf


def check_http_workflow():
    global http_fixture
    examples = (package if args.archive else ROOT)/'examples/http-api'
    http_fixture = name+'-http'
    docker('run', '-d', '--name', http_fixture, '--network', 'mcpdbhub-test',
           '--read-only', '--cap-drop', 'ALL', '--memory', '128m', '--cpus', '1',
           '-v', str(examples)+':/example:ro', 'python:3.13-alpine', 'python', '/example/server.py')
    for _ in range(40):
        if docker('exec', http_fixture, 'python', '-c', "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/customers', timeout=1).read()", check=False).returncode == 0:
            break
        time.sleep(.25)
    else:
        raise RuntimeError('HTTP API fixture did not become ready')
    config = json.loads((examples/'source.json').read_text())
    config['http_api']['base_url'] = 'http://'+http_fixture+':8080'
    api_source = request('POST', '/api/sources', config)
    probe = request('POST', '/api/sources/'+api_source['id']+'/test', {})
    assert probe['connected'] and probe['permission_status'] == 'unverified'
    reader = request('POST', '/api/agents', {'name': 'Package API reader', 'sources': [api_source['id']], 'enabled': True})
    def call(tool, arguments, error=False):
        reply = request('POST', '/mcp', {'jsonrpc': '2.0', 'id': 150, 'method': 'tools/call',
                        'params': {'name': tool, 'arguments': arguments}}, reader['token'])['result']
        assert bool(reply.get('isError')) == error, 'Unexpected HTTP API tool status: '+tool
        return reply.get('structuredContent')
    native = call('query_http_api', {'source_id': api_source['id'], 'operation': 'list_customers', 'named_params': {'region': 'east'}})
    post = call('query_http_api', {'source_id': api_source['id'], 'operation': 'search_customers', 'named_params': {'region': 'east'}})
    assert post['data'] == native['data'] and native['data'][0]['id'] == '9007199254740993'
    page = call('query_http_api', {'source_id': api_source['id'], 'operation': 'list_customers', 'named_params': {'region': 'east'}, 'cursor': native['next_cursor']})
    assert page['data'][0]['name'] == 'Northwind' and not page.get('next_cursor')
    path = '/api/sources/'+api_source['id']+'/semantics'
    snapshot = json.loads((examples/'semantics.json').read_text())
    snapshot['entries'][1]['template']['example_json'] = '{"region":"west"}'
    draft = request('PUT', path, {'revision': '0', 'snapshot': snapshot})
    request('POST', path+'/publish', {'revision': draft['revision']}, expected=400)
    slots = request('POST', path+'/bindings', {'query_json': snapshot['entries'][1]['template']['query_json']})
    assert slots['slots'] == ['/named_params/region']
    request('POST', path+'/trial', {'revision': draft['revision'], 'template_id': 'customers-by-region'})
    request('POST', path+'/publish', {'revision': draft['revision']})
    api_source = next(s for s in request('GET', '/api/sources') if s['id'] == api_source['id'])
    api_source['query_access_mode'] = 'templates_only'
    request('PUT', '/api/sources/'+api_source['id'], api_source)
    call('query_http_api', {'source_id': api_source['id'], 'operation': 'list_customers', 'named_params': {'region': 'east'}}, error=True)
    catalog = request('GET', '/api/business-catalog?view=queries&source_id='+api_source['id']+'&agent_id='+reader['agent']['id'])
    assert len(catalog['entries']) == 1 and catalog['entries'][0]['id'] == 'customers-by-region'
    evaluation_path = '/api/sources/'+api_source['id']+'/evaluation/history'
    evaluation = request('POST', evaluation_path, {'mode': 'single', 'kind': 'guided', 'name': 'API customer check',
                         'question': 'List east-region customers.', 'criteria': 'First page contains Acme with its exact integer ID.', 'agent_id': reader['agent']['id']})
    actual = call('execute_query_template', {'source_id': api_source['id'], 'template_id': 'customers-by-region', 'execution_version': '1', 'parameters': {'region': 'east'}})
    assert actual['data'] == native['data']
    evaluation_path += '/'+evaluation['id']
    evaluation = request('POST', evaluation_path+'/capture', {'revision': evaluation['revision'], 'kind': 'guided', 'action': 'collect'})
    assert evaluation['runs']['guided']['stats']['successful_queries'] == 1
    request('PUT', evaluation_path+'/review', {'revision': evaluation['revision'], 'reviews': {'guided': {'verdict': 'correct', 'notes': 'Exact fixture result verified.'}}})
    summary = request('GET', '/api/sources/'+api_source['id']+'/evaluation/history')['summary']
    assert summary['completed_singles'] == 1 and summary['single_correct'] == 1 and summary['completed_pairs'] == 0


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
        assert hashlib.sha256((package/'libexec/contextgate').read_bytes()).hexdigest() == manifest['binary_sha256']
        assert all((package/p).exists() for p in ('README.md', 'README.en.md', 'docs/install.md', 'docs/install.en.md', 'third_party/licenses'))
        assert all((package/p).exists() for p in ('docs/semantics.md', 'docs/semantics.zh-CN.md', 'examples/semantics/sql-postgres.json', 'docs/ontologies.md', 'docs/ontologies.zh-CN.md', 'examples/ontologies/commerce.json'))
        IMAGE = 'debian:bookworm-slim'
        binary = '/opt/contextgate/contextgate'
        runtime = ['--platform', 'linux/'+manifest['arch'], '--user', '10001:10001',
                   '-v', str(package)+':/opt/contextgate:ro', '--entrypoint', binary,
                   '-e', 'MCPDBHUB_DATA_DIR=/data', '-e', 'MCPDBHUB_LISTEN=0.0.0.0:8080']
        docker('run', '--rm', '--platform', 'linux/'+manifest['arch'], '-v', volume+':/data', IMAGE, 'chown', '10001:10001', '/data')
    metadata_name = name+'-postgres'
    metadata_password = uuid.uuid4().hex
    docker('run', '-d', '--name', metadata_name, '--network', 'mcpdbhub-test', '--cpus', '1', '--memory', '512m',
           '-e', 'POSTGRES_USER=mcpdbhub', '-e', 'POSTGRES_PASSWORD='+metadata_password,
           '-e', 'POSTGRES_DB=mcpdbhub', 'postgres:17.11')
    metadata = metadata_name
    for _ in range(80):
        if docker('exec', metadata, 'pg_isready', '-h', '127.0.0.1', '-U', 'mcpdbhub', '-d', 'mcpdbhub', check=False).returncode == 0:
            break
        time.sleep(.25)
    else:
        raise RuntimeError('Metadata PostgreSQL did not become ready')
    runtime += ['-e', 'MCPDBHUB_DATABASE_URL=postgres://mcpdbhub:'+metadata_password+'@'+metadata+':5432/mcpdbhub?sslmode=disable']
    docker("run", "-d", "--name", name, "--label", "com.mcpdbhub.fixture=true", "--network", "mcpdbhub-test",
           "--read-only", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m", "--cap-drop", "ALL",
           "--security-opt", "no-new-privileges", "-v", volume+":/data", "-p", "127.0.0.1::8080", *runtime, IMAGE, 'serve')
    created = True
    port = docker("port", name, "8080/tcp").stdout.strip().rsplit(":", 1)[1]
    base = "http://127.0.0.1:"+port
    ready()
    setup = re.search(r"First-time setup token: (\S+)", docker("logs", name).stderr).group(1)
    initial_password = uuid.uuid4().hex
    csrf = request("POST", "/api/setup", {"username":"admin","token": setup, "password": initial_password})["csrf"]
    assert docker("exec", name, "id", "-u").stdout.strip() == "10001"
    assert docker("exec", name, "sh", "-c", "command -v node", check=False).returncode != 0
    version_output = docker('exec', name, binary, 'version').stdout.strip()
    runtime_version = re.fullmatch(r'ContextGate (\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?) \(commit ([^)]+)\)', version_output)
    assert runtime_version, 'missing runtime version'
    legacy_binary = str(pathlib.PurePosixPath(binary).with_name('mcpdbhub'))
    assert docker('exec', name, legacy_binary, 'version').stdout.strip() == version_output
    if manifest:
        assert version_output == 'ContextGate '+manifest['version']+' (commit '+manifest['commit']+')'
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
        exported = await_export(lambda status: status['accepted'] >= 1 and status['pending'] == 0)
        http_accepted = exported['status']['accepted']
        config = exported['config']
        config.update(protocol='grpc', endpoint='http://'+collector+':4317')
        assert request('POST', '/api/settings/audit-export/test', config)['accepted'] is True
        request('PUT', '/api/settings/audit-export', config)
        assert request('POST', '/mcp', query, agent['token'])['result']['structuredContent']['data'] == [['3']]
        # Configuration changes also emit management audit records.
        exported = await_export(lambda status: status['accepted'] > http_accepted and status['pending'] == 0)
        grpc_accepted = exported['status']['accepted']
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
        assert initialized['result']['serverInfo']['name'] == 'contextgate'
        bridge.stdin.write(json.dumps({'jsonrpc': '2.0', 'method': 'notifications/initialized'})+'\n')
        tools = bridge_call({'jsonrpc': '2.0', 'id': 2, 'method': 'tools/list'})
        assert len(tools['result']['tools']) == 15
        assert 'query_http_api' in {tool['name'] for tool in tools['result']['tools']}
        bridged = bridge_call({**query, 'id': 3})['result']['structuredContent']
        assert bridged['data'] == [['3']]
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
    recovered = request("POST", "/mcp", query, agent["token"])["result"]["structuredContent"]
    assert recovered["data"] == [["3"]]
    if args.otlp:
        saved = export_view()
        assert saved['config']['protocol'] == 'grpc' and saved['headers_configured'] is True
        assert saved['status']['pending'] == 3 and saved['status']['accepted'] == grpc_accepted
        docker('start', collector)
        await_export(lambda status: status['pending'] == 0 and status['accepted'] == grpc_accepted + 3)
        logs = docker('logs', collector)
        wire = logs.stdout+logs.stderr
        assert source['id'] in wire and 'event.name: Str(mcpdbhub.audit)' in wire
        assert 'package-test-only' not in wire and 'SELECT count(*)' not in wire and agent['token'] not in wire
    configuration = request('POST', '/api/configuration-agents', {'name': 'Distribution setup Agent'})
    configuration_token = configuration['token']
    def configure(tool, arguments):
        reply = request('POST', '/mcp/config', {'jsonrpc': '2.0', 'id': 100, 'method': 'tools/call',
                        'params': {'name': tool, 'arguments': arguments}}, configuration_token)['result']
        assert not reply.get('isError'), 'Configuration MCP tool failed: '+tool
        return reply['structuredContent']
    config_tools = request('POST', '/mcp/config', {'jsonrpc': '2.0', 'id': 101, 'method': 'tools/list'}, configuration_token)
    assert len(config_tools['result']['tools']) == 22
    assert configure('get_configuration_guide', {})['publication'] == 'administrator_only'
    editable = configure('get_source_configuration', {'source_id': source['id']})
    assert fixture['password'] not in json.dumps(editable)
    request('POST', '/mcp/config', query, agent['token'], expected=401)
    request('POST', '/mcp', query, configuration_token, expected=401)
    semantic_path = '/api/sources/'+source['id']+'/semantics'
    snapshot = {'format_version': 1, 'overview': 'Release package semantics', 'entries': [{
        'id': 'event-count', 'kind': 'template', 'name': 'Event count', 'template': {
            'enabled': True, 'tool': 'query_sql',
            'query_json': json.dumps({'query': 'SELECT count(*) FROM events WHERE id >= $1', 'params': [0]}),
            'parameters': [{'name': 'minimum_id', 'type': 'integer', 'required': True, 'pointers': ['/params/0'], 'minimum': '1'}],
            'example_json': '{"minimum_id":1}'}}]}
    draft = configure('save_semantic_draft', {'source_id': source['id'], 'revision': '0', 'snapshot': snapshot})
    request('POST', semantic_path+'/publish', {'revision': draft['revision']}, expected=400)
    configure('trial_query_template', {'source_id': source['id'], 'revision': draft['revision'], 'template_id': 'event-count'})
    published = request('POST', semantic_path+'/publish', {'revision': draft['revision']})
    ontology = configure('create_ontology', {'definition': {
        'format_version': 1, 'name': 'Package business ontology',
        'entities': [{'id': 'Event', 'name': 'Event'}], 'properties': [], 'relations': []}})
    request('POST', '/api/ontologies/'+ontology['id']+'/publish', {'revision': ontology['revision']})
    snapshot['format_version'] = 2
    snapshot['ontology'] = {'ontology_id': ontology['id'], 'version': '1',
        'entities': [{'entity': 'Event', 'objects': [{'namespace': 'public', 'object': 'events'}]}],
        'properties': [], 'relations': []}
    snapshot['entries'][0]['template']['concept_refs'] = ['ontology:entity_type:Event']
    draft = request('PUT', semantic_path, {'revision': published['revision'], 'snapshot': snapshot})
    configure('check_ontology_mapping', {'source_id': source['id'], 'revision': draft['revision']})
    published = request('POST', semantic_path+'/publish', {'revision': draft['revision']})
    assert published['published']['entries'][0]['template']['execution_version'] == '1'
    preview = request('POST', semantic_path+'/preview', {'agent_id': agent['agent']['id'], 'kind': 'entity_type'})
    assert preview['entries'][0]['id'] == 'ontology:entity_type:Event'
    source = next(s for s in request('GET', '/api/sources') if s['id'] == source['id'])
    source['query_access_mode'] = 'templates_only'
    request('PUT', '/api/sources/'+source['id'], source)
    native_query = query
    assert request('POST', '/mcp', native_query, agent['token'])['result']['isError'] is True
    query = {'jsonrpc': '2.0', 'id': 10, 'method': 'tools/call', 'params': {
        'name': 'execute_query_template', 'arguments': {'source_id': source['id'], 'template_id': 'event-count',
        'execution_version': '1', 'parameters': {'minimum_id': 1}}}}
    content = request('POST', '/mcp', query, agent['token'])['result']['structuredContent']
    assert content['data'] == [['3']] and content['template_id'] == 'event-count' and content['template_version'] == '1'
    assert content['ontology_context'] == {'ontology_id': ontology['id'], 'version': '1', 'concept_refs': ['ontology:entity_type:Event']}
    bridge = subprocess.Popen(['docker', 'exec', '-i', '-e', 'MCPDBHUB_TOKEN', name, binary,
                               'stdio', '--url', 'http://127.0.0.1:8080/mcp'],
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              text=True, env={**os.environ, 'MCPDBHUB_TOKEN': agent['token']})
    try:
        bridge_call({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {
            'protocolVersion': '2025-11-25', 'capabilities': {}, 'clientInfo': {'name': 'template-dist-verification', 'version': '1'}}})
        bridge.stdin.write(json.dumps({'jsonrpc': '2.0', 'method': 'notifications/initialized'})+'\n')
        assert bridge_call({**native_query, 'id': 2})['result']['isError'] is True
        bridged = bridge_call({**query, 'id': 3})['result']['structuredContent']
        assert bridged['data'] == [['3']] and bridged['ontology_context'] == content['ontology_context']
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
        assert 'mcpdbhub.audit.ontology_id: Str('+ontology['id']+')' in wire
        assert 'mcpdbhub.audit.ontology_version: Str(1)' in wire
        assert 'Package business ontology' not in wire
        assert 'minimum_id' not in wire and 'SELECT count(*)' not in wire
    bridge = subprocess.Popen(['docker', 'exec', '-i', '-e', 'MCPDBHUB_TOKEN', name, binary,
                               'stdio', '--url', 'http://127.0.0.1:8080/mcp/config'],
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              text=True, env={**os.environ, 'MCPDBHUB_TOKEN': configuration_token})
    try:
        initialized = bridge_call({'jsonrpc': '2.0', 'id': 1, 'method': 'initialize', 'params': {
            'protocolVersion': '2025-11-25', 'capabilities': {}, 'clientInfo': {'name': 'config-dist-verification', 'version': '1'}}})
        assert initialized['result']['serverInfo']['name'] == 'contextgate-configuration'
        bridge.stdin.write(json.dumps({'jsonrpc': '2.0', 'method': 'notifications/initialized'})+'\n')
        reply = bridge_call({'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call', 'params': {
            'name': 'get_semantic_draft', 'arguments': {'source_id': source['id']}}})['result']
        assert not reply.get('isError')
        assert reply['structuredContent']['published']['entries'][0]['id'] == 'event-count'
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
        await_export(lambda status: status['pending'] == 0)
        logs = docker('logs', collector)
        wire = logs.stdout+logs.stderr
        assert configuration['id'] in wire and 'configuration.trial_query_template' in wire
        assert configuration_token not in wire and 'minimum_id' not in wire
    docker("stop", name)
    replacement_password = uuid.uuid4().hex
    recovery = subprocess.run(["docker", "run", "--rm", "-i", "--network", "mcpdbhub-test", "--read-only", "--cap-drop", "ALL",
                               "-v", volume+":/data", *runtime, IMAGE, "reset-password", "--password-stdin"],
                              input=replacement_password, text=True, capture_output=True)
    if recovery.returncode:
        raise RuntimeError("Password recovery failed: " + recovery.stderr[-2000:])
    assert replacement_password not in recovery.stdout + recovery.stderr
    docker("start", name)
    port = docker("port", name, "8080/tcp").stdout.strip().rsplit(":", 1)[1]
    base = "http://127.0.0.1:"+port
    ready()
    request("GET", "/api/sources", expected=401)
    request("POST", "/api/login", {"username":"admin","password": initial_password}, expected=401)
    csrf = request("POST", "/api/login", {"username":"admin","password": replacement_password})["csrf"]
    assert any(s["id"] == source["id"] for s in request("GET", "/api/sources"))
    recovered = request("POST", "/mcp", query, agent["token"])["result"]["structuredContent"]
    assert recovered["data"] == [["3"]] and recovered["ontology_context"] == content["ontology_context"]
    assert request('POST', '/mcp', native_query, agent['token'])['result']['isError'] is True
    request('POST', '/mcp/config', query, configuration_token, expected=401)
    restored_identity = request('GET', '/api/configuration-agents')['agents'][0]
    assert restored_identity['id'] == configuration['id']
    configuration = request('POST', '/api/configuration-agents/'+restored_identity['id']+'/token', {'revision': restored_identity['revision']})
    configuration_token = configuration['token']
    assert configure('get_configuration_guide', {})['publication'] == 'administrator_only'
    if args.otlp:
        check_administrator_otlp(source['id'], configuration)
    check_http_workflow()
    request('DELETE', '/api/configuration-agents/'+configuration['id'])
    request('POST', '/mcp/config', query, configuration_token, expected=401)
    request("DELETE", "/api/agents/"+agent["agent"]["id"])
    request("POST", "/mcp", query, agent["token"], expected=401)
    image = json.loads(docker("image", "inspect", IMAGE).stdout)[0]
    result = {"product": "ContextGate", "result": "passed", "image_id": image["Id"], "architecture": image["Architecture"],
              "version": runtime_version.group(1),
              "source_sha256": source_digest(), "verified_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "metadata_store": "PostgreSQL 17.11",
              "checks": ["contextgate_cli_and_legacy_alias", "contextgate_stdio_server_identity", "postgres_metadata_initialization", "nonroot_uid_10001", "readonly_root_filesystem", "no_node_runtime", "bootstrap",
                         "source_persistence", "HTTP_MCP_query", "stdio_initialize_discovery_and_query", "restart_session_agent_and_source_recovery", "revoked_agent_denied",
                         "password_recovery_cli", "old_password_and_admin_sessions_rejected", "recovery_preserves_database_credentials_and_agent_token"]}
    result['checks'].extend(['template_trial_required_for_publication', 'HTTP_and_stdio_template_query',
                             'HTTP_and_stdio_templates_only_native_denial', 'published_template_and_evidence_restart_recovery', 'ontology_mapping_and_agent_projection',
                             'ontology_publication_preserves_template_evidence', 'HTTP_and_stdio_ontology_context', 'ontology_restart_recovery'])
    result['checks'].extend(['configuration_MCP_typed_discovery', 'configuration_MCP_credential_isolation',
                             'configuration_MCP_semantic_trial_and_ontology_workflow', 'configuration_MCP_stdio',
                             'configuration_MCP_restart_and_revocation'])
    result['checks'].extend(['HTTP_API_GET_POST_lossless_results', 'HTTP_API_native_pagination',
                             'HTTP_API_unverified_permission_evidence', 'HTTP_API_trial_publication_and_native_template_equivalence',
                             'HTTP_API_templates_only_denial', 'business_catalog_executable_query_view',
                             'parameter_binding_position_suggestions', 'single_answer_capture_review_and_history_summary'])
    if args.otlp:
        result['checks'].append('OTLP_configuration_MCP_correlation_and_redaction')
        result['checks'].append('OTLP_HTTP_and_gRPC_two_administrators_UI_and_configuration_MCP_attribution')
        result['collector_version'] = '0.160.0'
        result['checks'].extend(['OTLP_HTTP_protobuf', 'OTLP_gRPC', 'OTLP_header_and_payload_redaction',
                                 'OTLP_receiver_outage_query_isolation', 'OTLP_pending_restart_and_recovery'])
        result['checks'].extend(['OTLP_template_correlation_and_redaction', 'OTLP_ontology_correlation_and_redaction'])
    if manifest:
        result.update(architecture=manifest['arch'], commit=manifest['commit'], image_id=manifest['image_id'],
                      binary_sha256=manifest['binary_sha256'], archive=args.archive.name,
                      archive_sha256=hashlib.sha256(args.archive.read_bytes()).hexdigest())
        result['checks'].extend(['independent_archive_extraction', 'private_runtime_libraries', 'version_matches_source', 'bilingual_installation_guides'])
        result['checks'].extend(['bilingual_semantics_guides_and_examples', 'bilingual_ontology_guides_and_examples'])
        assert all((package/p).exists() for p in ('docs/README.md', 'docs/README.zh-CN.md', 'docs/getting-started.md', 'docs/configuration-mcp.md', 'docs/configuration-mcp.zh-CN.md', 'docs/operations.md', 'docs/releases/'+manifest['version']+'.md', 'docs/http-api.md', 'docs/http-api.zh-CN.md'))
        result['checks'].append('bundled_configuration_and_operations_help')
        assert manifest['license'] == 'Apache-2.0'
        for filename in ('LICENSE', 'NOTICE'):
            assert (package/filename).read_bytes() == (ROOT/filename).read_bytes(), 'Archive project license differs from release source'
        assert (package/'docs/administrators.md').is_file() and (package/'docs/administrators.zh-CN.md').is_file()
        result['checks'].extend(['Apache_2_0_license_and_notice', 'bilingual_administrator_upgrade_help'])
        result['license'] = manifest['license']
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
    if metadata:
        docker("rm", "-fv", metadata, check=False)
    if collector:
        docker('rm', '-f', collector, check=False)
    if http_fixture:
        docker('rm', '-f', http_fixture, check=False)
    docker("volume", "rm", volume, check=False)
    if unpacked:
        unpacked.cleanup()
