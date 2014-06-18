// functions for handling posting, uploading, and post/thread/board page building

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/disintegration/imaging"
	"html"
	"image"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	whitespace_match = "[\000-\040]"
	last_post PostTable
)

func generateTripCode(input string) string {
	re := regexp.MustCompile("[^\\.-z]") // remove every ASCII character before . and after z

	input += "   " // padding
	salt := string(re.ReplaceAllLiteral([]byte(input), []byte(".")))
	salt = byteByByteReplace(salt[1:3],":;<=>?@[\\]^_`", "ABCDEFGabcdef") // stole-I MEAN BORROWED from Kusaba X

	return crypt(input,salt)[3:]
}


func buildBoardPage(boardid int, boards []BoardsTable, sections []interface{}) (html string) {
	start_time := benchmarkTimer("buildBoard" + string(boardid), time.Now(), true)
	var board BoardsTable
	for b,_ := range boards {
		if boards[b].ID == boardid {
			board = boards[b]
		}
	}

	var interfaces []interface{}
	var threads []interface{}
	var op_posts []interface{}
	op_posts,err := getPostArr("SELECT * FROM `"+config.DBprefix+"posts` WHERE `boardid` = "+strconv.Itoa(board.ID)+" AND `parentid` = 0 AND `deleted_timestamp` = '" + nil_timestamp + "' ORDER BY `bumped` DESC LIMIT "+strconv.Itoa(config.ThreadsPerPage_img))
	if err != nil {
		html += err.Error() + "<br />"
		op_posts = make([]interface{},0)
	}

	for _,op_post_i := range op_posts {
		var thread Thread
		var posts_in_thread []interface{}

		op_post := op_post_i.(PostTable)

		if op_post.Stickied {
			thread.IName = "thread"

			posts_in_thread,err = getPostArr("SELECT * FROM `"+config.DBprefix+"posts` WHERE `boardid` = "+strconv.Itoa(board.ID)+" AND `parentid` = "+strconv.Itoa(op_post.ID)+" AND `deleted_timestamp` = '" + nil_timestamp + "' ORDER BY `id` DESC LIMIT "+strconv.Itoa(config.StickyRepliesOnBoardPage))
			if err != nil {
				html += err.Error()+"<br />"
			}
			err = db.QueryRow("SELECT COUNT(*) FROM `"+config.DBprefix+"posts` WHERE `boardid` = "+strconv.Itoa(board.ID)+" AND `parentid` = "+strconv.Itoa(op_post.ID)).Scan(&thread.NumReplies)
			if err != nil {
				html += err.Error()+"<br />"
			}
			thread.OP = op_post_i
			if len(posts_in_thread) > 0 {
				thread.BoardReplies = posts_in_thread
			}
			threads = append(threads, thread)
		}
	}

	for _,op_post_i := range op_posts {
		var thread Thread
		var posts_in_thread []interface{}

		op_post := op_post_i.(PostTable)
		if !op_post.Stickied {
			thread.IName = "thread"

			posts_in_thread,err = getPostArr("SELECT * FROM (SELECT * FROM `"+config.DBprefix+"posts` WHERE `boardid` = "+strconv.Itoa(board.ID)+" AND `parentid` = "+strconv.Itoa(op_post.ID)+" AND `deleted_timestamp` = '" + nil_timestamp + "' ORDER BY `id` DESC  LIMIT "+strconv.Itoa(config.RepliesOnBoardpage)+") t ORDER BY `id` ASC")
			if err != nil {
				html += err.Error()+"<br />"
			}
			err = db.QueryRow("SELECT COUNT(*) FROM `"+config.DBprefix+"posts` WHERE `boardid` = "+strconv.Itoa(board.ID)+" AND `parentid` = "+strconv.Itoa(op_post.ID)).Scan(&thread.NumReplies)
			if err != nil {
				html += err.Error()+"<br />"
			}
			thread.OP = op_post_i
			if len(posts_in_thread) > 0 {
				thread.BoardReplies = posts_in_thread
			}
			threads = append(threads, thread)
		}
	}

    interfaces = append(interfaces, config)

    var boards_i []interface{}
    for _,b := range boards {
    	boards_i = append(boards_i,b)
    }
    var boardinfo_i []interface{}
    boardinfo_i = append(boardinfo_i,board)

    interfaces = append(interfaces, &Wrapper{IName: "boards", Data: boards_i})
    interfaces = append(interfaces, &Wrapper{IName: "sections", Data: sections})
    interfaces = append(interfaces, &Wrapper{IName: "threads", Data: threads})
    interfaces = append(interfaces, &Wrapper{IName: "boardinfo", Data: boardinfo_i})

	wrapped := &Wrapper{IName: "boardpage",Data: interfaces}
	os.Remove(path.Join(config.DocumentRoot,board.Dir,"board.html"))

	results,err := os.Stat(path.Join(config.DocumentRoot, board.Dir))
	if err != nil {
		err = os.Mkdir(path.Join(config.DocumentRoot,board.Dir),0777)
		if err != nil {
			html += "Failed creating /" + board.Dir + "/: " + err.Error() + "<br />\n"
		}
	} else if !results.IsDir() {
		html += "Error: /" + board.Dir + "/ exists, but is not a folder. <br />\n"
	}

	board_file,err := os.OpenFile(path.Join(config.DocumentRoot, board.Dir, "board.html"),os.O_CREATE|os.O_RDWR,0777)
	if err != nil {
		html += err.Error()+"<br />\n"
	}

	defer func() {
		if uhoh, ok := recover().(error); ok {
			error_log.Print("Failed executing template.")
			fmt.Println(uhoh.Error())
		}
		if board_file != nil {
			board_file.Close()
		}
	}()
	err = img_boardpage_tmpl.Execute(board_file,wrapped)
	if err != nil {
		html += "Failed building /"+board.Dir+"/: "+err.Error()+"<br />\n"
		error_log.Print(err.Error())
	} else {
		html += "/"+board.Dir+"/ built successfully.\n"
	}
	benchmarkTimer("buildBoard" + string(boardid), start_time, false)
	return
}

