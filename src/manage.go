package main

import (
	"bytes"
	"code.google.com/p/go.crypto/bcrypt"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

type ManageFunction struct {
	Permissions int // 0 -> non-staff, 1 => janitor, 2 => moderator, 3 => administrator
	Callback func() string //return string of html output
}

func callManageFunction(w http.ResponseWriter, r *http.Request) {
	request = *r
	writer = w
	cookies = r.Cookies()
	request.ParseForm()
	action := request.FormValue("action")
	staff_rank := getStaffRank()
	var manage_page_buffer bytes.Buffer
	manage_page_html := ""

	if action == ""  {
		action = "announcements"
	}

	err := global_header_tmpl.Execute(&manage_page_buffer,config)
	if err != nil {
		fmt.Fprintf(writer,manage_page_html + err.Error() + "\n</body>\n</html>")
		return
	}

	err = manage_header_tmpl.Execute(&manage_page_buffer,config)
	if err != nil {
		fmt.Fprintf(writer,manage_page_html + err.Error() + "\n</body>\n</html>")
		return
	}

	if _,ok := manage_functions[action]; ok {
		if staff_rank >= manage_functions[action].Permissions {
			manage_page_buffer.Write([]byte(manage_functions[action].Callback()))
		} else if staff_rank == 0 && manage_functions[action].Permissions == 0 {
			manage_page_buffer.Write([]byte(manage_functions[action].Callback()))
		} else if staff_rank == 0 {
			manage_page_buffer.Write([]byte(manage_functions["login"].Callback()))
		} else {
			manage_page_buffer.Write([]byte(action + " is undefined."))
		}
	} else {
		manage_page_buffer.Write([]byte(action + " is undefined."))
	}
	manage_page_buffer.Write([]byte("\n</body>\n</html>"))
	fmt.Fprintf(writer,manage_page_buffer.String())
}

func getCurrentStaff() string {
	session_cookie := getCookie("sessiondata")
	var key string
	if session_cookie == nil {
		return ""
	} else {
		key = session_cookie.Value
	}

	results,err := db.Start("SELECT * FROM `"+config.DBprefix+"sessions` WHERE `key` = '"+key+"';")
	if err != nil {
		error_log.Write(err.Error())
		return ""
	}

	rows, err := results.GetRows()
    if err != nil {
		error_log.Write(err.Error())
		return ""
    }
	if len(rows) > 0 {
		for  _, row := range rows {
		    for col_num, col := range row {
				if col_num == 2 {
					return string(col.([]byte))
				}
		    }
		}
	} else {
		//session key doesn't exist in db
		return ""
	}
	return ""
}

func getStaffRank() int {
	var key string
	var staffname string

	db.Start("USE `"+config.DBname+"`")
	session_cookie := getCookie("sessiondata")
	if session_cookie == nil {
		return 0
	} else {
		key = session_cookie.Value
	}

  	results,err := db.Start("SELECT * FROM `"+config.DBprefix+"sessions` WHERE `key` = '"+key+"';")
	if err != nil {
		error_log.Write(err.Error())
		return 0
	}

	rows, err := results.GetRows()
    if err != nil {
		error_log.Write(err.Error())
		return 1
    }
	if len(rows) > 0 {
		for  _, row := range rows {
		    for col_num, col := range row {
				if col_num == 2 {
					staffname = string(col.([]byte))
				}
		    }
		}
	} else {
		//session key doesn't exist in db
		return 0
	}

  	results,err = db.Start("SELECT * FROM `"+config.DBprefix+"staff` WHERE `username` = '"+staffname+"';")
	if err != nil {
		error_log.Write(err.Error())
		return 0
	}

	rows, err = results.GetRows()
    if err != nil {
		error_log.Write(err.Error())
		return 1
    }
	if len(rows) > 0 {
		for  _, row := range rows {
		    for col_num, col := range row {
				if col_num == 4 {
					rank,rerr := strconv.Atoi(string(col.([]byte)))
					if rerr == nil {
						return rank
					} else {
						return 0
					}
				}
		    }
		}
	}
	return 0
}

func createSession(key string,username string, password string, request *http.Request, writer *http.ResponseWriter) int {
	//returs 0 for successful, 1 for password mismatch, and 2 for other
	//db.Start("USE `"+config.DBname+"`;")
  	results,err := db.Start("SELECT * FROM `"+config.DBprefix+"staff` WHERE `username` = '"+username+"';")

	if err != nil {
		error_log.Write(err.Error())
		return 2
	} else {
		rows, err := results.GetRows()
	    if err != nil {
			error_log.Write(err.Error())
			return 1
	    }
		if len(rows) > 0 {
			for _, row := range rows {
			    for col_num, col := range row {
			    	if col_num == 2 {
			    		success := bcrypt.CompareHashAndPassword(col.([]byte), []byte(password))
			    		if success == nil {
			    			// successful login
							cookie := &http.Cookie{Name: "sessiondata", Value: key, Path: "/", Domain:config.Domain, RawExpires: getSpecificSQLDateTime(time.Now().Add(time.Duration(time.Hour*2)))}
			    			http.SetCookie(*writer, cookie)
							_,err := db.Start("INSERT INTO `"+config.DBprefix+"sessions` (`key`, `data`, `expires`) VALUES('"+key+"','"+username+"', '"+getSpecificSQLDateTime(time.Now().Add(time.Duration(time.Hour*2)))+"');")
							if err != nil {
								error_log.Write(err.Error())
								return 2
							}
							_,err = db.Start("UPDATE `"+config.DBprefix+"staff` SET `last_active` ='"+getSQLDateTime()+"' WHERE `username` = '"+username+"';")
							if err != nil {
								error_log.Write(err.Error())
							}

							return 0
			    		} else if success == bcrypt.ErrMismatchedHashAndPassword {
			    			// password mismatch
			    			_,err := db.Start("INSERT `"+config.DBprefix+"loginattempts` (`ip`,`timestamp`) VALUES('"+request.RemoteAddr+"','"+getSQLDateTime()+"');")
			    			if err != nil {
			    				error_log.Write(err.Error())
			    			}
			    			return 1
			    		}
			    	}
				}
			}
		} else {
			//username doesn't exist
			return 1
		}
	}
	return 1
}

var manage_functions = map[string]ManageFunction{
	"initialsetup": {
		Permissions: 0,
		Callback: func() string {
			html,_ := ioutil.ReadFile(config.DocumentRoot+"/index.html")
			return string(html)
	}},
	"error": {
		Permissions: 0,
		Callback: func() (html string) {
			exitWithErrorPage("lel, internet")
			return
	}},
	"login":{
		Permissions: 0,
		Callback: func() (html string) {
			username := request.FormValue("username")
			password := request.FormValue("password")

			if username == "" || password == "" {
				//assume that they haven't logged in
				html = "\t<form method=\"POST\" action=\"/manage?action=login\" class=\"loginbox\">\n" +
					//"\t\t<input type=\"hidden\" name=\"action\" value=\"login\" />\n" +
					"\t\t<input type=\"text\" name=\"username\" class=\"logindata\" /><br />\n" +
					"\t\t<input type=\"password\" name=\"password\" class=\"logindata\" /> <br />\n" +
					"\t\t<input type=\"submit\" value=\"Login\" />\n" +
					"\t</form>"
			} else {
				key := md5_sum(request.RemoteAddr+username+password+config.RandomSeed+generateSalt())[0:10]
				createSession(key,username,password,&request,&writer)
				redirect(path.Join(config.SiteWebfolder,"/manage?action=announcements"))

			}
			return
	}},
	"announcements": {
		Permissions: 1,
		Callback: func() (html string) {
			html = "<h1>Announcements</h1><br />"
			var subject string
			var message string
			var poster string
			var timestamp string

		  	results,err := db.Start("SELECT `subject`,`message`,`poster`,`timestamp` FROM `"+config.DBprefix+"announcements`;")
			if err != nil {
				error_log.Write(err.Error())
				html += err.Error()
				return
			}

			rows, err := results.GetRows()
		    if err != nil {
				error_log.Write(err.Error())
				html += err.Error()
				return
		    }
			if len(rows) > 0 {
				for  _, row := range rows {
				    for col_num, col := range row {
						switch {
							case col_num == 0:
								subject = string(col.([]byte))
							case col_num == 1:
								message = string(col.([]byte))
							case col_num == 2:
								poster = string(col.([]byte))
							case col_num == 3:
								timestamp = string(col.([]byte))
						}
				    }
				    html += "<div class=\"section-block\">"+subject+"</div>\n"
				    html += "<div class=\"section-block\">"+message+"</div>\n"
				    html += "<div class=\"section-block\">"+poster+"</div>\n"
				    html += "<div class=\"section-block\">"+timestamp+"</div>\n"
				}
			} else {
				html += "No announcements"
			}
		return
	}},
	"manageserver": {
		Permissions: 3,
		Callback: func() (html string) {
			html = "<script type=\"text/javascript\">\n$jq = jQuery.noConflict();\n$jq(document).ready(function() {\n\tvar killserver_btn = $jq(\"button#killserver\");\n\n\t$jq(\"button#killserver\").click(function() {\n\t\t$jq.ajax({\n\t\t\tmethod:'GET',\n\t\t\turl:\"/manage\",\n\t\t\tdata: {\n\t\t\t\taction: 'killserver'\n\t\t\t},\n\n\t\t\tsuccess: function() {\n\t\t\t\t\n\t\t\t},\n\t\t\terror:function() {\n\t\t\t\t\n\t\t\t}\n\t\t});\n\t});\n});\n</script>" +
			"<button id=\"killserver\">Kill server</button><br />\n"

			return
	}},
	"cleanup": {
		Permissions:3,
		Callback: func() (html string) {

			return
	}},
	"getstaffjquery": {
		Permissions:0,
		Callback: func() (html string) {
			current_staff := getCurrentStaff()
			staff_rank := getStaffRank()
			if staff_rank == 0 {
				html = "nobody;0;"
				return
			}
			staff_boards := ""
		  	results,err := db.Start("SELECT * FROM `"+config.DBprefix+"staff`;")
			if err != nil {
				error_log.Write(err.Error())
				html += err.Error()
				return
			}

			rows, err := results.GetRows()
		    if err != nil {
				error_log.Write(err.Error())
				html += err.Error()
				return
		    }
			if len(rows) > 0 {
				for  _, row := range rows {

				    for col_num, col := range row {
						if col_num == 5 {
							staff_boards = string(col.([]byte))
						}
				    }
				}
			} else {
				// fuck you, I'm Spiderman.
			}
			html = current_staff+";"+strconv.Itoa(staff_rank)+";"+staff_boards
			return
	}},
	"manageboards": {
		Permissions:3,
		Callback: func() (html string) {
			html = "<h1>Manage boards</h1>\n<select name=\"boardselect\">\n<option>Select board...</option>\n"
			db.Start("USE `"+config.DBname+"`;")
		 	results,err := db.Start("SELECT `dir` FROM `"+config.DBprefix+"boards`;")
			if err != nil {
				html += err.Error()
				return
			}


			rows, err := results.GetRows()
		    if err != nil {
				error_log.Write(err.Error())
				html += err.Error()
				return
		    }
			if len(rows) > 0 {
				for  _, row := range rows {
			    	for _, col := range row {
		    			html += "<option>"+string(col.([]byte))+"</option>\n"
					}
			
				}
			}
			html += "</select><hr />"
			return
	}},
	"staffmenu": {
		Permissions:1,
		Callback: func() (html string) {
			rank := getStaffRank()

			html = "<a href=\"javascript:void(0)\" id=\"logout\" class=\"staffmenu-item\">Log out</a><br />\n" +
				   "<a href=\"javascript:void(0)\" id=\"announcements\" class=\"staffmenu-item\">Announcements</a><br />\n"
			if rank == 3 {
			  	html += "<b>Admin stuff</b><br />\n<a href=\"javascript:void(0)\" id=\"staff\" class=\"staffmenu-item\">Manage staff</a><br />\n" +
					  	"<a href=\"javascript:void(0)\" id=\"rebuildfront\" class=\"staffmenu-item\">Rebuild front page</a><br />\n" +
					  	"<a href=\"javascript:void(0)\" id=\"manageboards\" class=\"staffmenu-item\">Add/edit/delete boards</a><br />\n"
			}
			if rank >= 2 {
				html += "<b>Mod stuff</b><br />\n"
			}

			if rank >= 1 {
				html += "<a href=\"javascript:void(0)\" id=\"recentimages\" class=\"staffmenu-item\">Recently uploaded images</a><br />\n" +
						"<a href=\"javascript:void(0)\" id=\"recentposts\" class=\"staffmenu-item\">Recent posts</a><br />\n" +
						"<a href=\"javascript:void(0)\" id=\"searchip\" class=\"staffmenu-item\">Search posts by IP</a><br />\n"
			}

			return
	}},
	"rebuildfront": {
		Permissions: 3,
		Callback: func() (html string) {
			initTemplates()
			// variables for sections table
			var section_id int
			var section_order int
			var section_hidden bool
			var section_arr []interface{}

			// variables for board
			var board_dir string
			var board_title string
			var board_subtitle string
			var board_description string
			var board_section int
			var board_arr []interface{}

			// variables for frontpage table
			var front_page int
			var front_order int
			var front_subject string
			var front_message string
			var front_timestamp string
			var front_poster string
			var front_email string
			var front_arr []interface{}

			os.Remove("html/index.html")
			front_file,err := os.OpenFile("html/index.html",os.O_CREATE|os.O_RDWR,0777)
			defer func() {
				front_file.Close()
			}()
			if err != nil {
				return err.Error()
			}

			// get boards from db and push to variables to be put in an interface
		  	results,err := db.Start("SELECT `dir`,`title`,`subtitle`,`description`,`section` FROM `"+config.DBprefix+"boards` ORDER BY `order`;")
			if err != nil {
				error_log.Write(err.Error())
				return err.Error()
			}
			rows,err := results.GetRows()
			if err != nil {
				error_log.Write(err.Error())
				return err.Error()
			}

			for _,row := range rows {
			    for col_num, col := range row {
					switch {
						case col_num == 0:
							board_dir = string(col.([]byte))
						case col_num == 1:
							board_title = string(col.([]byte))
						case col_num == 2:
							board_subtitle = string(col.([]byte))
						case col_num == 3:
							board_description = string(col.([]byte))
						case col_num == 4:
							board_section,_ = strconv.Atoi(string(col.([]byte)))

					}
			    }

			    board_arr = append(board_arr,BoardsTable{IName:"board", Dir:board_dir, Title:board_title, Subtitle:board_subtitle, Description:board_description, Section:board_section})
			}

			// get sections from db and push to variables to be put in an interface
		  	results,err = db.Start("SELECT `id`,`order`,`hidden` FROM `"+config.DBprefix+"sections` ORDER BY `order`;")
			if err != nil {
				error_log.Write(err.Error())
				return err.Error()
			}
			rows,err = results.GetRows()
			if err != nil {
				error_log.Write(err.Error())
				return err.Error()
			}

			for _,row := range rows {
			    for col_num, col := range row {
					switch {
						case col_num == 0:
							section_id,_ = strconv.Atoi(string(col.([]byte)))
						case col_num == 1:
							section_order,_ = strconv.Atoi(string(col.([]byte)))
						case col_num == 2:
							b,_ := strconv.Atoi(string(col.([]byte)))
							if b == 1 {
								section_hidden = true
							} else {
								section_hidden = false
							}
					}
			    }
			    section_arr = append(section_arr, BoardSectionsTable{IName: "section", ID: section_id, Order: section_order, Hidden: section_hidden})
			}

			// get front pages
			results,err = db.Start("SELECT * FROM `"+config.DBprefix+"frontpage`;")
			if err != nil {
				error_log.Write(err.Error())
				return err.Error()
			}

			rows, err = results.GetRows()
		    if err != nil {
				error_log.Write(err.Error())
				return err.Error()
		    }
			if len(rows) > 0 {
				for row_num, row := range rows {
				    for col_num, col := range row {
				    	switch {
				    		case col_num == 1:
				    			front_page,_ = strconv.Atoi(string(col.([]byte)))
				    		case col_num == 2:
				    			front_order,_ = strconv.Atoi(string(col.([]byte)))
				    		case col_num == 3:
				    			front_subject = string(col.([]byte))
				    		case col_num == 4:
				    			front_message = string(col.([]byte))
				    		case col_num == 5:
				    			front_timestamp = string(col.([]byte))
				    		case col_num == 6:
				    			front_poster = string(col.([]byte))
				    		case col_num == 7:
				    			front_email = string(col.([]byte))
				    	}
				    }
					front_arr = append(front_arr,FrontTable{IName:"front page", ID:row_num, Page: front_page, Order: front_order, Subject: front_subject, Message: front_message, Timestamp: front_timestamp, Poster: front_poster, Email: front_email})
				}
			} else {
				// no front pages
			}

		    page_data := &Wrapper{IName:"fronts", Data: front_arr}
		    board_data := &Wrapper{IName:"boards", Data: board_arr}
		    section_data := &Wrapper{IName:"sections", Data: section_arr}

		    var interfaces []interface{}
		    interfaces = append(interfaces, config)
		    interfaces = append(interfaces, page_data)
		    interfaces = append(interfaces, board_data)
		    interfaces = append(interfaces, section_data)

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
	}},
	"rebuildall": {
		Permissions:3,
		Callback: func() (html string) {
			initTemplates()
			return
	}},
	"recentposts": {
		Permissions:1,
		Callback: func() (html string) {
			html = "<h1>Recent posts</h1>\n<table style=\"border:2px solid;\">\n<tr><td>bleh</td><td>bleh bleh</td></tr>" +
			"</table>"
			return
	}},
	"killserver": {
		Permissions:3,
		Callback: func() (html string) {
			os.Exit(0)
			return
	}},
	"staff": {
		Permissions:3,
		Callback: func() (html string) {
			//do := request.FormValue("do")
			html = "<h1>Staff</h1><br />\n" +
					"<table border=\"1\"><tr><td><b>Username</b></td><td><b>Rank</b></td><td><b>Boards</b></td><td><b>Added on</b></td><td><b>Action</b></td></tr>\n"
			db.Start("USE `"+config.DBname+"`;")
		 	results,err := db.Start("SELECT `username`,`rank`,`boards`,`added_on` FROM `"+config.DBprefix+"staff`;")
			if err != nil {
				html += "<tr><td>"+err.Error()+"</td></tr></table>"
				return
			}

			row_num := 0
			for {
			    row, err := results.GetRow()
		        if err != nil {
					html += "<tr><td>"+err.Error()+"</td></tr></table>"
					return
		        }

		        if row == nil {
		            break
		        }
		        html  += "<tr>"
			    for col_num, col := range row {
			    	if col_num == 1 {
			    		rank := string(col.([]byte))
			    		if rank == "3" {
			    			rank = "admin"
			    		} else if rank == "2" {
			    			rank = "mod"
			    		} else if rank == "1" {
			    			rank = "janitor"
			    		}
			    		html += "<td>"+rank+"</td>"	
			    	} else {
			    		html += "<td>"+string(col.([]byte))+"</td>"
			    	}
				}
				
				html += "<td><a href=\"action=staff%26do=del%26index="+strconv.Itoa(row_num)+"\" style=\"float:right;color:red;\">X</a></td></tr>\n"
			    
			}
			html += "</table>"
			return
	}},
}