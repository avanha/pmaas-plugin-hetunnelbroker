package worker

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/worker/messages"
)

type Worker struct {
	requestCh chan common.BrokerRequest
	username  string
	updateKey string
	err       atomic.Value
}

func NewWorker(requestCh chan common.BrokerRequest, usenrame string, updateKey string) *Worker {
	return &Worker{
		requestCh: requestCh,
		username:  usenrame,
		updateKey: updateKey,
	}
}

func (w *Worker) Run(ctx context.Context) {
	for run := true; run; {
		select {
		case <-ctx.Done():
			run = false
			if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
				w.err.Store(fmt.Errorf("heTunnelBroker received unexpected error from context: %w", ctx.Err()))
			}
			break
		case request := <-w.requestCh:
			w.processRequest(&request)
			break
		}
	}

	// Drain the request channel
	for request := range w.requestCh {
		w.cancelRequest(&request)
	}
}

func (w *Worker) processRequest(request *common.BrokerRequest) {
	fmt.Printf("%T Received request, type %d\n", w, request.RequestType)
	switch request.RequestType {
	case common.BrokerRequestTypeGetTunnelInfo:
		w.processGetTunnelInfoRequest(&request.GetTunnelInfoRequest, request.ResultCh)
		break
	case common.BrokerRequestTypeUpdateTunnelClientIpV4Address:
		w.processUpdateTunnelClientIpV4Address(&request.UpdateTunnelClientIpV4AddressRequest, request.ResultCh)
	}
}

func (w *Worker) cancelRequest(request *common.BrokerRequest) {
	if request.ResultCh != nil {
		request.ResultCh <- common.BrokerResult{
			Error: errors.New("request cancelled"),
		}
	}
}

func (w *Worker) processGetTunnelInfoRequest(
	req *common.GetTunnelInfoRequest, resultCh chan<- common.BrokerResult) {
	tunnelInfo, err := w.getTunnelInfo(req.TunnelId)

	if err != nil {
		completeRequestWithError(
			resultCh,
			fmt.Errorf("error to retrieving tunnel info: %w", err),
			"Tunnel info retrieval failed")
		return
	}

	completeRequestWithSuccess(
		resultCh,
		buildTunnelData(&tunnelInfo, time.Now()),
		"Retrieved successfully",
		"Tunnel info retrieval")

}

func (w *Worker) processUpdateTunnelClientIpV4Address(
	req *common.UpdateTunnelClientIpV4AddressRequest,
	resultCh chan<- common.BrokerResult) {

	currentClientIpV4Address := req.CurrentData.ClientIpV4Address
	var tunnelData data.TunnelData

	if req.CurrentData.LastUpdateTime.Before(time.Now().Add(time.Duration(5) * time.Minute)) {
		tunnelInfo, err := w.getTunnelInfo(req.TunnelId)

		if err != nil {
			completeRequestWithError(
				resultCh,
				fmt.Errorf("error updating tunnel client IPv4 address: %w", err),
				"Tunnel client IPv4 address update failed")
			return
		}

		currentClientIpV4Address = net.ParseIP(tunnelInfo.ClientV4)
		tunnelData = buildTunnelData(&tunnelInfo, time.Now())
	} else {
		tunnelData = req.CurrentData
	}

	if req.NewAddress.Equal(currentClientIpV4Address) {
		completeRequestWithSuccess(
			resultCh,
			tunnelData,
			"Client IPv4 address already set to requested value",
			"Tunnel client IPv4 address update")

	}

	err := w.updateTunnelClientIpV4Address(req.TunnelId, req.NewAddress)

	if err != nil {
		completeRequestWithError(
			resultCh,
			fmt.Errorf("error updating tunnel client IPv4 address: %w", err),
			"Tunnel client IPv4 address update failed")
		return
	}

	tunnelData.ClientIpV4Address = req.NewAddress
	tunnelData.LastModifiedTime = time.Now()

	completeRequestWithSuccess(
		resultCh,
		tunnelData,
		"Client IPv4 address updated successfully",
		"Tunnel client IPv4 address update")
}