func buildFrontPage(boards []BoardsTable, sections []interface{}) (html string) {
	initTemplates()

	var front_arr []interface{}
	var recent_posts_arr []interface{}
	var boards_arr []interface{}
	
	for _,board := range boards {
		boards_arr = append(boards_arr, board)
	}


	os.Remove(path.Join(config.DocumentRoot,"index.html"))
	front_file,err := os.OpenFile(path.Join(config.DocumentRoot,"index.html"),os.O_CREATE|os.O_RDWR,0777)
	/*defer func() {
		if front_file != nil {
			front_file.Close()
		}
	}()*/
	if err != nil {
		return err.Error()
	}

	// get front pages
	rows,err := db.Query("SELECT * FROM `"+config.DBprefix+"frontpage`;")
	if err != nil {
		error_log.Print(err.Error())
		return err.Error()
	}
	for rows.Next() {
		frontpage := new(FrontTable)
		frontpage.IName = "front page"
		err = rows.Scan(&frontpage.ID, &frontpage.Page, &frontpage.Order, &frontpage.Subject, &frontpage.Message, &frontpage.Timestamp, &frontpage.Poster, &frontpage.Email)
		if err != nil {
			error_log.Print(err.Error())
			return err.Error()
		}
		front_arr = append(front_arr,frontpage)
	}

	// get recent posts
	rows,err = db.Query("SELECT `" + config.DBprefix + "posts`.`id`, " +
							   "`" + config.DBprefix + "posts`.`parentid`, " + 
							   "`" + config.DBprefix +"boards`.`dir` AS boardname, " +
							   "`" + config.DBprefix + "posts`.`boardid` AS boardid, " +
							   "`name`, " +
							   "`tripcode`, " +
							   "`message`," +
							   "`filename`, " +
							   "`thumb_w`, " +
							   "`thumb_h` " +
							   " FROM `" + config.DBprefix + "posts`, " +
							   "`" + config.DBprefix + "boards` " +
							   "WHERE `" + config.DBprefix + "posts`.`deleted_timestamp` = \"" + nil_timestamp + "\"" +
							   "ORDER BY `timestamp` DESC " +
							   "LIMIT " + strconv.Itoa(config.MaxRecentPosts))
	if err != nil {
		error_log.Print(err.Error())
		return err.Error()
	}
	for rows.Next() {
		recent_posts := new(RecentPost)
		err = rows.Scan(&recent_posts.PostID, &recent_posts.ParentID, &recent_posts.BoardName, &recent_posts.BoardID, &recent_posts.Name, &recent_posts.Tripcode, &recent_posts.Message, &recent_posts.Filename, &recent_posts.ThumbW, &recent_posts.ThumbH)
		if err != nil {
			error_log.Print(err.Error())
			return err.Error()
		}
		recent_posts_arr = append(recent_posts_arr, recent_posts)
	}

    page_data := &Wrapper{IName:"fronts", Data: front_arr}
    board_data := &Wrapper{IName:"boards", Data: boards_arr}
    section_data := &Wrapper{IName:"sections", Data: sections}
    recent_posts_data := &Wrapper{IName:"recent posts", Data: recent_posts_arr}
    

    var interfaces []interface{}
    interfaces = append(interfaces, config)
    interfaces = append(interfaces, page_data)
    interfaces = append(interfaces, board_data)
    interfaces = append(interfaces, section_data)
    interfaces = append(interfaces, recent_posts_data)

	wrapped := &Wrapper{IName: "frontpage",Data: interfaces}
	err = front_page_tmpl.Execute(front_file,wrapped)
	if err == nil {
		if err != nil {
			return err.Error()
		} else {
			return "Front page rebuilt successfully.<br />"
		}
	}
	return "Front page rebuilt successfully.<br />"	
}

