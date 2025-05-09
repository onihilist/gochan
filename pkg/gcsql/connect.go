package gcsql

import (
	"database/sql"
	"os"
	"regexp"
	"strings"

	"github.com/gochan-org/gochan/pkg/config"
	"github.com/gochan-org/gochan/pkg/gcutil/testutil"
)

// ConnectToDB initializes the database connection and exits if there are any errors
func ConnectToDB(cfg *config.SQLConfig) error {
	var err error
	gcdb, err = Open(cfg)
	return err
}

// SetDB sets the global database connection (mainly used by gochan-migration)
func SetDB(db *GCDB) {
	gcdb = db
}

func SetTestingDB(dbDriver string, dbName string, dbPrefix string, db *sql.DB) (err error) {
	testutil.PanicIfNotTest()
	sqlConfig := config.GetSQLConfig()
	if sqlConfig.DBname == "" {
		return ErrNotConnected
	}

	gcdb, err = setupDBConn(&config.SQLConfig{
		DBtype:               dbDriver,
		DBhost:               "localhost",
		DBname:               dbName,
		DBusername:           "gochan",
		DBpassword:           "gochan",
		DBprefix:             dbPrefix,
		DBTimeoutSeconds:     config.DefaultSQLTimeout,
		DBMaxOpenConnections: config.DefaultSQLMaxConns,
		DBMaxIdleConnections: config.DefaultSQLMaxConns,
		DBConnMaxLifetimeMin: config.DefaultSQLConnMaxLifetimeMin,
	})
	if err != nil {
		return
	}
	gcdb.db = db
	return
}

// RunSQLFile cuts a given sql file into individual statements and runs it.
func RunSQLFile(path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	sqlStr := regexp.MustCompile("--.*\n?").ReplaceAllString(string(sqlBytes), " ")
	sqlArr := strings.Split(gcdb.replacer.Replace(sqlStr), ";")

	tx, err := BeginTx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range sqlArr {
		statement = strings.Trim(statement, " \n\r\t")
		if len(statement) > 0 {
			if _, err = ExecTxSQL(tx, statement); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
