package auditexport

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const maxResponseBytes = 64 << 10

type sender struct {
	config Config
	http   *http.Client
	grpc   *grpc.ClientConn
}

type delivery struct {
	response   *collectorpb.ExportLogsServiceResponse
	error      string
	retry      bool
	retryAfter time.Duration
}

func newSender(c Config) (*sender, error) {
	tlsConfig, err := tlsConfig(c.CAPEM)
	if err != nil {
		return nil, err
	}
	s := &sender{config: c}
	u, _ := url.Parse(c.Endpoint)
	if c.Protocol == "grpc" {
		var creds credentials.TransportCredentials = insecure.NewCredentials()
		if u.Scheme == "https" {
			creds = credentials.NewTLS(tlsConfig)
		}
		s.grpc, err = grpc.NewClient("dns:///"+u.Host, grpc.WithTransportCredentials(creds), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxResponseBytes), grpc.MaxCallSendMsgSize(1<<20)))
		if err != nil {
			return nil, err
		}
	} else {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = tlsConfig
		transport.MaxConnsPerHost = 1
		transport.MaxIdleConnsPerHost = 1
		s.http = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return s, nil
}

func (s *sender) close() {
	if s.http != nil {
		s.http.CloseIdleConnections()
	}
	if s.grpc != nil {
		s.grpc.Close()
	}
}

func (s *sender) export(ctx context.Context, request *collectorpb.ExportLogsServiceRequest) delivery {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if s.grpc != nil {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(s.config.Headers))
		response, err := collectorpb.NewLogsServiceClient(s.grpc).Export(ctx, request)
		if err == nil {
			return delivery{response: response}
		}
		st := status.Convert(err)
		d := delivery{error: "gRPC " + st.Code().String()}
		for _, detail := range st.Details() {
			if retry, ok := detail.(*errdetails.RetryInfo); ok && retry.RetryDelay != nil {
				d.retryAfter = retry.RetryDelay.AsDuration()
				if st.Code() == codes.ResourceExhausted {
					d.retry = true
				}
			}
		}
		switch st.Code() {
		case codes.Canceled, codes.DeadlineExceeded, codes.Aborted, codes.OutOfRange, codes.Unavailable, codes.DataLoss:
			d.retry = true
		}
		return d
	}
	body, err := proto.Marshal(request)
	if err != nil {
		return delivery{error: "Invalid OTLP payload"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return delivery{error: "Invalid OTLP endpoint"}
	}
	for key, value := range s.config.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "application/x-protobuf")
	res, err := s.http.Do(req)
	if err != nil {
		return delivery{error: "OTLP connection failed (network, TLS or timeout)", retry: true}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		d := delivery{error: "HTTP " + strconv.Itoa(res.StatusCode)}
		switch res.StatusCode {
		case 429, 502, 503, 504:
			d.retry = true
		}
		if value := res.Header.Get("Retry-After"); value != "" {
			if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds > 0 {
				d.retryAfter = time.Duration(seconds) * time.Second
			} else if until, err := http.ParseTime(value); err == nil {
				d.retryAfter = time.Until(until)
			}
		}
		return d
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return delivery{error: "OTLP response interrupted", retry: true}
	}
	if len(b) > maxResponseBytes {
		return delivery{error: "OTLP response exceeds 64 KiB"}
	}
	// Empty protobuf messages are a valid acknowledgement; HTML/JSON success pages are not.
	contentType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil || contentType != "application/x-protobuf" {
		return delivery{error: "Receiver did not return OTLP protobuf"}
	}
	response := new(collectorpb.ExportLogsServiceResponse)
	if err := proto.Unmarshal(b, response); err != nil {
		return delivery{error: "Invalid OTLP response"}
	}
	return delivery{response: response}
}
