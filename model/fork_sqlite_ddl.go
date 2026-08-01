// FORK-CUSTOM: Normalize SQLite index DDL used by the channel circuit migration.
package model

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var sqliteIndexHeaderRegexp = regexp.MustCompile(`(?is)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\S+)\s+ON\s+(.*)$`)

type sqliteIndexDefinition struct {
	Name string
	SQL  string
}

func normalizeSQLiteIndexHeaders(tableName string) error {
	if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return nil
	}
	var indexes []sqliteIndexDefinition
	if err := DB.Raw("SELECT name, sql FROM sqlite_master WHERE type = ? AND tbl_name = ? AND sql IS NOT NULL", "index", tableName).Scan(&indexes).Error; err != nil {
		return err
	}
	for _, index := range indexes {
		normalized, ok := normalizedSQLiteIndexDDL(index.SQL)
		if !ok || normalized == index.SQL {
			continue
		}
		if err := DB.Migrator().DropIndex(tableName, index.Name); err != nil {
			return err
		}
		if err := DB.Exec(normalized).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizedSQLiteIndexDDL(ddl string) (string, bool) {
	matches := sqliteIndexHeaderRegexp.FindStringSubmatch(ddl)
	if len(matches) == 0 {
		return "", false
	}
	unique := ""
	if strings.TrimSpace(matches[1]) != "" {
		unique = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX %s ON %s", unique, matches[2], matches[3]), true
}
