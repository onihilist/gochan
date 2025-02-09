package gcsql

import (
	"context"
	"database/sql"
	"errors"
)

var (
	// AllSections provides a quick and simple way to access a list of all non-hidden sections without
	// having to do any SQL queries. It and AllBoards are updated by ResetBoardSectionArrays
	AllSections            []Section
	ErrSectionDoesNotExist = errors.New("section does not exist")
)

// GetAllSections gets a list of all existing sections, optionally omitting hidden ones
func GetAllSections(onlyNonHidden bool) ([]Section, error) {
	query := `SELECT
	id, name, abbreviation, position, hidden
	FROM DBPREFIXsections`
	if onlyNonHidden {
		query += " WHERE hidden = FALSE "
	}
	query += " ORDER BY position ASC, name ASC"

	rows, cancel, err := QueryTimeoutSQL(nil, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		cancel()
		rows.Close()
	}()
	var sections []Section
	for rows.Next() {
		var section Section
		err = rows.Scan(&section.ID, &section.Name, &section.Abbreviation, &section.Position, &section.Hidden)
		if err != nil {
			return nil, err
		}
		sections = append(sections, section)
	}
	return sections, rows.Close()
}

// getOrCreateDefaultSectionID creates the default section if no sections have been created yet,
// returns default section ID if it exists
func getOrCreateDefaultSectionID() (sectionID int, err error) {
	const query = `SELECT id FROM DBPREFIXsections WHERE name = 'Main'`
	var id int

	err = QueryRowTimeoutSQL(nil, query, nil, []any{&id})
	if errors.Is(err, sql.ErrNoRows) {
		var section *Section
		if section, err = NewSection("Main", "main", false, -1); err != nil {
			return 0, err
		}
		return section.ID, err
	}
	if err != nil {
		return 0, err //other error
	}
	return id, nil
}

// GetSectionFromID returns a section from the database, given its ID
func GetSectionFromID(id int) (*Section, error) {
	const query = `SELECT id, name, abbreviation, position, hidden FROM DBPREFIXsections WHERE id = ?`
	var section Section
	err := QueryRowTimeoutSQL(nil, query, []any{id}, []any{
		&section.ID, &section.Name, &section.Abbreviation, &section.Position, &section.Hidden,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSectionDoesNotExist
	} else if err != nil {
		return nil, err
	}
	return &section, err
}

// GetSectionFromID returns a section from the database, given its name
func GetSectionFromName(name string) (*Section, error) {
	const query = `SELECT id, name, abbreviation, position, hidden FROM DBPREFIXsections WHERE name = ?`
	var section Section
	err := QueryRowTimeoutSQL(nil, query, []any{name}, []any{
		&section.ID, &section.Name, &section.Abbreviation, &section.Position, &section.Hidden,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSectionDoesNotExist
	} else if err != nil {
		return nil, err
	}
	return &section, err
}

// DeleteSection deletes a section from the database and resets the AllSections array
func DeleteSection(id int) error {
	const query = `DELETE FROM DBPREFIXsections WHERE id = ?`
	_, err := ExecSQL(query, id)
	if err != nil {
		return err
	}
	return ResetBoardSectionArrays()
}

// NewSection creates a new board section in the database and returns a *Section struct pointer.
// If position < 0, it will use the ID
func NewSection(name string, abbreviation string, hidden bool, position int) (*Section, error) {
	const sqlINSERT = `INSERT INTO DBPREFIXsections (name, abbreviation, hidden, position) VALUES (?,?,?,?)`
	const sqlPosition = `SELECT COALESCE(MAX(position) + 1, 1) FROM DBPREFIXsections`

	tx, err := BeginTx()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), gcdb.defaultTimeout)
	defer func() {
		cancel()
		tx.Rollback()
	}()

	if position < 0 {
		// position not specified
		err = QueryRowContextSQL(ctx, tx, sqlPosition, nil, []any{&position})
		if errors.Is(err, sql.ErrNoRows) {
			position = 1
		} else if err != nil {
			return nil, err
		}
	}
	if _, err = ExecContextSQL(ctx, tx, sqlINSERT, name, abbreviation, hidden, position); err != nil {
		return nil, err
	}
	id, err := getLatestID("DBPREFIXsections", tx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &Section{
		ID:           id,
		Name:         name,
		Abbreviation: abbreviation,
		Position:     position,
		Hidden:       hidden,
	}, nil
}

func (s *Section) UpdateValues() error {
	const query = `UPDATE DBPREFIXsections set name = ?, abbreviation = ?, position = ?, hidden = ? WHERE id = ?`
	_, err := ExecTimeoutSQL(nil, query, s.Name, s.Abbreviation, s.Position, s.Hidden, s.ID)
	return err
}
