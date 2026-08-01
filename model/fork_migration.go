// FORK-CUSTOM: Single migration entry point for fork-owned tables.
package model

// migrateForkTables migrates every table introduced by the fork. Registering them here
// instead of inside the upstream AutoMigrate list keeps `model/main.go` down to one call
// per migration function, so upstream changes to its own model list never conflict.
func migrateForkTables() error {
	if err := migrateChannelMonitorTables(); err != nil {
		return err
	}
	return migrateUpstreamProviderTables()
}
