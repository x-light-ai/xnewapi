// FORK-CUSTOM: Isolate channel-monitor migration from upstream model/main.go.
package model

import "github.com/QuantumNous/new-api/common"

// migrateChannelMonitorTables owns every table behind channel monitoring. Keeping the
// registration here instead of in the upstream AutoMigrate list avoids conflicting with
// upstream whenever it appends models of its own.
func migrateChannelMonitorTables() error {
	if err := DB.AutoMigrate(&ChannelMonitorStat{}, &ChannelMonitorError{}); err != nil {
		return err
	}
	// Circuit events keep a hand-written SQLite path; other databases use AutoMigrate.
	if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return DB.AutoMigrate(&ChannelCircuitEvent{})
	}
	return ensureChannelCircuitEventTableSQLite()
}

func ensureChannelCircuitEventTableSQLite() error {
	if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return nil
	}
	tableName := (ChannelCircuitEvent{}).TableName()
	if !DB.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
	` + "`id`" + ` integer,
	` + "`channel_id`" + ` integer NOT NULL,
	` + "`group_name`" + ` varchar(64) NOT NULL DEFAULT '',
	` + "`model_name`" + ` varchar(128) NOT NULL DEFAULT '',
	` + "`circuit_type`" + ` varchar(32) NOT NULL DEFAULT 'temporary',
	` + "`trigger_type`" + ` varchar(32) NOT NULL DEFAULT '',
	` + "`reason`" + ` text NOT NULL DEFAULT '',
	` + "`opened_at`" + ` bigint NOT NULL,
	` + "`recovered_at`" + ` bigint NOT NULL DEFAULT 0,
	` + "`created_at`" + ` bigint NOT NULL,
	` + "`updated_at`" + ` bigint NOT NULL,
	PRIMARY KEY (` + "`id`" + `)
	)`
		if err := DB.Exec(createSQL).Error; err != nil {
			return err
		}
	} else {
		var cols []struct {
			Name string `gorm:"column:name"`
		}
		if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
			return err
		}
		existing := make(map[string]struct{}, len(cols))
		for _, c := range cols {
			existing[c.Name] = struct{}{}
		}
		required := []sqliteColumnDef{
			{Name: "channel_id", DDL: "`channel_id` integer NOT NULL DEFAULT 0"},
			{Name: "group_name", DDL: "`group_name` varchar(64) NOT NULL DEFAULT ''"},
			{Name: "model_name", DDL: "`model_name` varchar(128) NOT NULL DEFAULT ''"},
			{Name: "circuit_type", DDL: "`circuit_type` varchar(32) NOT NULL DEFAULT 'temporary'"},
			{Name: "trigger_type", DDL: "`trigger_type` varchar(32) NOT NULL DEFAULT ''"},
			{Name: "reason", DDL: "`reason` text NOT NULL DEFAULT ''"},
			{Name: "opened_at", DDL: "`opened_at` bigint NOT NULL DEFAULT 0"},
			{Name: "recovered_at", DDL: "`recovered_at` bigint NOT NULL DEFAULT 0"},
			{Name: "created_at", DDL: "`created_at` bigint NOT NULL DEFAULT 0"},
			{Name: "updated_at", DDL: "`updated_at` bigint NOT NULL DEFAULT 0"},
		}
		for _, col := range required {
			if _, ok := existing[col.Name]; ok {
				continue
			}
			if err := DB.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
				return err
			}
		}
	}
	// Legacy databases stored these indexes with a newline in the DDL header, which
	// glebarez/sqlite v1.9.0 rejects as invalid DDL; IF NOT EXISTS alone cannot fix them.
	if err := normalizeSQLiteIndexHeaders(tableName); err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS `idx_channel_circuit_events_channel_opened_at` ON `" + tableName + "`(`channel_id`,`opened_at`)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS `idx_channel_circuit_events_opened_at` ON `" + tableName + "`(`opened_at`)").Error; err != nil {
		return err
	}
	return nil
}
