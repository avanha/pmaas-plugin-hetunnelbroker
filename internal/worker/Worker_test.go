package worker

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
)

const TunnelResponse = `
<tunnels>
  <tunnel id="12345">
    <description>DESCRIPTION</description>
    <serverv4>111.111.111.111</serverv4>
    <clientv4>222.222.222.222</clientv4>
    <serverv6>2001:0db8:85a3:0000:0000:8a2e:0370:7334</serverv6>
    <clientv6>2001:0db8:85a3:0000:0000:8a2e:0370:7335</clientv6>
    <routed64>2001:0cc8:124b:125c::/64</routed64>
    <routed48>2001:0cc8:2345::/48</routed48>
    <rdns1>333.333.333.333</rdns1>
    <rdns2>444.444.444.444</rdns2>
  </tunnel>
</tunnels>
`

// fakeHttpClient is a mock implementation of HttpClient for testing
type fakeHttpClient struct {
	lastRequestedURL string
	responseBody     string
	responseStatus   int
	responseErr      error
}

func (f *fakeHttpClient) Get(url string) (*http.Response, error) {
	f.lastRequestedURL = url
	if f.responseErr != nil {
		return nil, f.responseErr
	}

	resp := &http.Response{
		StatusCode: f.responseStatus,
		Body:       io.NopCloser(strings.NewReader(f.responseBody)),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// blockingFakeHttpClient is a mock that blocks on Get() until signaled.
// This lets the test cancel the context while a request is in-flight.
type blockingFakeHttpClient struct {
	fakeHttpClient
	gate chan struct{} // close this to unblock Get()
}

func (b *blockingFakeHttpClient) Get(url string) (*http.Response, error) {
	<-b.gate // block until the test signals
	return b.fakeHttpClient.Get(url)
}

func TestNewWorker(t *testing.T) {
	reqCh := make(chan common.BrokerRequest)
	w := NewWorker(reqCh, "test_user", "test_key")

	if w.username != "test_user" {
		t.Errorf("expected username test_user, got %s", w.username)
	}
	if w.updateKey != "test_key" {
		t.Errorf("expected updateKey test_key, got %s", w.updateKey)
	}
	if w.requestCh != reqCh {
		t.Error("expected requestCh to match")
	}
}

// TestWorker_Run_ContextCancellation verifies that when the context is canceled while
// a request is in-flight, any remaining queued requests are canceled with "request canceled".
func TestWorker_Run_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reqCh := make(chan common.BrokerRequest, 10)
		w := NewWorker(reqCh, "user", "key")

		// Create a blocking HTTP client so the first request stays in-flight
		gate := make(chan struct{})
		blockingClient := &blockingFakeHttpClient{
			fakeHttpClient: fakeHttpClient{
				responseStatus: 200,
				responseBody:   TunnelResponse,
			},
			gate: gate,
		}
		w.httpClient = blockingClient

		ctx, cancel := context.WithCancel(context.Background())

		runDone := make(chan struct{})
		go func() {
			w.Run(ctx)
			close(runDone)
		}()

		// First request: this will be picked up by the worker and block inside processRequest
		resultCh1 := make(chan common.BrokerResult, 1)
		reqCh <- common.BrokerRequest{
			RequestType: common.BrokerRequestTypeGetTunnelInfo,
			ResultCh:    resultCh1,
		}

		// Let the worker goroutine pick up the first request (it will block on the HTTP call)
		synctest.Wait()

		// Cancel the context while the first request is still blocked
		cancel()

		// Unblock the first request so the worker can see the canceled context and exit
		close(gate)

		// Wait for the worker to select Done or RequestCh.  Since RequestCh is empty, it can only
		// select Done.
		synctest.Wait()

		// Now write the second request while the worker is running the drain loop.
		resultCh2 := make(chan common.BrokerResult, 1)
		reqCh <- common.BrokerRequest{
			RequestType: common.BrokerRequestTypeGetTunnelInfo,
			ResultCh:    resultCh2,
		}

		// Close the channel so the drain loop can exit after processing remaining items
		close(reqCh)

		// Wait for the worker to finish
		synctest.Wait()
		<-runDone

		// The first request should have completed successfully (it was already being processed)
		select {
		case res := <-resultCh1:
			if res.Error != nil {
				t.Errorf("expected first request to succeed, got error: %v", res.Error)
			}
		default:
			t.Error("expected a result on the first result channel")
		}

		// The second request should have been canceled during the drain
		select {
		case res := <-resultCh2:
			if res.Error == nil || res.Error.Error() != "request cancelled" {
				t.Errorf("expected request cancelled error, got %v", res.Error)
			}
		default:
			t.Error("expected a result on the second result channel")
		}
	})
}

