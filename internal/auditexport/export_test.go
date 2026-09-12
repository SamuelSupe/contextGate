package auditexport

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

func waitView(t *testing.T, m *Manager, predicate func(View) bool) View {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		v, err := m.View(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if predicate(v) {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	v, _ := m.View(context.Background())
	t.Fatalf("export did not reach expected state: %+v", v.Status)
	return v
}
func openManager(t *testing.T, dir string) (*store.Store, *Manager) {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(st)
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(); st.Close() })
	return st, m
}
func configure(t *testing.T, m *Manager, c Config) {
	t.Helper()
	if err := m.Update(context.Background(), Update{Config: c}); err != nil {
		t.Fatal(err)
	}
}
func addAudit(t *testing.T, st *store.Store, id string) {
	t.Helper()
	if err := st.Audit(model.Audit{RequestID: id, At: time.UnixMilli(1750000000123), AgentID: "agent-reader", SourceID: "source-orders", Operation: "query_sql", Fingerprint: "fingerprint", ElapsedMS: 12, Rows: 3}); err != nil {
		t.Fatal(err)
	}
}
func records(request *collectorpb.ExportLogsServiceRequest) []*logspb.LogRecord {
	return request.ResourceLogs[0].ScopeLogs[0].LogRecords
}
func requestIDs(request *collectorpb.ExportLogsServiceRequest) []string {
	var ids []string
	for _, r := range records(request) {
		for _, a := range r.Attributes {
			if a.Key == "mcpdbhub.audit.request_id" {
				ids = append(ids, a.Value.GetStringValue())
			}
		}
	}
	return ids
}

func TestHTTPJournalRetryRestartAndDisabledBoundary(t *testing.T) {
	var attempts atomic.Int32
	var unavailable atomic.Bool
	var mu sync.Mutex
	var received []*collectorpb.ExportLogsServiceRequest
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/logs" || r.Header.Get("Authorization") != "Bearer export-secret" || r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Error("wrong OTLP wire request")
		}
		body, _ := io.ReadAll(r.Body)
		req := new(collectorpb.ExportLogsServiceRequest)
		if err := proto.Unmarshal(body, req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if attempts.Add(1) == 1 || unavailable.Load() {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(503)
			return
		}
		mu.Lock()
		received = append(received, req)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer receiver.Close()
	dir := filepath.Join(t.TempDir(), "config")
	st, m := openManager(t, dir)
	addAudit(t, st, "before-enable")
	configure(t, m, Config{Enabled: true, Endpoint: receiver.URL, Headers: map[string]string{"Authorization": "Bearer export-secret"}})
	addAudit(t, st, "first")
	v := waitView(t, m, func(v View) bool { return v.Status.Accepted == 1 })
	if attempts.Load() != 2 || v.Status.Pending != 0 || !v.HeadersConfigured || len(v.Config.Headers) != 0 {
		t.Fatal("retry or header-redaction contract failed", v)
	}
	raw, err := st.Get(stateKey)
	if err != nil || strings.Contains(raw, "export-secret") {
		t.Fatal("export settings not encrypted", err)
	}
	unavailable.Store(true)
	addAudit(t, st, "pending-before-restart")
	waitView(t, m, func(v View) bool { return v.Status.State == "retrying" && v.Status.Pending == 1 })
	m.Close()
	st.Close()
	unavailable.Store(false)
	st, m = openManager(t, dir)
	addAudit(t, st, "after-restart")
	v = waitView(t, m, func(v View) bool { return v.Status.Accepted == 3 })
	v.Config.Enabled = false
	configure(t, m, v.Config)
	addAudit(t, st, "while-disabled")
	v, _ = m.View(context.Background())
	v.Config.Enabled = true
	configure(t, m, v.Config)
	addAudit(t, st, "after-reenable")
	waitView(t, m, func(v View) bool { return v.Status.Accepted == 4 })
	mu.Lock()
	defer mu.Unlock()
	var ids []string
	instance := ""
	for _, req := range received {
		ids = append(ids, requestIDs(req)...)
		for _, a := range req.ResourceLogs[0].Resource.Attributes {
			if a.Key == "service.instance.id" {
				if instance != "" && a.Value.GetStringValue() != instance {
					t.Fatal("instance ID changed across restart")
				}
				instance = a.Value.GetStringValue()
			}
		}
		for _, r := range records(req) {
			if r.TimeUnixNano != uint64(time.UnixMilli(1750000000123).UnixNano()) || r.SeverityNumber != logspb.SeverityNumber_SEVERITY_NUMBER_INFO {
				t.Fatal("audit timestamp/severity lost")
			}
		}
	}
	if strings.Join(ids, ",") != "first,pending-before-restart,after-restart,after-reenable" {
		t.Fatal("unexpected replay or lost events", ids)
	}
}