func buildThread(op_id int, board_id int) (err error) {
	var posts []PostTable
	var post_table_interface []interface{}
	start_time := benchmarkTimer("buildThread" + string(op_id), time.Now(), true)

	rows,err := db.Query("SELECT * FROM `" + config.DBprefix + "posts` WHERE `deleted_timestamp` = '"+nil_timestamp+"' AND (`parentid` = "+strconv.Itoa(op_id)+" OR `id` = "+strconv.Itoa(op_id)+") AND `boardid` = "+strconv.Itoa(board_id))
	if err != nil {
		error_log.Print(err.Error())
		return
	}
	for rows.Next() {
		var post PostTable
		err = rows.Scan(&post.ID, &post.BoardID, &post.ParentID, &post.Name, &post.Tripcode, &post.Email, &post.Subject, &post.Message, &post.Password, &post.Filename, &post.FilenameOriginal, &post.FileChecksum, &post.Filesize, &post.ImageW, &post.ImageH, &post.ThumbW, &post.ThumbH, &post.IP, &post.Tag, &post.Timestamp, &post.Autosage, &post.PosterAuthority, &post.DeletedTimestamp, &post.Bumped, &post.Stickied, &post.Locked, &post.Reviewed, &post.Sillytag)
		if err != nil {
			error_log.Print(err.Error())
			return
		}
		posts = append(posts, post)
	}

	for _,post := range posts {
		post_msg_before := post.Message
		post.Message = parseBacklinks(post.Message, post.BoardID)
		if post_msg_before != post.Message {
			_,err := db.Exec("UPDATE `" + config.DBprefix + "posts` SET `message` = '" + post.Message + "' WHERE `id` = " + strconv.Itoa(post.ID))
			if err != nil {
				server.ServeErrorPage(writer, err.Error())
			}
		}
		post_table_interface = append(post_table_interface, post)
	}
	board_arr := getBoardArr("")
	sections_arr := getSectionArr("")

	var board_dir string
	for _,board_i := range board_arr {
		board := board_i

		if board.ID == board_id {
			board_dir = board.Dir

			break
		}
	}

    var interfaces []interface{}
    interfaces = append(interfaces, config)
    interfaces = append(interfaces, post_table_interface)
    var board_arr_i []interface{}
    for _,b := range board_arr {
    	board_arr_i = append(board_arr_i,b)
    }
    interfaces = append(interfaces, &Wrapper{IName:"boards", Data: board_arr_i})
    interfaces = append(interfaces, &Wrapper{IName:"sections", Data: sections_arr})

	wrapped := &Wrapper{IName: "threadpage",Data: interfaces}
	os.Remove(path.Join(config.DocumentRoot,board_dir+"/res/"+strconv.Itoa(op_id)+".html"))
	thread_file,err := os.OpenFile(path.Join(config.DocumentRoot,board_dir+"/res/"+strconv.Itoa(op_id)+".html"),os.O_CREATE|os.O_RDWR,0777)
	if err != nil {
		return err
	}

	defer func() {
		if _, ok := recover().(error); ok {
			error_log.Print("Failed executing template.")
		}
		if thread_file != nil {
			thread_file.Close()
		}
	}()
	err = img_thread_tmpl.Execute(thread_file,wrapped)
	benchmarkTimer("buildThread" + string(op_id), start_time, false)
	return err
}

