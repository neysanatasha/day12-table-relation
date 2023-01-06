package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"my-project/config"
	"my-project/middleware"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

// Project Struct
type Project struct {
	ID           int
	ProjectName  string
	StartDate    time.Time
	EndDate      time.Time
	Duration     string
	Description  string
	Technologies []string
	Image        string
	UserId       int
}
type MetaData struct {
	Id        int
	IsLogin   bool
	UserName  string
	FlashData string
}

var Data = MetaData{}

type User struct {
	Id       int
	Name     string
	Email    string
	Password string
}

func main() {
	route := mux.NewRouter()

	// Connect to Database
	config.DatabaseConnect()

	// for public folder
	// ex: localhost:port/public/ +../path/to/file
	route.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))

	route.HandleFunc("/", home).Methods("GET")
	route.HandleFunc("/contact", contact).Methods("GET")

	// CRUD Project
	// create
	route.HandleFunc("/create", createProject).Methods("GET")
	route.HandleFunc("/create", middleware.UploadFile(storeProject)).Methods("POST")
	// read
	route.HandleFunc("/detail/{id}", detailProject).Methods("GET")
	// update
	route.HandleFunc("/edit/{id}", editProject).Methods("GET")
	route.HandleFunc("/edit/{id}", middleware.UploadFile(updateProject)).Methods("POST")
	// delete
	route.HandleFunc("/delete/{id}", deleteProject).Methods("GET")

	// Auth
	// Register
	route.HandleFunc("/register", registerForm).Methods("GET")
	route.HandleFunc("/register", register).Methods("POST")
	// Login
	route.HandleFunc("/login", loginForm).Methods("GET")
	route.HandleFunc("/login", login).Methods("POST")
	// Logout
	route.HandleFunc("/logout", logout).Methods("GET")

	port := "5050"
	fmt.Println("Server berjalan pada port", port)
	http.ListenAndServe("localhost:"+port, route)
}

// home
func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/index.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	fm := session.Flashes("message")

	var flashes []string
	if len(fm) > 0 {
		session.Save(r, w)
		for _, fl := range fm {
			flashes = append(flashes, fl.(string))
		}
	}
	Data.FlashData = strings.Join(flashes, "")

	// Project slice to hold data from returned rowsData.
	var resultData []Project

	if session.Values["IsLogin"] != true {
		Data.IsLogin = false
		// Query to database
		rowsData, err := config.Conn.Query(context.Background(), "SELECT id, project_name, start_date, end_date, description, technologies, image FROM tb_projects")
		if err != nil {
			fmt.Println("Message : 1" + err.Error())
			return
		}

		// Loop through rowsData, using Scan to assign column data to struct fields.
		for rowsData.Next() {
			var each = Project{}
			err := rowsData.Scan(&each.ID, &each.ProjectName, &each.StartDate, &each.EndDate, &each.Description, &each.Technologies, &each.Image)
			if err != nil {
				fmt.Println("Message : 2" + err.Error())
				return
			}
			// add Duration result from calc of StartDate and EndDate
			each.Duration = config.GetDurationTime(each.StartDate, each.EndDate)
			// Append to Project slice
			resultData = append(resultData, each)
		}
		// for more database query: https://go.dev/doc/database/querying

		// fmt.Println(resultData)
	} else {
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)
		Data.Id = session.Values["Id"].(int)
		// user_id := session.Values["Id"].(int)
		resultData = []Project{}
		rowsData, err := config.Conn.Query(context.Background(), "SELECT tb_projects.id, project_name, start_date, end_date, description, technologies, image FROM tb_projects LEFT JOIN tb_users ON tb_projects.user_id = tb_users.id where tb_projects.user_id =$1", Data.Id)
		if err != nil {
			fmt.Println("Message : 3" + err.Error())
			return
		}

		// Loop through rowsData, using Scan to assign column data to struct fields.
		for rowsData.Next() {
			var each = Project{}
			err := rowsData.Scan(&each.ID, &each.ProjectName, &each.StartDate, &each.EndDate, &each.Description, &each.Technologies, &each.Image)
			if err != nil {
				fmt.Println("Message : 4" + err.Error())
				return
			}
			// add Duration result from calc of StartDate and EndDate
			each.Duration = config.GetDurationTime(each.StartDate, each.EndDate)
			// Append to Project slice
			resultData = append(resultData, each)
		}
	}

	data := map[string]interface{}{
		"Projects": resultData,
		"Data":     Data,
	}

	tmpt.Execute(w, data)
}

//
// CRUD Project
//

// create - createProject
func createProject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/create-project.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] != true {
		Data.IsLogin = false
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	} else {
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)

	}
	data := map[string]interface{}{
		"Data": Data,
	}
	tmpt.Execute(w, data)
}

