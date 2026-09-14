#!/usr/bin/env python3
"""Exercise the running ContextGate service against its isolated PostgreSQL fixture.

Requires MCPDBHUB_ADMIN_PASSWORD, optional MCPDBHUB_TEST_URL. Creates and removes
one test source; stops/restarts only mcpdbhub-it-postgres (fixture label required).
"""
import http.cookiejar,json,os,pathlib,subprocess,time,urllib.request,urllib.error
root=pathlib.Path(__file__).resolve().parents[1]
base=os.environ.get('MCPDBHUB_TEST_URL','http://127.0.0.1:19840')
fixture='mcpdbhub-it-postgres'
label=subprocess.check_output(['docker','inspect','-f','{{index .Config.Labels "com.mcpdbhub.fixture"}}',fixture],text=True).strip()
assert label=='true','Refuse to restart a non-fixture container'
client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
csrf=''
def api(method,path,body=None,ok=True):
 req=urllib.request.Request(base+path,data=json.dumps(body).encode() if body is not None else None,method=method,headers={'Content-Type':'application/json','X-CSRF-Token':csrf})
 try:
  with client.open(req,timeout=20) as res: out=json.load(res)
  assert ok, 'Expected query to fail'
  return out
 except urllib.error.HTTPError as e:
  out=json.load(e)
  if ok:raise RuntimeError(out)
  return out
csrf=api('POST','/api/login',{'password':os.environ['MCPDBHUB_ADMIN_PASSWORD']})['csrf']
src=json.loads((root/'artifacts/matrix/postgres-fixture.json').read_text())[0]['source']
src.update(name='Recovery verification fixture',enabled=True)
id=api('POST','/api/sources',src)['id']
def query(sql,ok=True,**limits):return api('POST','/api/query',{'operation':'query_sql','query':{'source_id':id,'query':sql,**limits}},ok)
checks=[]
try:
 assert query('SELECT count(*) FROM events')['data'][0][0]=='3'
 start=time.monotonic()
 denied=query('SELECT sum(i) FROM generate_series(1,1000000000) AS t(i)',False,timeout_seconds=1)
 assert denied['error']['code']=='timeout',denied
 assert time.monotonic()-start<5
 checks.append('postgres_statement_timeout')
 assert query('SELECT count(*) FROM events')['data'][0][0]=='3'
 checks.append('connection_reused_after_cancellation')
 subprocess.run(['docker','stop','-t','1',fixture],check=True,stdout=subprocess.DEVNULL)
 failed=query('SELECT count(*) FROM events',False,timeout_seconds=2)
 assert failed['error']['code'] in ('database_error','database_connection','timeout'),failed
 subprocess.run(['docker','start',fixture],check=True,stdout=subprocess.DEVNULL)
 for _ in range(30):
  p=subprocess.run(['docker','exec',fixture,'pg_isready','-U','reader','-d','hubtest'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  if p.returncode==0:break
  time.sleep(.5)
 assert query('SELECT count(*) FROM events')['data'][0][0]=='3'
 checks.append('postgres_restart_connection_recovery')
 updated=api('GET','/api/sources');current=next(s for s in updated if s['id']==id);current['enabled']=False
 api('PUT','/api/sources/'+id,current)
 assert query('SELECT count(*) FROM events',False)['error']['code']=='not_found'
 checks.append('source_disable_denies_queries')
 current=next(s for s in api('GET','/api/sources') if s['id']==id);current['enabled']=True
 api('PUT','/api/sources/'+id,current)
 assert query('SELECT count(*) FROM events')['data'][0][0]=='3'
 checks.append('source_reenable_and_data_unchanged')
 out={'checks':checks,'result':'passed','database':'PostgreSQL 17.11','environment':'OrbStack Linux arm64'}
 (root/'docs/verification/recovery.json').write_text(json.dumps(out,indent=2)+'\n')
 print(json.dumps(out))
finally:
 subprocess.run(['docker','start',fixture],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 api('DELETE','/api/sources/'+id)
