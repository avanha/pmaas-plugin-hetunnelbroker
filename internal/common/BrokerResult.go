package common

import "github.com/avanha/pmaas-plugin-hetunnelbroker/data"

type BrokerResult struct {
	Error      error
	Message    string
	TunnelData data.TunnelData
}
