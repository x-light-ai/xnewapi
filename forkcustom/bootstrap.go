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
		model.StartChannelMonitorPersistenceTask()
		model.StartChannelMonitorCleanupTask()
		service.StartChannelSuccessRateHealthManager()
	})
}