// checks to see if the poster's tripcode/name is banned, if the IP is banned, or if the file checksum is banned
// returns true if the user is banned
func checkBannedStatus(post *PostTable, writer *http.ResponseWriter) ([]interface{}, error) {
	var is_expired bool
	var ban_entry BanlistTable
	// var count int
	// var search string

	err := db.QueryRow("SELECT `ip`, `name`, `tripcode`, `message`, `boards`, `timestamp`, `expires`, `appeal_at` FROM `" + config.DBprefix + "banlist` WHERE `ip` = '" + post.IP + "'").Scan(&ban_entry.IP,&ban_entry.Name,&ban_entry.Tripcode, &ban_entry.Message, &ban_entry.Boards, &ban_entry.Timestamp, &ban_entry.Expires, &ban_entry.AppealAt)
	var interfaces []interface{}

	if err != nil {
		if err == sql.ErrNoRows {
			// the user isn't banned
			// We don't need to return err because it isn't necessary
			return interfaces, nil

		} else {
			// something went wrong
			fmt.Println("something's wrong")
			return interfaces,err
		}
	} else {

		is_expired = ban_entry.Expires.After(time.Now()) == false

		if is_expired {
			// if it is expired, send a message saying that it's expired, but still post
			fmt.Println("expired")
			return interfaces,nil

		}
		// the user's IP is in the banlist. Check if the ban has expired
		if getSpecificSQLDateTime(ban_entry.Expires) == "0001-01-01 00:00:00" || ban_entry.Expires.After(time.Now()) {
			// for some funky reason, Go's MySQL driver seems to not like getting a supposedly nil timestamp as an ACTUAL nil timestamp
			// so we're just going to wing it and cheat. Of course if they change that, we're kind of hosed.
			
			var interfaces []interface{}
			interfaces = append(interfaces, config)
			interfaces = append(interfaces, ban_entry)
			return interfaces,nil
		}
		 return interfaces,nil
	}
	return interfaces, nil
}

func createThumbnail(image_obj image.Image, size string) image.Image {
	var thumb_width int
	var thumb_height int

	switch {
		case size == "op":
			thumb_width = config.ThumbWidth
			thumb_height = config.ThumbHeight
		case size == "reply":
			thumb_width = config.ThumbWidth_reply
			thumb_height = config.ThumbHeight_reply
		case size == "catalog":
			thumb_width = config.ThumbWidth_catalog
			thumb_height = config.ThumbHeight_catalog
	}
	old_rect := image_obj.Bounds()
	if thumb_width >= old_rect.Max.X && thumb_height >= old_rect.Max.Y {
		return image_obj
	}
	
	thumb_w,thumb_h := getThumbnailSize(old_rect.Max.X,old_rect.Max.Y,size)
	image_obj = imaging.Resize(image_obj, thumb_w, thumb_h, imaging.CatmullRom) // resize to 600x400 px using CatmullRom cubic filter
	return image_obj
}


func getFiletype(name string) string {
	filetype := strings.ToLower(name[len(name)-4:])
	if filetype == ".gif" {
		return "gif"
	} else if filetype == ".jpg" || filetype == "jpeg" {
		return "jpg"
	} else if filetype == ".png" {
		return "png"
	} else {
		return name[len(name)-3:]
	}
}

func getNewFilename() string {
	now := time.Now().Unix()
	rand.Seed(now)
	return strconv.Itoa(int(now))+strconv.Itoa(int(rand.Intn(98)+1))
}

// find out what out thumbnail's width and height should be, partially ripped from Kusaba X
func getThumbnailSize(w int, h int,size string) (new_w int, new_h int) {
	var thumb_width int
	var thumb_height int

	switch {
		case size == "op":
			thumb_width = config.ThumbWidth
			thumb_height = config.ThumbHeight
		case size == "reply":
			thumb_width = config.ThumbWidth_reply
			thumb_height = config.ThumbHeight_reply
		case size == "catalog":
			thumb_width = config.ThumbWidth_catalog
			thumb_height = config.ThumbHeight_catalog
	}
	if w == h {
		new_w = thumb_width
		new_h = thumb_height
	} else {
		var percent float32
		if (w > h) {
			percent = float32(thumb_width) / float32(w)
		} else {
			percent = float32(thumb_height) / float32(h)
		}
		new_w = int(float32(w) * percent)
		new_h = int(float32(h) * percent)
	}
	return
}