func TestPartialAndPermanentRejection(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial", true: "permanent"}[permanent], func(t *testing.T) {
			var bad atomic.Bool
			bad.Store(true)
			var attempts atomic.Int32
			receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/x-protobuf")
				if bad.Load() {
					if permanent {
						w.WriteHeader(401)
						io.WriteString(w, "private receiver diagnostic: secret")
						return
					}
					b, _ := proto.Marshal(&collectorpb.ExportLogsServiceResponse{PartialSuccess: &collectorpb.ExportLogsPartialSuccess{RejectedLogRecords: 1, ErrorMessage: "private receiver diagnostic: secret"}})
					w.Write(b)
				}
			}))
			defer receiver.Close()
			st, m := openManager(t, t.TempDir())
			configure(t, m, Config{Enabled: true, Endpoint: receiver.URL})
			addAudit(t, st, "rejected")
			v := waitView(t, m, func(v View) bool { return v.Status.Rejected == 1 })
			if strings.Contains(v.Status.LastError, "secret") {
				t.Fatal("receiver diagnostics leaked")
			}
			if (v.Status.State == "blocked") != permanent {
				t.Fatal("wrong failure state", v.Status)
			}
			bad.Store(false)
			addAudit(t, st, "next")
			if permanent {
				configure(t, m, v.Config)
			}
			waitView(t, m, func(v View) bool { return v.Status.Accepted == 1 })
			if attempts.Load() != 2 {
				t.Fatal("rejected batch retried", attempts.Load())
			}
			rows, err := st.Audits(store.AuditFilter{}, 10)
			if err != nil || len(rows) != 2 {
				t.Fatal("local audits lost", err)
			}
		})
	}
}

func TestReconfigureCancelsOldDelivery(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		cancelled <- struct{}{}
	}))
	defer old.Close()
	received := make(chan *collectorpb.ExportLogsServiceRequest, 1)
	next := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		req := new(collectorpb.ExportLogsServiceRequest)
		proto.Unmarshal(b, req)
		received <- req
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer next.Close()
	st, m := openManager(t, t.TempDir())
	configure(t, m, Config{Enabled: true, Endpoint: old.URL})
	addAudit(t, st, "in-flight")
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("delivery did not start")
	}
	v, _ := m.View(context.Background())
	v.Config.Endpoint = next.URL
	configure(t, m, v.Config)
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("old delivery not cancelled")
	}
	select {
	case req := <-received:
		if strings.Join(requestIDs(req), ",") != "in-flight" {
			t.Fatal("backlog lost")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("new destination did not receive backlog")
	}
	waitView(t, m, func(v View) bool { return v.Status.Accepted == 1 })
}

func TestHTTPTrustRedirectAndResponseValidation(t *testing.T) {
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/x-protobuf") }))
	defer receiver.Close()
	_, m := openManager(t, t.TempDir())
	update := Update{Config: Config{Endpoint: receiver.URL}}
	result, err := m.Test(context.Background(), update)
	if err != nil || result.Accepted {
		t.Fatal("untrusted certificate accepted", err)
	}
	update.CAPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: receiver.Certificate().Raw}))
	result, err = m.Test(context.Background(), update)
	if err != nil || !result.Accepted {
		t.Fatal("trusted certificate rejected", result, err)
	}
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, kind := range []string{"redirect", "html", "oversized", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "redirect":
					http.Redirect(w, r, target.URL, 307)
				case "html":
					w.Header().Set("Content-Type", "text/html")
					io.WriteString(w, "<html>OK</html>")
				case "oversized":
					w.Header().Set("Content-Type", "application/x-protobuf")
					io.WriteString(w, strings.Repeat("x", maxResponseBytes+1))
				case "malformed":
					w.Header().Set("Content-Type", "application/x-protobuf")
					w.Write([]byte{255})
				}
			}))
			defer bad.Close()
			result, err := m.Test(context.Background(), Update{Config: Config{Endpoint: bad.URL, Headers: map[string]string{"Authorization": "Bearer private"}}})
			if err != nil || result.Accepted {
				t.Fatal("invalid response accepted", kind, result, err)
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("authorization header could follow redirect")
	}
}

