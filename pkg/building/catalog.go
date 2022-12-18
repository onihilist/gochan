package building

import (
	"fmt"
	"os"
	"path"

	"github.com/gochan-org/gochan/pkg/config"
	"github.com/gochan-org/gochan/pkg/gcsql"
	"github.com/gochan-org/gochan/pkg/gctemplates"
	"github.com/gochan-org/gochan/pkg/gcutil"
	"github.com/gochan-org/gochan/pkg/serverutil"
)

type catalogThreadData struct {
	Replies       int    `json:"replies"`
	Images        int    `json:"images"`
	OmittedPosts  int    `json:"omitted_posts"`  // posts in the thread but not shown on the board page
	OmittedImages int    `json:"omitted_images"` // uploads in the thread but not shown on the board page
	Sticky        int    `json:"sticky"`
	Locked        int    `json:"locked"`
	Posts         []Post `json:"-"`
	uploads       []gcsql.Upload
}

type catalogPage struct {
	PageNum int                 `json:"page"`
	Threads []catalogThreadData `json:"threads"`
}

type boardCatalog struct {
	pages       []catalogPage // this array gets marshalled, not the boardCatalog object
	numPages    int
	currentPage int
}

// fillPages fills the catalog's pages array with pages of the specified size, with the remainder
// on the last page
func (catalog *boardCatalog) fillPages(threadsPerPage int, threads []catalogThreadData) {
	catalog.pages = []catalogPage{} // clear the array if it isn't already
	catalog.numPages = len(threads) / threadsPerPage
	remainder := len(threads) % threadsPerPage
	currentThreadIndex := 0
	var i int
	for i = 0; i < catalog.numPages; i++ {
		catalog.pages = append(catalog.pages,
			catalogPage{
				PageNum: i + 1,
				Threads: threads[currentThreadIndex : currentThreadIndex+threadsPerPage],
			},
		)
		currentThreadIndex += threadsPerPage
	}
	if remainder > 0 {
		catalog.pages = append(catalog.pages,
			catalogPage{
				PageNum: i + 1,
				Threads: threads[len(threads)-remainder:],
			},
		)
	}
}

// BuildCatalog builds the catalog for a board with a given id
func BuildCatalog(boardID int) error {
	errEv := gcutil.LogError(nil).
		Str("building", "catalog").
		Int("boardID", boardID)
	err := gctemplates.InitTemplates("catalog")
	if err != nil {
		errEv.Err(err).Send()
		return err
	}

	board, err := gcsql.GetBoardFromID(boardID)
	if err != nil {
		errEv.Err(err).
			Caller().Msg("Unable to get board information")
		return err
	}
	errEv.Str("boardDir", board.Dir)
	criticalCfg := config.GetSystemCriticalConfig()
	catalogPath := path.Join(criticalCfg.DocumentRoot, board.Dir, "catalog.html")
	catalogFile, err := os.OpenFile(catalogPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return fmt.Errorf("failed opening /%s/catalog.html: %s", board.Dir, err.Error())
	}

	threadOPs, err := getBoardTopPosts(boardID)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return fmt.Errorf("failed building catalog for /%s/: %s", board.Dir, err.Error())
	}
	boardConfig := config.GetBoardConfig(board.Dir)

	if err = serverutil.MinifyTemplate(gctemplates.Catalog, map[string]interface{}{
		"boards":       gcsql.AllBoards,
		"webroot":      criticalCfg.WebRoot,
		"board":        board,
		"board_config": boardConfig,
		"sections":     gcsql.AllSections,
		"threads":      threadOPs,
	}, catalogFile, "text/html"); err != nil {
		errEv.Err(err).Caller().Send()
		return fmt.Errorf("failed building catalog for /%s/: %s", board.Dir, err.Error())
	}
	return nil
}