// create - storeProject
func storeProject(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()

	if err != nil {
		log.Fatal(err)
	}

	project_name := r.PostForm.Get("project_name")
	technologies := r.Form["technologies"]
	description := r.PostForm.Get("description")

	// Image
	dataContext := r.Context().Value("dataFile")
	image_path := dataContext.(string)

	// Date
	const (
		layoutISO = "2006-01-02"
	)
	start_date, _ := time.Parse(layoutISO, r.PostForm.Get("start_date"))
	end_date, _ := time.Parse(layoutISO, r.PostForm.Get("end_date"))

	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] != true {
		Data.IsLogin = false
	} else {
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)
	}
	// fmt.Println(Data.Id)

	user_id := session.Values["Id"].(int)

	_, err = config.Conn.Exec(context.Background(), "INSERT INTO tb_projects(project_name, start_date, end_date, technologies, description, image, user_id) VALUES ($1, $2, $3, $4, $5, $6, $7)", project_name, start_date, end_date, technologies, description, image_path, user_id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Message: " + err.Error()))
		return
	}

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

// read
// detailProject
func detailProject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/detail-project.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var resultData = Project{}

	err = config.Conn.QueryRow(context.Background(), "SELECT * FROM tb_projects WHERE id=$1", id).Scan(
		&resultData.ID, &resultData.ProjectName, &resultData.StartDate, &resultData.EndDate, &resultData.Description, &resultData.Technologies, &resultData.Image, &resultData.UserId,
	)
	// add Duration result from calc of StartDate and EndDate
	resultData.Duration = config.GetDurationTime(resultData.StartDate, resultData.EndDate)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// fmt.Println(resultData)
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] != true {
		Data.IsLogin = false
	} else {
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)

	}
	data := map[string]interface{}{
		"Project": resultData,
		"Data":    Data,
	}
	tmpt.Execute(w, data)
}

// update - editProject
func editProject(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/edit-project.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] != true {
		Data.IsLogin = false
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	} else {
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)

	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var resultData = Project{}

	err = config.Conn.QueryRow(context.Background(), "SELECT * FROM tb_projects WHERE id=$1", id).Scan(
		&resultData.ID, &resultData.ProjectName, &resultData.StartDate, &resultData.EndDate, &resultData.Description, &resultData.Technologies, &resultData.Image, &resultData.UserId,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// fmt.Println(resultData)

	data := map[string]interface{}{
		"Project": resultData,
		"Data":    Data,
	}
	tmpt.Execute(w, data)
}

// update - updateProject
func updateProject(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()

	if err != nil {
		log.Fatal(err)
	}

	project_name := r.PostForm.Get("project_name")
	technologies := r.Form["technologies"]
	description := r.PostForm.Get("description")

	// Image
	dataContext := r.Context().Value("dataFile")
	image_path := dataContext.(string)

	// Date
	const (
		layoutISO = "2006-01-02"
	)
	start_date, _ := time.Parse(layoutISO, r.PostForm.Get("start_date"))
	end_date, _ := time.Parse(layoutISO, r.PostForm.Get("end_date"))
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	_, err = config.Conn.Exec(context.Background(), "UPDATE tb_projects SET project_name = $1, start_date = $2, end_date = $3, technologies = $4, description = $5, image = $6 WHERE id = $7", project_name, start_date, end_date, technologies, description, image_path, id)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Message: " + err.Error()))
		return
	}

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

// delete
func deleteProject(w http.ResponseWriter, r *http.Request) {

	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	_, err := config.Conn.Exec(context.Background(), "DELETE FROM tb_projects WHERE id=$1", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// Auth
// register - registerForm
func registerForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/register.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] == true {
		Data.IsLogin = true
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
	tmpt.Execute(w, nil)
}

// register
func register(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Fatal(err)
	}

	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")

	password := r.PostForm.Get("password")
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(password), 10)

	_, err = config.Conn.Exec(context.Background(), "INSERT INTO tb_users(name, email, password) VALUES ($1, $2, $3)", name, email, passwordHash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Message: " + err.Error()))
		return
	}

	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	session.AddFlash("Successfully registered!", "message")

	session.Save(r, w)

	http.Redirect(w, r, "/login", http.StatusMovedPermanently)
}

// login - loginForm
func loginForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/login.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}

	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] == true {
		Data.IsLogin = true
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	tmpt.Execute(w, nil)
}

// login - login
func login(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Fatal(err)
	}

	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")

	user := User{}

	err = config.Conn.QueryRow(context.Background(), "SELECT * FROM tb_users WHERE email=$1", email).Scan(
		&user.Id, &user.Name, &user.Email, &user.Password,
	)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Message: " + err.Error()))
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	session.Values["IsLogin"] = true
	session.Values["Name"] = user.Name
	session.Values["Id"] = user.Id
	session.Options.MaxAge = 10800 // 3 hours

	session.AddFlash("Successfully login!", "message")
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

// Logout
func logout(w http.ResponseWriter, r *http.Request) {
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")
	session.Options.MaxAge = -1

	session.Save(r, w)

	// fmt.Println("Logout")

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// contact
func contact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	tmpt, err := template.ParseFiles("views/contact.html")

	if err != nil {
		w.Write([]byte("Message: " + err.Error()))
		return
	}
	// Session
	var store = sessions.NewCookieStore([]byte("SESSIONS_ID"))
	session, _ := store.Get(r, "SESSIONS_ID")

	if session.Values["IsLogin"] != true {
		Data.IsLogin = false
	} else {
		Data.IsLogin = session.Values["IsLogin"].(bool)
		Data.UserName = session.Values["Name"].(string)
	}
	data := map[string]interface{}{
		"Data": Data,
	}

	tmpt.Execute(w, data)
}
