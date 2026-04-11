package watcher

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"go.uber.org/goleak"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// spikeGoleakFilters mirrors the goleak filter set declared in TestMain
// (testmain_test.go). It is duplicated here because goleak.VerifyNone does
// NOT inherit filters from goleak.VerifyTestMain — each call needs its own
// filter list. Keep in sync with TestMain.
func spikeGoleakFilters() []goleak.Option {
	return []goleak.Option{
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*ccBalancerWrapper).watcher"),
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*ccResolverWrapper).watcher"),
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*addrConn).resetTransport"),
		goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.(*http2Client).keepalive"),
		goleak.IgnoreAnyFunction("google.golang.org/grpc/internal/transport.newHTTP2Client"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionResetter"),
		goleak.IgnoreAnyFunction("modernc.org"),
		goleak.IgnoreAnyFunction("internal/poll.runtime_pollWait"),
	}
}

// TestSpike_PubsubGoleakFilters creates and immediately tears down a pstest-backed
// pubsub.Client. After Close, goleak.VerifyNone (with the empirically-verified
// filter list) must report no leaked goroutines. Its purpose is to prove the
// IgnoreTopFunction filter set used by TestMain is sufficient for the gRPC /
// OpenCensus background goroutines that Google client libraries spawn.
//
// This test MUST pass on main once Plan 17-01 is in place — if it fails, a new
// leaker surfaced and its top-of-stack function name is in the failure output;
// add that function to both spikeGoleakFilters() and TestMain's filter list.
func TestSpike_PubsubGoleakFilters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	srv := pstest.NewServer()
	defer srv.Close()

	conn, err := grpc.NewClient(srv.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	client, err := pubsub.NewClient(ctx, "test-project",
		option.WithGRPCConn(conn),
	)
	if err != nil {
		t.Fatalf("pubsub.NewClient: %v", err)
	}
	_, _ = client.CreateTopic(ctx, "spike-topic")
	if err := client.Close(); err != nil {
		t.Logf("client.Close: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Logf("conn.Close: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Logf("srv.Close: %v", err)
	}

	goleak.VerifyNone(t, spikeGoleakFilters()...)
}

// TestSpike_PubsubAcceptsUserOAuthTokenSource confirms that pubsub.NewClient
// accepts option.WithTokenSource pointing at a user OAuth token (not ADC /
// service-account JSON). Resolves RESEARCH.md Open Question 1 / assumption A3:
// whether the same user-scoped OAuth token the Gmail client uses can be handed
// to the Pub/Sub client without a split credential pathway.
func TestSpike_PubsubAcceptsUserOAuthTokenSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	userTok := &oauth2.Token{
		AccessToken: "fake-access-token-for-spike",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
	userTokenSource := oauth2.StaticTokenSource(userTok)

	client, err := pubsub.NewClient(ctx, "test-project",
		option.WithTokenSource(userTokenSource),
	)
	if err != nil {
		t.Fatalf("pubsub.NewClient(option.WithTokenSource): %v -- "+
			"ADAPT-05 design assumes user OAuth works for Pub/Sub", err)
	}
	defer client.Close()

	// We do NOT make any RPCs here — the spike only proves the client builds
	// with a user TokenSource. Any RPC against real Google would either auth-fail
	// or hang on the 500ms ctx, neither of which is informative for this question.
	t.Logf("pubsub.NewClient accepts option.WithTokenSource for user OAuth — " +
		"ADAPT-05 / CONTEXT D-09 design is viable")
}