// inserts prepared post object into the SQL table so that it can be rendered
func insertPost(writer *http.ResponseWriter, post PostTable,bump bool) sql.Result {
	post_sql_str := "INSERT INTO `"+config.DBprefix+"posts` (`boardid`,`parentid`,`name`,`tripcode`,`email`,`subject`,`message`,`password`"
	if post.Filename != "" {
		post_sql_str += ",`filename`,`filename_original`,`file_checksum`,`filesize`,`image_w`,`image_h`,`thumb_w`,`thumb_h`"
	}
	post_sql_str += ",`ip`"
	post_sql_str += ",`timestamp`,`poster_authority`,"
	if post.ParentID == 0 {
		post_sql_str += "`bumped`,"
	}
	post_sql_str += "`stickied`,`locked`) VALUES("+strconv.Itoa(post.BoardID)+","+strconv.Itoa(post.ParentID)+",'"+post.Name+"','"+post.Tripcode+"','"+post.Email+"','"+post.Subject+"','"+post.Message+"','"+post.Password+"'"
	if post.Filename != "" {
		post_sql_str += ",'"+post.Filename+"','"+post.FilenameOriginal+"','"+post.FileChecksum+"',"+strconv.Itoa(int(post.Filesize))+","+strconv.Itoa(post.ImageW)+","+strconv.Itoa(post.ImageH)+","+strconv.Itoa(post.ThumbW)+","+strconv.Itoa(post.ThumbH)
	}
	post_sql_str += ",'"+post.IP+"','"+getSpecificSQLDateTime(post.Timestamp)+"',"+strconv.Itoa(post.PosterAuthority)+","
	if post.ParentID == 0 {
		post_sql_str += "'"+getSpecificSQLDateTime(post.Bumped)+"',"
	}
	if post.Stickied {
		post_sql_str += "1,"
	} else {
		post_sql_str += "0,"
	}
	if post.Locked {
		post_sql_str += "1);"
	} else {
		post_sql_str += "0);"
	}
	result,err := db.Exec(post_sql_str)
	if err != nil {
		server.ServeErrorPage(*writer,err.Error())
	}
	if post.ParentID != 0 {
		_,err := db.Exec("UPDATE `" + config.DBprefix + "posts` SET `bumped` = '" + getSpecificSQLDateTime(post.Bumped) + "' WHERE `id` = " + strconv.Itoa(post.ParentID))
		if err != nil {
			server.ServeErrorPage(*writer, err.Error())
		}
	}
	return result
}


