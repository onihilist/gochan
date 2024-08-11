package gcsql

import (
	"database/sql"
	"errors"
)

// GetAllFilters returns an array of all post filters, and an error if one occured
func GetAllFilters() ([]Filter, error) {
	return GetFiltersByBoardID(0, true)
}

// GetFiltersByBoardID returns an array of post filters associated to the given board ID, including
// filters set to "All boards" if includeAllBoards is true
func GetFiltersByBoardID(boardID int, includeAllBoards bool) ([]Filter, error) {
	query := `SELECT DBPREFIXfilters.id, staff_id, staff_note, issued_at, match_action, match_detail
		FROM DBPREFIXfilters LEFT JOIN DBPREFIXfilter_boards ON filter_id = DBPREFIXfilters.id
		WHERE board_id = ?`
	if includeAllBoards {
		query += " OR board_id IS NULL"
	}
	rows, cancel, err := QueryTimeoutSQL(nil, query, boardID)
	if err != nil {
		return nil, err
	}
	defer func() {
		cancel()
		rows.Close()
	}()
	var filters []Filter
	for rows.Next() {
		var filter Filter
		if err = rows.Scan(
			&filter.ID, &filter.StaffID, &filter.StaffNote, &filter.IssuedAt, &filter.MatchAction, &filter.MatchDetail,
		); err != nil {
			return nil, err
		}
	}
	return filters, rows.Close()
}

// Conditions returns an array of filter conditions associated with the filter
func (f *Filter) Conditions() ([]FilterCondition, error) {
	if len(f.conditions) > 0 {
		return f.conditions, nil
	}

	rows, cancel, err := QueryTimeoutSQL(nil, `SELECT id, filter_id, is_regex, search, field WHERE filter_id = ?`, f.ID)
	if err != nil {
		return nil, err
	}
	defer func() {
		cancel()
		rows.Close()
	}()
	for rows.Next() {
		var condition FilterCondition
		if err = rows.Scan(&condition.ID, &condition.FilterID, &condition.IsRegex, &condition.Search, &condition.Field); err != nil {
			return nil, err
		}
		f.conditions = append(f.conditions, condition)
	}
	return f.conditions, rows.Close()
}

// BoardDirs returns an array of board directories associated with this filter
func (f *Filter) BoardDirs() ([]string, error) {
	rows, cancel, err := QueryTimeoutSQL(nil, `SELECT dir FROM DBPREFIXfilter_boards
		LEFT JOIN DBPREFIXboards ON DBPREFIXboards.id = board_id WHERE filter_id = ?`, f.ID)
	if errors.Is(err, sql.ErrNoRows) {
		cancel()
		return nil, nil
	} else if err != nil {
		cancel()
		return nil, err
	}
	defer func() {
		cancel()
		rows.Close()
	}()
	var dirs []string
	for rows.Next() {
		var dir *string
		if err = rows.Scan(&dir); err != nil {
			return nil, err
		}
		if dir != nil {
			dirs = append(dirs, *dir)
		}
	}
	return dirs, rows.Close()
}

func (f *Filter) BoardIDs() ([]int, error) {
	rows, cancel, err := QueryTimeoutSQL(nil, `SELECT board_id FROM DBPREFIXfilter_boards WHERE filter_id = ?`, f.ID)
	if err != nil {
		return nil, err
	}
	defer func() {
		cancel()
		rows.Close()
	}()
	var ids []int
	for rows.Next() {
		var id int
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}