type grpcReceiver struct {
	collectorpb.UnimplementedLogsServiceServer
	received  chan *collectorpb.ExportLogsServiceRequest
	failure   codes.Code
	retryInfo bool
}

func (r *grpcReceiver) Export(ctx context.Context, req *collectorpb.ExportLogsServiceRequest) (*collectorpb.ExportLogsServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("authorization")) != 1 || md.Get("authorization")[0] != "Bearer grpc-secret" {
		return nil, status.Error(codes.Unauthenticated, "private diagnostic")
	}
	if r.failure != codes.OK {
		failure := status.New(r.failure, "private diagnostic")
		if r.retryInfo {
			failure, _ = failure.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(time.Second)})
		}
		return nil, failure.Err()
	}
	r.received <- proto.Clone(req).(*collectorpb.ExportLogsServiceRequest)
	return &collectorpb.ExportLogsServiceResponse{}, nil
}
func TestGRPCWireAndThrottling(t *testing.T) {
	for _, mode := range []string{"success", "retry", "reject"} {
		t.Run(mode, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			receiver := &grpcReceiver{received: make(chan *collectorpb.ExportLogsServiceRequest, 2)}
			if mode != "success" {
				receiver.failure = codes.ResourceExhausted
				receiver.retryInfo = mode == "retry"
			}
			server := grpc.NewServer()
			collectorpb.RegisterLogsServiceServer(server, receiver)
			go server.Serve(listener)
			defer server.Stop()
			config, err := normalize(Config{Enabled: true, Protocol: "grpc", Endpoint: "http://" + listener.Addr().String(), Headers: map[string]string{"authorization": "Bearer grpc-secret"}})
			if err != nil {
				t.Fatal(err)
			}
			client, err := newSender(config)
			if err != nil {
				t.Fatal(err)
			}
			defer client.close()
			result := client.export(context.Background(), testRequest(config, "instance"))
			if mode == "success" {
				if result.error != "" {
					t.Fatal(result.error)
				}
				req := <-receiver.received
				if len(records(req)) != 1 || req.ResourceLogs[0].Resource.Attributes[0].Value.GetStringValue() != "mcpdbhub" {
					t.Fatal("gRPC log payload missing")
				}
			} else if result.retry != (mode == "retry") || strings.Contains(result.error, "private") {
				t.Fatal("gRPC throttling or redaction failed", result)
			}
		})
	}
}

func TestExportBoundsCallerFields(t *testing.T) {
	st, m := openManager(t, t.TempDir())
	if err := st.Audit(model.Audit{At: time.Now(), SourceID: strings.Repeat("界", 100000), Operation: "query_sql", OntologyID: "commerce", OntologyVersion: "2"}); err != nil {
		t.Fatal(err)
	}
	audits, err := st.AuditBatch(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	req, count := auditRequest(m.state.Config, "instance", audits)
	if count != 1 || proto.Size(req) > 512<<10 {
		t.Fatal("unbounded export")
	}
	b, _ := json.Marshal(req)
	if !strings.Contains(string(b), "mcpdbhub.audit.ontology_id") || !strings.Contains(string(b), "commerce") || !strings.Contains(string(b), "mcpdbhub.audit.ontology_version") {
		t.Fatal("ontology correlation missing from OTLP")
	}
	if !strings.Contains(string(b), "attributes_truncated") || strings.Contains(string(b), strings.Repeat("界", 2049)) {
		t.Fatal("unbounded caller field")
	}
}