func makePost(w http.ResponseWriter, r *http.Request, data interface{}) {
	start_time := benchmarkTimer("makePost", time.Now(), true)
	request = *r
	writer = w
	
	var post PostTable
	post.IName = "post"
	post.ParentID,_ = strconv.Atoi(request.FormValue("threadid"))
	post.BoardID,_ = strconv.Atoi(request.FormValue("boardid"))

	var count int
	var postid int
	var boardid int
	var email_command string

	err := db.QueryRow("SELECT (SELECT COUNT(*) FROM `"+config.DBprefix+"posts` WHERE `boardid` = "+strconv.Itoa(post.BoardID)+") AS `count`, `"+config.DBprefix+"posts`.`id` AS `id`, `"+config.DBprefix+"boards`.`id` AS `boardid` FROM `"+config.DBprefix+"posts`, `"+config.DBprefix+"boards` WHERE `boardid` = "+strconv.Itoa(post.BoardID)+" ORDER BY `"+config.DBprefix+"posts`.`id` DESC LIMIT 1").Scan(&count,&postid,&boardid)
	
	if err != nil {
		if err == sql.ErrNoRows {
			count = 0
		} else {
			error_log.Print(err.Error())
			server.ServeErrorPage(w, err.Error())
			return
		}
	}

	if count == 0 {
		var first_post int
		err = db.QueryRow("SELECT `first_post` FROM `"+config.DBprefix+"boards` WHERE `id` = "+strconv.Itoa(post.BoardID)+" LIMIT 1").Scan(&first_post)
		if err != nil {
			error_log.Print(err.Error())
			server.ServeErrorPage(w, err.Error())
			return
		}
		post.ID = first_post
	} else {
		post.ID = postid + 1
	}
	
	post_name := escapeString(request.FormValue("postname"))
	if strings.Index(post_name, "#") == -1 {
		post.Name = post_name
	} else if strings.Index(post_name, "#") == 0 {
		post.Tripcode = generateTripCode(post_name[1:])
	} else if strings.Index(post_name, "#") > 0 {
		post_name_arr := strings.SplitN(post_name,"#",2)
		post.Name = post_name_arr[0]
		post.Tripcode = generateTripCode(post_name_arr[1])
	}
	
	post_email := escapeString(request.FormValue("postemail"))
	if strings.Index(post_email,"noko") == -1 && strings.Index(post_email,"sage") == -1 {
		post.Email = html.EscapeString(escapeString(post_email))
	} else if strings.Index(post_email, "#") > 1 {
		post_email_arr := strings.SplitN(post_email,"#",2)
		post.Email = html.EscapeString(escapeString(post_email_arr[0]))
		email_command = post_email_arr[1]
	} else if post_email == "noko" || post_email == "sage" {
		email_command = post_email
		post.Email = ""
	}
	post.Subject = html.EscapeString(escapeString(request.FormValue("postsubject")))
	post.Message = escapeString(strings.Replace(html.EscapeString(request.FormValue("postmsg")), "\n", "<br />", -1))

	post.Message = parseBacklinks(post.Message, post.BoardID)
	post.Password = md5_sum(request.FormValue("postpassword"))
	post_name_cookie := strings.Replace(url.QueryEscape(post_name),"+", "%20", -1)
	url.QueryEscape(post_name_cookie)
	http.SetCookie(writer, &http.Cookie{Name: "name", Value: post_name_cookie, Path: "/", Domain: config.SiteDomain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})
	// http.SetCookie(writer, &http.Cookie{Name: "name", Value: post_name_cookie, Path: "/", Domain: config.Domain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})
	if email_command == "" {
		http.SetCookie(writer, &http.Cookie{Name: "email", Value: post.Email, Path: "/", Domain: config.SiteDomain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})
		// http.SetCookie(writer, &http.Cookie{Name: "email", Value: post.Email, Path: "/", Domain: config.Domain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})		
	} else {
		if email_command == "noko" {
			if post.Email == "" {
				http.SetCookie(writer, &http.Cookie{Name: "email", Value:"noko", Path: "/", Domain: config.SiteDomain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})						
				// http.SetCookie(writer, &http.Cookie{Name: "email", Value:"noko", Path: "/", Domain: config.Domain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})						
			} else {
				http.SetCookie(writer, &http.Cookie{Name: "email", Value: post.Email + "#noko", Path: "/", Domain: config.SiteDomain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})
				//http.SetCookie(writer, &http.Cookie{Name: "email", Value: post.Email + "#noko", Path: "/", Domain: config.Domain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})		
			}
		}
	}

	
	http.SetCookie(writer, &http.Cookie{Name: "password", Value: request.FormValue("postpassword"), Path: "/", Domain: config.SiteDomain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})	
	//http.SetCookie(writer, &http.Cookie{Name: "password", Value: request.FormValue("postpassword"), Path: "/", Domain: config.Domain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(31536000))),MaxAge: 31536000})

	post.IP = request.RemoteAddr
	post.Timestamp = time.Now()
	post.PosterAuthority = getStaffRank()
	post.Bumped = time.Now()
	post.Stickied = request.FormValue("modstickied") == "on"
	post.Locked = request.FormValue("modlocked") == "on"

	//post has no referrer, or has a referrer from a different domain, probably a spambot
	if !validReferrer(request) {
		access_log.Print("Rejected post from possible spambot @ : "+request.RemoteAddr)
		//TODO: insert post into temporary post table and add to report list
		return
	}

	file,handler,uploaderr := request.FormFile("imagefile")
	if uploaderr != nil {
		// no file was uploaded
		post.Filename = ""
		access_log.Print("Receiving post from "+request.RemoteAddr+", referred from: "+request.Referer())

	} else {
		data,err := ioutil.ReadAll(file)
		if err != nil {
			server.ServeErrorPage(w,"Couldn't read file")
		} else {
			post.FilenameOriginal = handler.Filename
			filetype := getFiletype(post.FilenameOriginal)
			thumb_filetype := filetype
			if thumb_filetype == "gif" {
				thumb_filetype = "jpg"
			}
			post.FilenameOriginal = escapeString(post.FilenameOriginal)
			post.Filename = getNewFilename()+"."+getFiletype(post.FilenameOriginal)
			board_arr := getBoardArr("`id` = "+request.FormValue("boardid"))
			if len(board_arr) == 0 {
				server.ServeErrorPage(w, "No boards have been created yet")
			}
			board_dir := getBoardArr("`id` = "+request.FormValue("boardid"))[0].Dir
			file_path := path.Join(config.DocumentRoot,"/"+board_dir+"/src/",post.Filename)
			thumb_path := path.Join(config.DocumentRoot,"/"+board_dir+"/thumb/",strings.Replace(post.Filename,"."+filetype,"t."+thumb_filetype,-1))
			catalog_thumb_path := path.Join(config.DocumentRoot,"/"+board_dir+"/thumb/",strings.Replace(post.Filename,"."+filetype,"c."+thumb_filetype,-1))


			err := ioutil.WriteFile(file_path, data, 0777)
			if err != nil {
				server.ServeErrorPage(w,"Couldn't write file.")
				return
			}

			img,err := imaging.Open(file_path)
			if err != nil {
				server.ServeErrorPage(w, "Upload filetype not supported")
				return
			} else {
				//post.FileChecksum string
				stat,err := os.Stat(file_path)
				if err != nil {
					server.ServeErrorPage(w,err.Error())
				} else {
					post.Filesize = int(stat.Size())
				}
				post.ImageW = img.Bounds().Max.X
				post.ImageH = img.Bounds().Max.Y
				if post.ParentID == 0 {
					post.ThumbW,post.ThumbH = getThumbnailSize(post.ImageW,post.ImageH,"op")	
				} else {
					post.ThumbW,post.ThumbH = getThumbnailSize(post.ImageW,post.ImageH,"reply")	
				}
				

				access_log.Print("Receiving post with image: "+handler.Filename+" from "+request.RemoteAddr+", referrer: "+request.Referer())

				if(request.FormValue("spoiler") == "on") {
					_,err := os.Stat(path.Join(config.DocumentRoot,"spoiler.png"))
					if err != nil {
						server.ServeErrorPage(w,"missing /spoiler.png")
						return
					} else {
						err = syscall.Symlink(path.Join(config.DocumentRoot,"spoiler.png"),thumb_path)
						if err != nil {
							server.ServeErrorPage(w,err.Error())
							return
						}
					}
				} else 	if config.ThumbWidth >= post.ImageW && config.ThumbHeight >= post.ImageH {
					post.ThumbW = img.Bounds().Max.X
					post.ThumbH = img.Bounds().Max.Y
					err := syscall.Symlink(file_path,thumb_path)
					if err != nil {
						server.ServeErrorPage(w,err.Error())
						return
					}
				} else {
					var thumbnail image.Image
					var catalog_thumbnail image.Image
					if post.ParentID == 0 {
						thumbnail = createThumbnail(img,"op")
						catalog_thumbnail = createThumbnail(img,"catalog")
						err = saveImage(catalog_thumb_path, &catalog_thumbnail)
						if err != nil {
							server.ServeErrorPage(w, err.Error())
							return
						}
					} else {
						thumbnail = createThumbnail(img,"reply")
					}
					err = saveImage(thumb_path, &thumbnail)
					if err != nil {
						server.ServeErrorPage(w, err.Error())
						return
					}

				}
			}
		}
	}

	if post.Message == "" && post.Filename == "" {
		server.ServeErrorPage(w,"Post must contain a message if no image is uploaded.")
		return
	}

	isbanned, err := checkBannedStatus(&post, &w)
	if err != nil {
		server.ServeErrorPage(w, err.Error())
		return
	}

	if len(isbanned) > 0 {
		post.IP = request.RemoteAddr
		wrapped := &Wrapper{IName: "bans",Data: isbanned}

		var banpage_buffer bytes.Buffer
		var banpage_html string
		banpage_buffer.Write([]byte(""))

		err = banpage_tmpl.Execute(&banpage_buffer,wrapped)
		if err != nil {
			fmt.Println(banpage_html)
			fmt.Fprintf(writer,banpage_html + err.Error() + "\n</body>\n</html>")
			return
		}
		fmt.Fprintf(w,banpage_buffer.String())

		return
	}

	result := insertPost(&w, post,email_command != "sage")
	if err != nil {
		server.ServeErrorPage(w, err.Error())
		return
	}
	id,_ := result.LastInsertId()

	parsed_backlinks:= parseBacklinks(post.Message, post.BoardID)
	if post.Message != parsed_backlinks {
		_,err := db.Exec("UPDATE `" + config.DBprefix + "posts` SET `message` = '" + post.Message + "' WHERE `id` = " + strconv.Itoa(int(id)))
		if err != nil {
			server.ServeErrorPage(writer, err.Error())
			return
		}
	}

	// rebuild the thread page
	if post.ParentID > 0 {
		buildThread(post.ParentID,post.BoardID)
	} else {
		buildThread(int(id),post.BoardID)
	}
	
	// rebuild the board page
	boards := getBoardArr("")
	sections := getSectionArr("")
	buildBoardPage(post.BoardID, boards, sections)

	buildFrontPage(boards, sections)

	if email_command == "noko" {
		if post.ParentID == 0 {
			http.Redirect(writer,&request,"/" + boards[post.BoardID-1].Dir + "/res/"+strconv.Itoa(post.ID)+".html",http.StatusFound)
		} else {
			http.Redirect(writer,&request, "/" + boards[post.BoardID-1].Dir + "/res/"+strconv.Itoa(post.ParentID)+".html",http.StatusFound)
		}
	} else {
		http.Redirect(writer,&request,"/" + boards[post.BoardID-1].Dir + "/",http.StatusFound)
	}
	benchmarkTimer("makePost", start_time, false)
}

