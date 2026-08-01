// FORK-CUSTOM: Centralize fork-owned runtime composition outside upstream main.
package forkcustom

import (
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

var startOnce sync.Once

func Start() {
	startOnce.Do(func() {
		if err := registerClaudeTranslator(); err != nil {
			common.FatalLog(fmt.Sprintf("failed to register fork Claude translator: %v", err))
		}
		// FORK-CUSTOM: Register provider collectors at the fork composition boundary.
		if err := service.RegisterUpstreamProviderSyncAdapters(); err != nil {
			common.FatalLog(fmt.Sprintf("failed to register provider sync adapters: %v", err))
		}
		model.StartChannelMonitorPersistenceTask()
		model.StartChannelMonitorCleanupTask()
		service.StartChannelSuccessRateHealthManager()
		service.StartUpstreamProviderSyncScheduler()
	})
}
