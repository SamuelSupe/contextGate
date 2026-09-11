#!/usr/bin/env python3
"""Provision isolated fixtures and run the actual Go adapters in OrbStack.

Usage: python3 scripts/matrix.py postgres mysql ...
Only containers created by this script are removed. No host ports are published.
"""
import hashlib, json, os, pathlib, subprocess, sys, time
ROOT = pathlib.Path(__file__).resolve().parents[1]
OUT = ROOT / "artifacts" / "matrix"
OUT.mkdir(parents=True, exist_ok=True)
ADMIN = "fixture-admin-only"
READ = "fixture-reader-only"
SEARCH_ADMIN = "Violet-Quartz-Harbor-2026!"
IMAGES = {
 "postgres":"postgres:17.11", "mysql":"mysql:8.4.11", "mariadb":"mariadb:10.11",
 "tidb":"pingcap/tidb:v8.5.1", "cockroachdb":"cockroachdb/cockroach:v24.3.15",
 "timescaledb":"timescale/timescaledb:2.25.2-pg17", "mongodb":"mongo:7.0.39-jammy",
 "redis":"redis:7.4.6-alpine", "valkey":"valkey/valkey:8.1.9-alpine",
 "clickhouse":"clickhouse/clickhouse-server@sha256:b002e56ed5c16e224c312527f6fcba7e77216fec5d7a88a7828f59efc614feb5",
 "elasticsearch":"docker.elastic.co/elasticsearch/elasticsearch@sha256:aa6ee0ea2d708cea22a24ed44a420c5bbde9853d58a872e1e870f2086d04f652",
 "opensearch":"opensearchproject/opensearch@sha256:23297b8d8545e129dd58c254ed08d786dc552410ba772983ad2af31048d2f04b",
 "neo4j":"neo4j@sha256:225b14923479dde15797534ee887d9950077308f6b56c96d2fb0daf3a2194814",
 "cassandra":"cassandra@sha256:e52bb93c21f69cc5c2f5eb9e7d9736b10a047ad38bc7776480c7dbbf7d4e0ba7",
 "scylladb":"scylladb/scylla@sha256:62dcb58bdc1d1e9aad47c2af02a6a36e8f7a0a61ab5ac13b447898f87d3730c8",
 "influxdb1":"influxdb:1.8.10",
 "influxdb2":"influxdb@sha256:b8d940ca9376f85118260f5b6bd236b8a00b1749c3350c5578d4cde8e27f31f2",
 "influxdb3":"influxdb@sha256:f4a6d4a76f0ed0a196cc997da472cd0b7ae52a766430493a1bead807ab8c1217",
}
def run(args, data=None, check=True):
 p = subprocess.run(args, input=data, text=True, capture_output=True)
 if check and p.returncode: raise RuntimeError(p.stderr[-3000:] or p.stdout[-3000:])
 return p
def docker(*args,data=None,check=True): return run(["docker",*args],data,check)
def wait(fn, seconds=150):
 deadline=time.monotonic()+seconds; last=None
 while time.monotonic()<deadline:
  try: return fn()
  except RuntimeError as e: last=e; time.sleep(2)
 raise RuntimeError(f"readiness timeout: {last}")
def query(q, rows, contains="", error=False, **expected):
 return {"query": q, "rows": rows, "contains": contains, "error": error, **expected}

def search_certificates(kind, name):
 directory = OUT / "tls" / kind
 directory.mkdir(parents=True, exist_ok=True)
 target = "/work/artifacts/matrix/tls/" + kind
 docker("exec", "mcpdbhub-dev", "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
        "-keyout", target+"/ca.key", "-out", target+"/ca.crt", "-days", "2", "-subj", "/CN=MCP DB Hub fixture CA")
 docker("exec", "mcpdbhub-dev", "openssl", "req", "-newkey", "rsa:2048", "-nodes",
        "-keyout", target+"/server.key", "-out", target+"/server.csr", "-subj", "/CN="+name)
 (directory/"extensions.cnf").write_text("subjectAltName=DNS:"+name+"\nextendedKeyUsage=serverAuth\n")
 docker("exec", "mcpdbhub-dev", "openssl", "x509", "-req", "-in", target+"/server.csr",
        "-CA", target+"/ca.crt", "-CAkey", target+"/ca.key", "-CAcreateserial", "-out", target+"/server.crt",
        "-days", "2", "-extfile", target+"/extensions.cnf")
 # Fixture keys must be readable by the database image's non-root user.
 (directory/"server.key").chmod(0o644)
 return directory