func parseBacklinks(post string, boardid int) string {
	whitespace_regex, err := regexp.Compile(whitespace_match)
	var post_words []string
	if err != nil {
		// since the whitespace_match variable is built-in, there is no way this should happen, unless you mess with the code
		error_log.Print(err.Error())
		return post
	} else {
		post = strings.Replace(post,"<br />", "\n", -1)
		// split the post into indeividual words
		post_words = whitespace_regex.Split(post, -1)
	}

	gt := "&gt;"
	// go through each word and if it is a backlink, check to see if it points to a valid post
	for _,word := range post_words {
		var linked_post string
		if strings.Index(word,gt + gt) == 0 {
			if strings.Index(string(word[8:]), gt + gt) > 0 {
				// >>345435>>234, this may work on some imageboards, but it's bad and you shouldn't do that
				continue
			}

			linked_post = strings.Replace(word, gt + gt, "", -1)
			if linked_post == "" {
				// fmt.Println("empty")
				continue
			}

			linked_post = strings.Replace(linked_post, "\\r", "", -1)
			linked_post = strings.Replace(linked_post, "<br", "", -1)

			if string(linked_post[0]) == "/" {
				board_post_arr := strings.Split(linked_post,"/")
				if len(board_post_arr) == 3 {
					// >>/board/1234
				} else {
					// fmt.Println(">>11/11")
					// something like >>11/111
					continue
				}
			} else {
				_, err:= strconv.Atoi(linked_post)
				if err != nil {
					fmt.Println(">>letters:  " + linked_post)
					// something like >>letters
					continue
				}

				var parent_id string
				var board_dir string
				//err = db.QueryRow("SELECT `parentid`, `" + config.DBprefix + "boards`.`dir` as `boarddir` FROM `" + config.DBprefix + "posts`, `" + config.DBprefix + "boards` WHERE `deleted_timestamp` = '" + nil_timestamp + "' AND `id` = " + linked_post).Scan(&parent_id,&board_dir)
				err = db.QueryRow("SELECT `" + config.DBprefix + "boards`.`dir` AS boarddir, `" + config.DBprefix + "posts`.`parentid` AS parentid FROM `" + config.DBprefix + "posts`, `" + config.DBprefix + "boards` WHERE `" + config.DBprefix + "posts`.`deleted_timestamp` = \"" + nil_timestamp + "\"  AND `boardid` = `" + config.DBprefix + "boards`.`id` AND `" + config.DBprefix + "posts`.`id` = " + linked_post).Scan(&board_dir,&parent_id)

				if err == sql.ErrNoRows {
					// fmt.Println("post doesn't exist:  " +  linked_post)
					// post doesn't exist on this board
					// format the backlink with a strikethrough
					continue
				}

				if parent_id == "0" {
					// this is a thread
					post = strings.Replace(post,gt + gt + linked_post, "<a href=\"/" + board_dir + "/res/" + linked_post + ".html#" + linked_post + "\">&gt;&gt;" + linked_post + "</a>", -1)
				} else {
					post = strings.Replace(post, gt + gt + linked_post, "<a href=\"/" + board_dir + "/res/" + parent_id + ".html#" + linked_post + "\">&gt;&gt;" + linked_post + "</a>", -1)
				}
			}
		}
	}
	post = strings.Replace(post,"\n", "<br />", -1)
	return post
}


func shortenPostForBoardPage(post *string) {

}


func saveImage(path string, image_obj *image.Image) error {
	return imaging.Save(*image_obj, path)
}
