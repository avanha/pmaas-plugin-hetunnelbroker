package tunnel

import (
	"fmt"
	"sync/atomic"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/entities"
	spicommon "github.com/avanha/pmaas-spi/common"
)

type Stub struct {
	pmaasEntityId          string
	closeFn                func() error
	entityWrapperReference atomic.Pointer[spicommon.ThreadSafeEntityWrapper[entities.Tunnel]]
}

func NewStub(pmaasEntityId string, entityWrapper *spicommon.ThreadSafeEntityWrapper[entities.Tunnel]) *Stub {
	stub := &Stub{
		pmaasEntityId: pmaasEntityId,
	}

	stub.entityWrapperReference.Store(entityWrapper)

	stub.closeFn = func() error {
		if stub.entityWrapperReference.CompareAndSwap(entityWrapper, nil) {
			stub.closeFn = nil
			return nil
		}

		return fmt.Errorf("failed to clear entity wrapper, current value does not match expected value")
	}

	return stub
}

func (s *Stub) Close() {

}

func (s *Stub) Name() string {
	return spicommon.ThreadSafeEntityWrapperExecValueFunc(
		s.entityWrapperReference.Load(),
		func(target entities.Tunnel) string { return target.Name() })
}