def sql_manifest(kind,host):
 pg=kind in ("postgres","timescaledb","cockroachdb")
 port={"postgres":5432,"timescaledb":5432,"cockroachdb":26257,"tidb":4000,"clickhouse":9000}.get(kind,3306)
 s={"kind":kind,"host":host,"port":port,"database":"hubtest","username":"reader","password":READ,"tls_mode":"disable"}
 marker="$1" if pg else "?"
 if kind=="clickhouse":marker="{id:Int64}"
 bound={"named_params":{"id":1}} if kind=="clickhouse" else {"params":[1]}
 complex_query=f"WITH totals AS (SELECT id,amount FROM events WHERE id >= {marker}) SELECT id,amount,sum(amount) OVER () AS total FROM totals ORDER BY id"
 manifest = {"name":kind,"source":s,"namespace":"public" if pg else "hubtest","object":"events","baseline":{"query":"SELECT * FROM events ORDER BY id"},"queries":[query({"query":complex_query,**bound},3),query({"query":"SELECT big_num,amount FROM events WHERE id=1"},1,"9007199254740993"),query({"query":"SELECT * FROM events WHERE id=99"},0),query({"query":"SELECT missing FROM events"},0,error=True)],"denied":[{"query":q} for q in ["DELETE FROM events","SELECT 1; DELETE FROM events","WITH d AS (DELETE FROM events RETURNING *) SELECT * FROM d","SELECT dangerous_function()","SELECT * FROM events FOR UPDATE","SELECT 1 INTO OUTFILE '/tmp/escape'"]]}
 if kind != "clickhouse":
  manifest["queries"].append(query({"query":"SELECT replace('abc','a','x') AS value"},1,"xbc"))
  statement="SELECT format('%s','ok') AS value" if pg else "SELECT format(1234.5,1) AS value"
  manifest["queries"].append(query({"query":statement},1,"ok" if pg else "1,234.5"))
 return manifest

def provision(kind,name):
 env=[];cmd=[];memory="768m"
 if kind in ("postgres","timescaledb"):env=["POSTGRES_PASSWORD="+ADMIN,"POSTGRES_DB=hubtest"]
 if kind in ("mysql","mariadb"):env=["MYSQL_ROOT_PASSWORD="+ADMIN,"MYSQL_DATABASE=hubtest"]
 if kind=="tidb":cmd=["--store=unistore","--path=/tmp/tidb","--host=0.0.0.0"]
 if kind=="cockroachdb":cmd=["start-single-node","--insecure","--listen-addr=0.0.0.0:26257","--cache=128MiB","--max-sql-memory=128MiB"]
 if kind=="mongodb":env=["MONGO_INITDB_ROOT_USERNAME=admin","MONGO_INITDB_ROOT_PASSWORD="+ADMIN]
 if kind=="clickhouse":env=["CLICKHOUSE_PASSWORD="+ADMIN,"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1"];memory="1536m"
 if kind=="elasticsearch":
  env=["discovery.type=single-node", "xpack.security.enabled=true", "ELASTIC_PASSWORD="+SEARCH_ADMIN,
       "xpack.security.http.ssl.enabled=true", "xpack.security.http.ssl.key=certs/server.key",
       "xpack.security.http.ssl.certificate=certs/server.crt", "xpack.security.http.ssl.certificate_authorities=certs/ca.crt",
       "ES_JAVA_OPTS=-Xms512m -Xmx512m"]
  memory="1536m"
 if kind=="opensearch":
  env=["discovery.type=single-node", "OPENSEARCH_INITIAL_ADMIN_PASSWORD="+SEARCH_ADMIN,
       "plugins.security.ssl.http.pemkey_filepath=certs/server.key", "plugins.security.ssl.http.pemcert_filepath=certs/server.crt",
       "plugins.security.ssl.http.pemtrustedcas_filepath=certs/ca.crt", "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m"]
  memory="1536m"
 if kind=="neo4j":env=["NEO4J_AUTH=neo4j/"+ADMIN,"NEO4J_server_memory_heap_initial__size=256m","NEO4J_server_memory_heap_max__size=512m","NEO4J_server_memory_pagecache_size=128m"];memory="1536m"
 if kind=="cassandra":
  env=["MAX_HEAP_SIZE=768M","HEAP_NEWSIZE=128M"];memory="2g"
  cmd=["bash","-c","sed -i 's/^authenticator:.*/authenticator: PasswordAuthenticator/;s/^authorizer:.*/authorizer: CassandraAuthorizer/' /etc/cassandra/cassandra.yaml; exec /usr/local/bin/docker-entrypoint.sh cassandra -f"]
 if kind=="scylladb":cmd=["--smp","1","--memory","1G","--reserve-memory","128M","--overprovisioned","1","--developer-mode","1","--authenticator","PasswordAuthenticator","--authorizer","CassandraAuthorizer"];memory="3g"
 if kind=="influxdb1":env=["INFLUXDB_DB=hubtest","INFLUXDB_HTTP_AUTH_ENABLED=true","INFLUXDB_ADMIN_USER=admin","INFLUXDB_ADMIN_PASSWORD="+ADMIN]
 if kind=="influxdb2":env=["DOCKER_INFLUXDB_INIT_MODE=setup","DOCKER_INFLUXDB_INIT_USERNAME=admin","DOCKER_INFLUXDB_INIT_PASSWORD="+ADMIN,"DOCKER_INFLUXDB_INIT_ORG=hub","DOCKER_INFLUXDB_INIT_BUCKET=hubtest","DOCKER_INFLUXDB_INIT_ADMIN_TOKEN="+ADMIN]
 if kind=="influxdb3":cmd=["influxdb3","serve","--node-id=hubtest","--object-store=memory","--http-bind=0.0.0.0:8181"];memory="1536m"
 args=["run","-d","--name",name,"--label","com.mcpdbhub.fixture=true","--network","mcpdbhub-test","--memory",memory,"--cpus","2"]
 if kind in ("elasticsearch", "opensearch"):
  directory = search_certificates(kind, name)
  args.extend(["-v", str(directory)+":/usr/share/"+kind+"/config/certs:ro"])
 if kind=="scylladb":
  config=OUT/"scylla.yaml"
  original=docker("run","--rm","--entrypoint","cat",IMAGES[kind],"/etc/scylla/scylla.yaml").stdout
  config.write_text(original+"\nauth_superuser_name: cassandra\nauth_superuser_salted_password: '$6$mcpdbhub$YU59LSSh/yowLxaFu3/4PJP2jlMwjBDe5ECyc9nCcE9XjnQKoolCkJllJqI0bMO2k.GzDDEEt8hVGJMQheUoQ0'\n")
  args.extend(["-v",str(config)+":/etc/scylla/scylla.yaml:ro"])
 if kind=="cockroachdb":args.extend(["--entrypoint","/cockroach/cockroach"])
 for v in env: args.extend(["-e",v])
 docker(*args,IMAGES[kind],*cmd)
