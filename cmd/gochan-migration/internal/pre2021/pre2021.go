// used for migrating pre-refactor gochan databases to the new schema
package pre2021

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/gochan-org/gochan/cmd/gochan-migration/internal/common"
	"github.com/gochan-org/gochan/pkg/config"
	"github.com/gochan-org/gochan/pkg/gcsql"
)

type Pre2021Config struct {
	config.SQLConfig
	DocumentRoot string
}

type Pre2021Migrator struct {
	db      *gcsql.GCDB
	options *common.MigrationOptions
	config  Pre2021Config

	boards   []migrationBoard
	sections []migrationSection
}

// IsMigratingInPlace implements common.DBMigrator.
func (m *Pre2021Migrator) IsMigratingInPlace() bool {
	sqlConfig := config.GetSQLConfig()
	return m.config.DBname == sqlConfig.DBname && m.config.DBhost == sqlConfig.DBhost && m.config.DBprefix == sqlConfig.DBprefix
}

func (m *Pre2021Migrator) readConfig() error {
	ba, err := os.ReadFile(m.options.OldChanConfig)
	if err != nil {
		return err
	}
	m.config.SQLConfig = config.GetSQLConfig()
	return json.Unmarshal(ba, &m.config)
}

func (m *Pre2021Migrator) Init(options *common.MigrationOptions) error {
	m.options = options
	var err error
	m.config.SQLConfig = config.GetSQLConfig()
	if err = m.readConfig(); err != nil {
		return err
	}

	m.db, err = gcsql.Open(&m.config.SQLConfig)
	return err
}

func (m *Pre2021Migrator) IsMigrated() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.config.DBTimeoutSeconds)*time.Second)
	defer cancel()
	var sqlConfig config.SQLConfig
	if m.IsMigratingInPlace() {
		sqlConfig = config.GetSQLConfig()
	} else {
		sqlConfig = m.config.SQLConfig
	}
	return common.TableExists(ctx, m.db, nil, "DBPREFIXdatabase_version", &sqlConfig)
}

func (m *Pre2021Migrator) MigrateDB() (bool, error) {
	errEv := common.LogError()
	defer errEv.Discard()
	migrated, err := m.IsMigrated()
	if err != nil {
		errEv.Caller().Err(err).Msg("Error checking if database is migrated")
		return false, err
	}
	if migrated {
		// db is already migrated, stop
		common.LogWarning().Msg("Database is already migrated (database_version table exists)")
		return true, nil
	}

	if err := m.MigrateBoards(); err != nil {
		return false, err
	}
	common.LogInfo().Msg("Migrated boards successfully")
	if err = m.MigratePosts(); err != nil {
		return false, err
	}
	// if err = m.MigrateStaff("password"); err != nil {
	// 	return false, err
	// }
	// if err = m.MigrateBans(); err != nil {
	// 	return false, err
	// }
	// if err = m.MigrateAnnouncements(); err != nil {
	// 	return false, err
	// }

	return true, nil
}

func (*Pre2021Migrator) MigrateStaff(_ string) error {
	return nil
}

func (*Pre2021Migrator) MigrateBans() error {
	return nil
}

func (*Pre2021Migrator) MigrateAnnouncements() error {
	return nil
}

func (m *Pre2021Migrator) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
