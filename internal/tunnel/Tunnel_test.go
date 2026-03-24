package tunnel

import (
	"errors"
	"net"
	"testing"
	"testing/synctest"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	spi "github.com/avanha/pmaas-spi"
)

// mockContainer is a minimal implementation of spi.IPMAASContainer for testing
type mockContainer struct {
	spi.IPMAASContainer
}

func (m *mockContainer) EnqueueOnPluginGoRoutine(fn func()) error {
	fn()
	return nil
}

func TestNewTunnel(t *testing.T) {
	container := &mockContainer{}
	localId := "local-1"
	tunnelId := "tunnel-1"
	name := "Test Tunnel"

	tun := NewTunnel(container, nil, localId, tunnelId, name, nil)

	if tun.LocalId() != localId {
		t.Errorf("expected LocalId %s, got %s", localId, tun.LocalId())
	}
	if tun.Name() != name {
		t.Errorf("expected Name %s, got %s", name, tun.Name())
	}
	if tun.Data().Name != name {
		t.Errorf("expected Data().Name %s, got %s", name, tun.Data().Name)
	}
}

func TestTunnel_PmaasEntityId(t *testing.T) {
	tun := &Tunnel{localId: "local-1"}

	tun.SetPmaasEntityId("pmaas-1")
	if tun.PmaasEntityId() != "pmaas-1" {
		t.Errorf("expected PmaasEntityId pmaas-1, got %s", tun.PmaasEntityId())
	}

	tun.ClearPmaasEntityId()
	if tun.PmaasEntityId() != "" {
		t.Errorf("expected empty PmaasEntityId, got %s", tun.PmaasEntityId())
	}
}

func TestTunnel_Refresh_Success(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		container := &mockContainer{}
		expectedDesc := "My Test Tunnel Description"

		var capturedRequest common.BrokerRequest
		handlerFn := func(req common.BrokerRequest) error {
			capturedRequest = req
			return nil
		}

		/// The last nil arg is onEntityStubAvailableListeners
		tun := NewTunnel(container, handlerFn, "local-1", "12345", "Test", nil)

		err := tun.Refresh()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedRequest.RequestType != common.BrokerRequestTypeGetTunnelInfo {
			t.Errorf("expected request type %v, got %v", common.BrokerRequestTypeGetTunnelInfo, capturedRequest.RequestType)
		}

		// Simulate the worker processing the request and sending a result back
		go func() {
			capturedRequest.ResultCh <- common.BrokerResult{
				Error: nil,
				TunnelData: data.TunnelData{
					Description: expectedDesc,
				},
			}
		}()

		synctest.Wait()

		if tun.Data().Description != expectedDesc {
			t.Errorf("expected Description %s, got %s", expectedDesc, tun.Data().Description)
		}
		if tun.Data().GetSuccessCount != 1 {
			t.Errorf("expected GetSuccessCount 1, got %d", tun.Data().GetSuccessCount)
		}
	})
}

func TestTunnel_Refresh_Error(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		container := &mockContainer{}
		expectedErr := errors.New("api error")

		var capturedRequest common.BrokerRequest
		handlerFn := func(req common.BrokerRequest) error {
			capturedRequest = req
			return nil
		}

		tun := NewTunnel(container, handlerFn, "local-1", "12345", "Test", nil)

		_ = tun.Refresh()

		go func() {
			capturedRequest.ResultCh <- common.BrokerResult{
				Error: expectedErr,
			}
		}()

		synctest.Wait()

		if tun.Data().LastError != expectedErr {
			t.Errorf("expected LastError %v, got %v", expectedErr, tun.Data().LastError)
		}
		if tun.Data().GetErrorCount != 1 {
			t.Errorf("expected GetErrorCount 1, got %d", tun.Data().GetErrorCount)
		}
	})
}

func TestTunnel_UpdateClientIpV4Address_Success(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		container := &mockContainer{}
		newIP := net.ParseIP("192.168.1.1")

		var capturedRequest common.BrokerRequest
		handlerFn := func(req common.BrokerRequest) error {
			capturedRequest = req
			return nil
		}

		tun := NewTunnel(container, handlerFn, "local-1", "12345", "Test", nil)

		err := tun.UpdateClientIpV4Address(newIP)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedRequest.RequestType != common.BrokerRequestTypeUpdateTunnelClientIpV4Address {
			t.Errorf("expected request type %v, got %v", common.BrokerRequestTypeUpdateTunnelClientIpV4Address, capturedRequest.RequestType)
		}
		if !capturedRequest.UpdateTunnelClientIpV4AddressRequest.NewAddress.Equal(newIP) {
			t.Errorf("expected NewAddress %s, got %s", newIP, capturedRequest.UpdateTunnelClientIpV4AddressRequest.NewAddress)
		}

		go func() {
			capturedRequest.ResultCh <- common.BrokerResult{
				Error: nil,
				TunnelData: data.TunnelData{
					ClientIpV4Address: newIP,
				},
			}
		}()

		synctest.Wait()

		if !tun.Data().ClientIpV4Address.Equal(newIP) {
			t.Errorf("expected ClientIpV4Address %s, got %s", newIP.String(), tun.Data().ClientIpV4Address.String())
		}
		if tun.Data().UpdateSuccessCount != 1 {
			t.Errorf("expected UpdateSuccessCount 1, got %d", tun.Data().UpdateSuccessCount)
		}
	})
}