def seed(kind,name):
 if kind in ("postgres","timescaledb","cockroachdb","mysql","mariadb","tidb","clickhouse"):
  m=sql_manifest(kind,name)
  def execute(sql):
   if kind in ("postgres","timescaledb"):return docker("exec","-i","-e","PGPASSWORD="+ADMIN,name,"psql","-h","127.0.0.1","-U","postgres","-d","hubtest","-v","ON_ERROR_STOP=1",data=sql)
   if kind=="cockroachdb":return docker("exec","-i",name,"cockroach","sql","--insecure",data=sql)
   if kind=="clickhouse":return docker("exec","-i",name,"clickhouse-client","--password",ADMIN,"--multiquery",data=sql)
   if kind=="tidb":return docker("run","--rm","-i","--network","mcpdbhub-test","mysql:8.4.11","mysql","-h",name,"-P","4000","-uroot",data=sql)
   # The image initializes through a temporary socket-only server. Wait for TCP
   # so fixture creation cannot race that server's shutdown.
   return docker("exec","-i",name,"mariadb" if kind=="mariadb" else "mysql","--protocol=TCP","-h127.0.0.1","-uroot","-p"+ADMIN,data=sql)
  wait(lambda:execute("SELECT 1;"))
  setup="CREATE DATABASE IF NOT EXISTS hubtest; USE hubtest; "
  if kind in ("postgres","timescaledb"):setup=""
  if kind=="clickhouse":
   setup+="CREATE TABLE events(id Int64,amount Decimal(20,2),note String,big_num Int64) ENGINE=Memory; "
  else:setup+="CREATE TABLE events(id BIGINT PRIMARY KEY,amount DECIMAL(20,2),note VARCHAR(120),big_num BIGINT); "
  setup+="INSERT INTO events VALUES(1,12.50,'one',9007199254740993),(2,20.25,'two',4),(3,4.00,'three',5); "
  if kind in ("postgres","timescaledb"):
   setup+=f"CREATE USER reader PASSWORD '{READ}'; GRANT CONNECT ON DATABASE hubtest TO reader; GRANT USAGE ON SCHEMA public TO reader; GRANT SELECT ON ALL TABLES IN SCHEMA public TO reader;"
  elif kind=="cockroachdb":setup+="CREATE USER reader; GRANT SELECT ON TABLE events TO reader; GRANT CONNECT ON DATABASE hubtest TO reader;";m["source"]["password"]=""
  elif kind=="clickhouse":setup+=f"CREATE USER reader IDENTIFIED BY '{READ}'; GRANT SELECT ON hubtest.* TO reader; GRANT SELECT ON system.* TO reader;"
  else:setup+=f"CREATE USER 'reader'@'%' IDENTIFIED BY '{READ}'; GRANT SELECT ON hubtest.* TO 'reader'@'%';"
  execute(setup)
  if kind=="timescaledb":
   execute("CREATE TABLE metrics(time TIMESTAMPTZ NOT NULL,value DOUBLE PRECISION); SELECT create_hypertable('metrics','time'); INSERT INTO metrics VALUES('2026-09-11T00:00:00Z',1),('2026-09-11T00:10:00Z',3); GRANT SELECT ON metrics TO reader;")
   m["queries"].append(query({"query":"SELECT time_bucket(INTERVAL '1 hour',time) AS bucket, avg(value) FROM metrics GROUP BY bucket"},1))
  return m
 if kind in ("redis","valkey"):
  cli="valkey-cli" if kind=="valkey" else "redis-cli"
  def cmd(*a):return docker("exec",name,cli,*a)
  wait(lambda:cmd("PING"));cmd("SET","events","9007199254740993");cmd("HSET","eventhash","amount","12.50","note","one");cmd("RPUSH","eventlist","one","two","three");cmd("SADD","eventset","one","two");cmd("ZADD","eventzset","1","one","2","two");cmd("XADD","eventstream","*","note","one")
  cmd("ACL","SETUSER","reader","on",">"+READ,"~*","+@read","+ping","+info","+hello","+client|setinfo","+client|setname","+select")
  baseline={"command":"LRANGE","args":["eventlist","0","-1"]}
  queries=[query({"command":"GET","args":["events"]},1,"9007199254740993"),query({"command":"HMGET","args":["eventhash","amount","note"]},2,"12.50"),query({"command":"SSCAN","args":["eventset","0"]},2),query({"command":"ZRANGE","args":["eventzset","0","-1","WITHSCORES"]},4),query({"command":"XRANGE","args":["eventstream","-","+"]},1),query({"command":"LRANGE","args":["missing","0","-1"]},0),query({"command":"GET","args":[]},0,error=True)]
  return {"name":kind,"source":{"kind":kind,"host":name,"port":6379,"database":"0","username":"reader","password":READ,"tls_mode":"disable"},"namespace":"0","object":"events","baseline":baseline,"queries":queries,"denied":[{"command":"SET","args":["events","changed"]},{"command":"EVAL","args":["return 1","0"]},{"command":"CONFIG","args":["GET","*"]}]}
 if kind=="mongodb":
  def js(code):return docker("exec",name,"mongosh","--quiet","-u","admin","-p",ADMIN,"--authenticationDatabase","admin","--eval",code)
  wait(lambda:js("db.runCommand({ping:1})"));js("db=db.getSiblingDB('hubtest'); db.events.insertMany([{id:1,n:Long('9007199254740993'),amount:Decimal128('12.50'),tags:['red','blue']},{id:2,n:Long('4'),amount:Decimal128('20.25')},{id:3,n:Long('5'),amount:Decimal128('4.00')}]); db.createUser({user:'reader',pwd:'"+READ+"',roles:[{role:'read',db:'hubtest'}]})")
  return {"name":kind,"source":{"kind":kind,"host":name,"port":27017,"database":"hubtest","username":"reader","password":READ,"tls_mode":"disable"},"namespace":"hubtest","object":"events","baseline":{"object":"events","operation":"find","sort":{"id":1}},"queries":[
   query({"object":"events", "operation":"distinct", "query":"tags", "filter":{"id":1}}, 2, '"_id":"red"'),
   query({"object":"events", "operation":"distinct", "query":"tags", "filter":{"id":3}}, 0),
   query({"object":"events","filter":{"id":1}},1,"9007199254740993"),query({"object":"events","operation":"aggregate","pipeline":[{"$match":{"id":{"$gte":1}}},{"$group":{"_id":None,"total":{"$sum":"$amount"}}}]},1,"36.75"),query({"object":"events","operation":"count"},1),query({"object":"events","operation":"distinct","query":"id"},3),query({"object":"events","filter":{"id":99}},0),query({"object":"events","filter":{"$invalid":1}},0,error=True)],"denied":[{"object":"events","operation":"aggregate","pipeline":[{"$out":"copy"}]},{"object":"events","filter":{"$where":"return true"}},{"object":"events","operation":"delete"}]}
 def http(method,path,body=None,headers=(),port=9200):
  search = kind in ("elasticsearch", "opensearch")
  scheme = "https" if search else "http"
  args=["exec","-i","mcpdbhub-dev","curl","-sS","--fail-with-body","--max-time","15","-X",method,scheme+"://"+name+":"+str(port)+path]
  if search:
   user = "elastic" if kind == "elasticsearch" else "admin"
   args.extend(["--cacert", "/work/artifacts/matrix/tls/"+kind+"/ca.crt", "-u", user+":"+SEARCH_ADMIN])
  for h in headers:args.extend(["-H",h])
  if body is not None:args.extend(["--data-binary","@-"])
  return docker(*args,data=body).stdout
 if kind in ("elasticsearch","opensearch"):
  wait(lambda:http("GET","/"))
  http("PUT","/events",json.dumps({"mappings":{"properties":{"id":{"type":"integer"},"amount":{"type":"double"},"big_num":{"type":"long"},"note":{"type":"keyword"}}}}),["Content-Type: application/json"])
  lines=[]
  for i in range(1,4):lines.extend([json.dumps({"index":{"_index":"events","_id":str(i)}}),json.dumps({"id":i,"amount":12.5,"big_num":9007199254740993,"note":"one"})])
  http("POST","/_bulk?refresh=true","\n".join(lines)+"\n",["Content-Type: application/x-ndjson"])
  if kind == "elasticsearch":
   http("PUT", "/_security/role/hub_reader", json.dumps({"cluster":["monitor"], "indices":[{"names":["events"], "privileges":["read","view_index_metadata"]}]}), ["Content-Type: application/json"])
   http("PUT", "/_security/user/reader", json.dumps({"password":READ, "roles":["hub_reader"]}), ["Content-Type: application/json"])
  else:
   http("PUT", "/_plugins/_security/api/roles/hub_reader", json.dumps({"cluster_permissions":["cluster_monitor"], "index_permissions":[{"index_patterns":["events"],"allowed_actions":["read","indices:admin/mappings/get"]}]}), ["Content-Type: application/json"])
   http("PUT", "/_plugins/_security/api/internalusers/reader", json.dumps({"password":"Copper-Pebble-River-2026!"}), ["Content-Type: application/json"])
   http("PUT", "/_plugins/_security/api/rolesmapping/hub_reader", json.dumps({"users":["reader"]}), ["Content-Type: application/json"])
  password = READ if kind == "elasticsearch" else "Copper-Pebble-River-2026!"
  ca = (OUT/"tls"/kind/"ca.crt").read_text()
  source = {"kind":kind,"host":name,"port":9200,"database":"events","username":"reader","password":password,"tls_mode":"verify","ca_cert":ca}
  base={"object":"events","body":{"query":{"match_all":{}},"sort":[{"id":"asc"}]}}
  return {"name":kind,"source":source,"namespace":"events","object":"events","baseline":base,"queries":[
   query({"object":"events", "body":{"size":0,"aggs":{"total":{"sum":{"field":"amount"}}}}}, 0, "37.5", truncated=False),
   query({"object":"events", "body":{"size":1,"sort":[{"id":"asc"}]}}, 1),
   query({"object":"events", "max_rows":1, "body":{"query":{"match_all":{}}}}, 1, truncated=True),
   query({"object":"events", "body":{"size":-1}}, 0, error=True),
   query(base,3,"9007199254740993"),query({"object":"events","body":{"query":{"term":{"id":1}},"aggs":{"amount":{"sum":{"field":"amount"}}}}},1,"aggregations"),query({"object":"events","operation":"get","query":"1"},1),query({"object":"events","operation":"count","body":{"query":{"match_all":{}}}},1),query({"object":"events","body":{"query":{"term":{"id":99}}}},0),query({"object":"events","body":{"query":{"bad":{}}}},0,error=True)],"denied":[{"object":"../_delete_by_query","body":{}},{"object":"events","operation":"delete"},{"object":"events","body":{"script_fields":{"x":{"script":"1"}}}}]}
 if kind=="neo4j":
  def cypher(q):return docker("exec","-i",name,"cypher-shell","-u","neo4j","-p",ADMIN,"--format","plain",data=q)
  wait(lambda:cypher("RETURN 1;"));cypher("CREATE (:Event {id:1,amount:12.5,big_num:9007199254740993}),(:Event {id:2,amount:20.25}),(:Event {id:3,amount:4.0}); MATCH (a:Event {id:1}),(b:Event {id:2}) CREATE (a)-[:NEXT]->(b);")
  return {"name":kind,"source":{"kind":kind,"host":name,"port":7687,"database":"neo4j","username":"neo4j","password":ADMIN,"tls_mode":"disable"},"namespace":"neo4j","object":"Event","baseline":{"query":"MATCH (n:Event) RETURN n ORDER BY n.id"},"queries":[
   query({"query":"RETURN $x + 1 AS value", "named_params":{"x":2.5}}, 1, data=[[3.5]]),
   query({"query":"RETURN $x.values[0] + $x.values[1] AS value", "named_params":{"x":{"values":[9007199254740993,2]}}}, 1, data=[["9007199254740995"]]),
   query({"query":"RETURN $x AS value", "named_params":{"x":9223372036854775808}}, 0, error=True),
   query({"query":"MATCH (n:Event) WHERE n.id >= $min WITH n ORDER BY n.id RETURN n, sum(n.amount) AS total","named_params":{"min":1}},3,"9007199254740993"),query({"query":"MATCH p=(a:Event)-[r:NEXT]->(b:Event) WHERE a.id=$id RETURN p,r","named_params":{"id":1}},1,"relationship"),query({"query":"MATCH (n:Event) WHERE n.id=99 RETURN n"},0),query({"query":"MATCH (n:Event RETURN n"},0,error=True)],"denied":[{"query":"MATCH (n:Event) DELETE n"},{"query":"CALL dbms.listConfig()"},{"query":"RETURN 1; CREATE ()"}]}
 if kind in ("cassandra","scylladb"):
  def cql(q):return docker("exec","-i",name,"cqlsh","-u","cassandra","-p","cassandra",data=q)
  wait(lambda:cql("SELECT release_version FROM system.local;"),240)
  cql("CREATE KEYSPACE hubtest WITH replication={'class':'NetworkTopologyStrategy','replication_factor':1}; CREATE TABLE hubtest.events(id bigint PRIMARY KEY,amount decimal,big_num bigint,note text,ratio float,weight double,scores frozen<list<int>>,attributes frozen<map<text,decimal>>); INSERT INTO hubtest.events(id,amount,big_num,note) VALUES(1,12.50,9007199254740993,'one'); INSERT INTO hubtest.events(id,amount,big_num,note) VALUES(2,20.25,4,'two'); INSERT INTO hubtest.events(id,amount,big_num,note) VALUES(3,4.00,5,'three'); UPDATE hubtest.events SET ratio=2.5,weight=3.75,scores=[1,2],attributes={'cost':12.50} WHERE id=1; CREATE ROLE reader WITH PASSWORD='"+READ+"' AND LOGIN=true; GRANT SELECT ON KEYSPACE hubtest TO reader;")
  if kind=="scylladb":cql("GRANT SELECT ON TABLE system.versions TO reader;")
  return {"name":kind,"source":{"kind":kind,"host":name,"port":9042,"database":"hubtest","username":"reader","password":READ,"tls_mode":"disable"},"namespace":"hubtest","object":"events","baseline":{"query":"SELECT * FROM events"},"queries":[
   query({"query":"SELECT amount FROM events WHERE id=? AND amount=? ALLOW FILTERING", "params":[1,12.5]}, 1, data=[["12.50"]]),
   query({"query":"SELECT ratio,weight FROM events WHERE id=? AND ratio=? AND weight=? ALLOW FILTERING", "params":[1,2.5,3.75]}, 1, data=[[2.5,3.75]]),
   query({"query":"SELECT scores,attributes FROM events WHERE id=? AND scores=? ALLOW FILTERING", "params":[1,[1,2]]}, 1, data=[[["1","2"],{"cost":"12.50"}]]),
   query({"query":"SELECT id FROM events WHERE id=? AND attributes=? ALLOW FILTERING", "params":[1,{"cost":12.5}]}, 1, data=[["1"]]),
   query({"query":"SELECT id,amount,big_num,ttl(note),writetime(note) FROM events WHERE id=?","params":[1]},1,"9007199254740993"),query({"query":"SELECT * FROM events WHERE id=?","params":[99]},0),query({"query":"SELECT missing FROM events"},0,error=True)],"denied":[{"query":"DELETE FROM events WHERE id=1"},{"query":"BEGIN BATCH INSERT INTO events(id) VALUES(9); APPLY BATCH"},{"query":"SELECT * FROM events; DELETE FROM events WHERE id=1"},{"query":"SELECT dangerous_function(id) FROM events"}]}
 if kind.startswith("influxdb"):
  v=kind[-1];port=8181 if v=="3" else 8086
  headers=["Content-Type: application/json"]
  if v=="1":
   def inf(q):return docker("exec",name,"influx","-username","admin","-password",ADMIN,"-execute",q)
   wait(lambda:inf("SHOW DATABASES"));inf("CREATE USER reader WITH PASSWORD '"+READ+"'; GRANT READ ON hubtest TO reader")
   import base64
   http("POST","/write?db=hubtest","events,region=east amount=12.5,big_num=9007199254740993i\nevents,region=west amount=20.25,big_num=4i\nevents,region=north amount=4.0,big_num=5i",["Authorization: Basic "+base64.b64encode(("admin:"+ADMIN).encode()).decode()],port)
   s={"username":"reader","password":READ};baseline={"language":"influxql","query":"SELECT * FROM events ORDER BY time"};param={"language":"influxql","query":"SELECT mean(amount) FROM events WHERE region=$region GROUP BY region","named_params":{"region":"east"}};empty={"query":"SELECT * FROM events WHERE region='missing'"};denied=[{"query":"SELECT * INTO copy FROM events"},{"query":"DROP DATABASE hubtest"}]
  elif v=="2":
   auth=["Authorization: Token "+ADMIN];wait(lambda:http("GET","/health",port=port))
   # Setup finishes after /health becomes available.
   wait(lambda:http("GET","/api/v2/buckets?name=hubtest",headers=auth,port=port));buckets=json.loads(http("GET","/api/v2/buckets?name=hubtest",headers=auth,port=port));bucket=buckets["buckets"][0];org=bucket["orgID"]
   readonly=json.loads(http("POST","/api/v2/authorizations",json.dumps({"orgID":org,"permissions":[{"action":"read","resource":{"type":"buckets","id":bucket["id"],"orgID":org}}]}),auth+headers,port))["token"]
   http("POST","/api/v2/write?org=hub&bucket=hubtest","events,region=east amount=12.5,big_num=9007199254740993i\nevents,region=west amount=20.25,big_num=4i\nevents,region=north amount=4.0,big_num=5i",auth,port)
   s={"token":readonly,"options":{"org":"hub","bucket":"hubtest"}};baseline={"language":"flux","query":'from(bucket:"hubtest") |> range(start:-1h) |> filter(fn:(r)=>r._field == "amount") |> keep(columns:["_time","_value","region"]) |> group() |> sort(columns:["region"])'};param={"language":"flux","query":'from(bucket:"hubtest") |> range(start:-1h) |> filter(fn:(r)=>r._field == "amount" and r.region == params.region) |> mean()',"named_params":{"region":"east"}};empty={"language":"flux","query":'from(bucket:"hubtest") |> range(start:-1h) |> filter(fn:(r)=>r.region == "missing")'};denied=[{"query":'from(bucket:"hubtest") |> range(start:-1h) |> to(bucket:"copy")'},{"query":'import "http" http.get(url:"http://evil")'}]
  else:
   def create_token():
    p=docker("exec",name,"influxdb3","create","token","--admin","--format","json")
    try:return json.loads(p.stdout)["token"]
    except (ValueError,KeyError):raise RuntimeError("token initialization is not ready")
   token=wait(create_token)
   auth=["Authorization: Bearer "+token]
   http("POST","/api/v3/write_lp?db=hubtest","events,region=east amount=12.5,big_num=9007199254740993i\nevents,region=west amount=20.25,big_num=4i\nevents,region=north amount=4.0,big_num=5i",auth,port)
   s={"token":token};baseline={"language":"sql","query":"SELECT * FROM events ORDER BY region"};param={"language":"sql","query":"SELECT region,sum(amount) AS total FROM events WHERE region=$region GROUP BY region","named_params":{"region":"east"}};empty={"query":"SELECT * FROM events WHERE region='missing'"};denied=[{"query":"DELETE FROM events"},{"query":"SELECT 1; DELETE FROM events"},{"language":"influxql","query":"SELECT * INTO copy FROM events"}]
  source={"kind":"influxdb","version":v,"host":name,"port":port,"database":"hubtest","tls_mode":"disable",**s}
  checks=[query(baseline,3),query(param,1,"12.5"),query(empty,0),query({"query":"SELECT FROM"},0,error=True)]
  if v!="2":checks[0]["contains"]="9007199254740993"
  if v=="2":
   checks.append(query({"language":"flux","query":'from(bucket:"hubtest") |> range(start:-1h) |> filter(fn:(r)=>r.region == params.region)',"named_params":{"region":'${die(msg:"must remain literal")}' }},0))
  if v=="3":checks.append(query({"language":"influxql","query":"SELECT mean(amount) FROM events WHERE region=$region","named_params":{"region":"east"}},1,"12.5"))
  return {"name":kind,"source":source,"namespace":"hubtest","object":"events","baseline":baseline,"queries":checks,"denied":denied}
 raise RuntimeError("unknown fixture "+kind)