func (w *Worker) getTunnelInfo(tunnelId string) (messages.Tunnel, error) {
	uri := fmt.Sprintf("https://%s:%s@ipv4.tunnelbroker.net/tunnelInfo.php?tid=%s",
		w.username, w.updateKey, tunnelId)
	tunnels := messages.Tunnels{}
	err := w.executeHttpGet(uri, &tunnels)

	if err != nil {
		return messages.Tunnel{},
			fmt.Errorf("error retrieving tunnel %s: %w",
				tunnelId, err)
	}

	fmt.Printf("%T Retrieved tunnel info: %+v\n", w, tunnels)

	count := len(tunnels.Tunnel)

	if count == 0 {
		return messages.Tunnel{},
			fmt.Errorf("no tunnels with tid %s",
				tunnelId)
	} else if count > 1 {
		fmt.Printf("%T Warning: multiple tunnels for tid %s, using first one\n",
			w, tunnelId)
	}

	return tunnels.Tunnel[0], nil
}

func (w *Worker) updateTunnelClientIpV4Address(tunnelId string, newAddress net.IP) error {
	uri := fmt.Sprintf("https://ipv4.tunnelbroker.net/nic/update?username=%s&password=%s&hostname=%s&myip=%s",
		w.username, w.updateKey, tunnelId, newAddress.String())

	if err := w.executeHttpGet(uri, nil); err != nil {
		return err
	}

	return nil
}

func (w *Worker) executeHttpGet(uri string, result any) error {
	response, err := http.Get(uri)

	if err != nil {
		return fmt.Errorf("http get failed: %w", err)
	}
	defer func() { closeResponse(response) }()

	responseBytes, err := io.ReadAll(response.Body)

	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if result != nil {
		err = xml.Unmarshal(responseBytes, result)

		if err != nil {
			return fmt.Errorf("error unmarshalling response: %w (body: %s)", err, string(responseBytes))
		}
	}

	return nil
}

func closeResponse(response *http.Response) {
	if response != nil {
		closeErr := response.Body.Close()

		if closeErr != nil {
			fmt.Printf("Error closing response body: %s\n", closeErr)
		}
	}
}

func completeRequestWithError(resultCh chan<- common.BrokerResult, err error, logMessage string) {
	if resultCh == nil {
		fmt.Printf("%s: %s\n", logMessage, err)
	} else {
		resultCh <- common.BrokerResult{
			Error: err,
		}
		close(resultCh)
	}
}

func completeRequestWithSuccess(
	resultCh chan<- common.BrokerResult,
	tunnelData data.TunnelData,
	message string,
	logMessage string) {

	if resultCh == nil {
		fmt.Printf("%s: %s\n", logMessage, message)
	} else {
		resultCh <- common.BrokerResult{
			Message:    message,
			TunnelData: tunnelData,
		}
		close(resultCh)
	}
}

func buildTunnelData(tunnelInfo *messages.Tunnel, lastUpdateTime time.Time) data.TunnelData {
	_, routed64Net, routed64NetError := net.ParseCIDR(tunnelInfo.Routed64)
	_, routed48Net, routed48NetError := net.ParseCIDR(tunnelInfo.Routed48)

	tunnelData := data.TunnelData{
		LastUpdateTime:    lastUpdateTime,
		Description:       tunnelInfo.Description,
		ServerIpV4Address: net.ParseIP(tunnelInfo.ServerV4),
		ServerIpV6Address: net.ParseIP(tunnelInfo.ServerV6),
		ClientIpV4Address: net.ParseIP(tunnelInfo.ClientV4),
		ClientIpV6Address: net.ParseIP(tunnelInfo.ClientV6),
	}

	if routed64NetError == nil {
		tunnelData.Routed64Net = *routed64Net
	}

	if routed48NetError == nil {
		tunnelData.Routed48Net = *routed48Net
	}

	return tunnelData
}