func TestWorker_cancelRequest(t *testing.T) {
	w := NewWorker(nil, "", "")
	resultCh := make(chan common.BrokerResult, 1)
	req := common.BrokerRequest{
		ResultCh: resultCh,
	}

	w.cancelRequest(&req)

	select {
	case res := <-resultCh:
		if res.Error == nil || res.Error.Error() != "request cancelled" {
			t.Errorf("expected request cancelled error, got %v", res.Error)
		}
	default:
		t.Error("expected a result on the result channel")
	}
}

func TestWorker_processGetTunnelInfoRequest(t *testing.T) {
	reqCh := make(chan common.BrokerRequest, 1)
	w := NewWorker(reqCh, "test_user", "test_key")

	// Create and inject a fake HttpClient
	fakeClient := &fakeHttpClient{
		responseStatus: 200,
		responseBody:   TunnelResponse,
	}
	w.httpClient = fakeClient

	resultCh := make(chan common.BrokerResult, 1)
	req := common.BrokerRequest{
		RequestType: common.BrokerRequestTypeGetTunnelInfo,
		GetTunnelInfoRequest: common.GetTunnelInfoRequest{
			TunnelId: "12345",
		},
		ResultCh: resultCh,
	}

	w.processRequest(&req)

	select {
	case res := <-resultCh:
		// Verify no error occurred
		if res.Error != nil {
			t.Errorf("expected no error, got %v", res.Error)
		}

		// Verify that the HTTP request was made to the correct URL
		expectedURL := "https://test_user:test_key@ipv4.tunnelbroker.net/tunnelInfo.php?tid=12345"

		if fakeClient.lastRequestedURL != expectedURL {
			t.Errorf("expected URL %s, got %s", expectedURL, fakeClient.lastRequestedURL)
		}

		// Verify the response was parsed correctly
		if res.TunnelData.Description != "DESCRIPTION" {
			t.Errorf("expected tunnel description DESCRIPTION, got %s", res.TunnelData.Description)
		}

		expectedClientV4IpAddress := net.ParseIP("222.222.222.222")

		if !res.TunnelData.ClientIpV4Address.Equal(expectedClientV4IpAddress) {
			t.Errorf("expected ClientIpV4Address %s, got %s", expectedClientV4IpAddress, res.TunnelData.ClientIpV4Address)
		}

		expectedServerV4IpAddress := net.ParseIP("111.111.111.111")

		if !res.TunnelData.ServerIpV4Address.Equal(expectedServerV4IpAddress) {
			t.Errorf("expected ServerIpV4Address %s, got %s", expectedServerV4IpAddress, res.TunnelData.ServerIpV4Address)
		}

		// Add tests for additional struct members

		expectedServerV6IpAddress := net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
		if !res.TunnelData.ServerIpV6Address.Equal(expectedServerV6IpAddress) {
			t.Errorf("expected ServerIpV6Address %s, got %s", expectedServerV6IpAddress, res.TunnelData.ServerIpV6Address)
		}

		expectedClientV6IpAddress := net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7335")
		if !res.TunnelData.ClientIpV6Address.Equal(expectedClientV6IpAddress) {
			t.Errorf("expected ClientIpV6Address %s, got %s", expectedClientV6IpAddress, res.TunnelData.ClientIpV6Address)
		}

		// Check Routed64Net
		_, expectedRouted64Net, _ := net.ParseCIDR("2001:0cc8:124b:125c::/64")
		if res.TunnelData.Routed64Net.String() != expectedRouted64Net.String() {
			t.Errorf("expected Routed64Net %s, got %s", expectedRouted64Net, res.TunnelData.Routed64Net)
		}

		// Check Routed48Net
		_, expectedRouted48Net, _ := net.ParseCIDR("2001:0cc8:2345::/48")
		if res.TunnelData.Routed48Net.String() != expectedRouted48Net.String() {
			t.Errorf("expected Routed48Net %s, got %s", expectedRouted48Net, res.TunnelData.Routed48Net)
		}

	default:
		t.Error("expected a result on the result channel")
	}
}
