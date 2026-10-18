// FORK-CUSTOM: Isolate upstream provider management migration from upstream model/main.go.
package model

// upstreamProviderMigrationModels lists the tables owned by model provider management.
func upstreamProviderMigrationModels() []any {
	return []any{
		&UpstreamProvider{},
		&UpstreamProviderAccount{},
		&UpstreamProviderGroup{},
		&UpstreamProviderGroupChannel{},
		&UpstreamProviderKey{},
		&UpstreamProviderSyncRun{},
		&UpstreamProviderGroupProfitDaily{},
	}
}

func migrateUpstreamProviderTables() error {
	return DB.AutoMigrate(upstreamProviderMigrationModels()...)
}