def implementation_digest():
 paths = [p for base in (ROOT/"cmd", ROOT/"internal") for p in base.rglob("*.go") if not p.name.endswith("_test.go")]
 paths.extend([ROOT/"go.mod", ROOT/"go.sum"])
 digest = hashlib.sha256()
 for path in sorted(paths):
  digest.update(str(path.relative_to(ROOT)).encode()+b"\0"+path.read_bytes())
 return digest.hexdigest()

def main():
    source_digest = implementation_digest()
    selected = sys.argv[1:] or list(IMAGES) + ["sqlite", "duckdb"]
    for package in ("adapter", "server"):
        docker("exec", "mcpdbhub-dev", "go", "test", "-c", "./internal/"+package,
               "-o", "/work/artifacts/"+package+".test")
    failures = []
    for kind in selected:
        report = OUT / "results" / (kind+".json")
        report.unlink(missing_ok=True)
        name = "mcpdbhub-it-"+kind
        created = False
        local = kind in ("sqlite", "duckdb")
        print("Testing", kind, flush=True)
        try:
            environment = ["-e", "MCPDBHUB_MATRIX_REPORT=/work/artifacts/matrix/results"]
            if local:
                adapter_test = "TestLocalDatabaseReadOnly/"+kind+"$"
                server_test = "TestDuckDBMCPReadOnly$" if kind == "duckdb" else "TestLifecycleAuthorizationReadOnlyAndPersistence$"
            else:
                provision(kind, name)
                created = True
                fixture = seed(kind, name)
                (OUT/(kind+"-fixture.json")).write_text(json.dumps([fixture], indent=2))
                environment += ["-e", "MCPDBHUB_MATRIX=/work/artifacts/matrix/"+kind+"-fixture.json"]
                adapter_test, server_test = "TestDatabaseMatrix$", "TestDatabaseMCPMatrix$"
            for package, test in (("adapter", adapter_test), ("server", server_test)):
                result = docker("exec", *environment, "mcpdbhub-dev", "/work/artifacts/"+package+".test",
                                "-test.run="+test, "-test.v", check=False)
                suffix = ".log" if package == "adapter" else "-mcp.log"
                (OUT/(kind+suffix)).write_text(result.stdout+result.stderr)
                print(result.stdout+result.stderr, flush=True)
                if result.returncode:
                    raise RuntimeError(package+" acceptance failed")
            if source_digest != implementation_digest():
                raise RuntimeError("implementation changed during verification; rerun")
            result = json.loads(report.read_text())
            result["checks"].extend(["mcp_http_queries", "mcp_discovery", "mcp_limits_and_pagination"])
            if not local:
                result["checks"].extend(["agent_isolation", "mcp_audit", "token_revocation"])
                result["image_id"] = docker("inspect", "--format", "{{.Image}}", name).stdout.strip()
            result["implementation_sha256"] = source_digest
            result["environment"] = "OrbStack Linux arm64"
            if kind in ("elasticsearch", "opensearch"):
                result["checks"].extend(["product_tls_verified", "untrusted_certificate_denied", "database_read_account_write_denied"])
            report.write_text(json.dumps(result, indent=2)+"\n")
        except Exception as error:
            report.unlink(missing_ok=True)
            print(kind, str(error), flush=True)
            failures.append(kind)
        finally:
            if created:
                logs = docker("logs", "--tail", "120", name, check=False)
                (OUT/(kind+"-server.log")).write_text(logs.stdout+logs.stderr)
                if not os.getenv("MCPDBHUB_KEEP_FIXTURES"):
                    docker("rm", "-f", name, check=False)
    print("FAILED:", ", ".join(failures) or "none")
    return bool(failures)

if __name__ == "__main__":
    sys.exit(main())
