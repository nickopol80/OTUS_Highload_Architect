package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"

	_ "github.com/go-sql-driver/mysql"
)

type Article struct {
	Id                         uint16
	Title, Anons, Text, UserId string
}

type User struct {
	Id, Age, Name, Surname, Sex, City, Hobbies, Email, Password string
	IsAuthorized                                                bool
}

type DataPage struct {
	Customer User
	Posts    []Article
}

type DataFormPage struct {
	Customer User
	Users    []User
}

var article = Article{}
var userInfo = User{}
var customer = User{}

func index(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/index.html", "templates/header.html",
		"templates/footer.html", "templates/login.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	//Выборка данных
	res, err := db.Query("select * from articles;")
	if err != nil {
		panic(err)
	}

	var posts = []Article{} //Чтоб не дублировались одни и теже посты при обновлении страницы
	for res.Next() {
		var post Article
		err = res.Scan(&post.Id, &post.Title, &post.Anons, &post.Text, &post.UserId)
		if err != nil {
			panic(err)
		}

		posts = append(posts, post)
	}

	//fmt.Printf("index --> user : %v\n", customer)
	var data = DataPage{Customer: customer, Posts: posts}
	t.ExecuteTemplate(w, "index", data)
	//t.ExecuteTemplate(w, "index", posts)
}

func usersForms(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/usersForms.html", "templates/header.html",
		"templates/footer.html", "templates/login.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	//Выборка данных
	res, err := db.Query("select * from users;")
	if err != nil {
		panic(err)
	}

	var usersForms = []User{} //Чтоб не дублировались одни и теже посты при обновлении страницы
	for res.Next() {
		var user User
		err = res.Scan(&user.Id, &user.Name, &user.Age, &user.Surname, &user.Sex, &user.City, &user.Hobbies, &user.Email, &user.Password)
		if err != nil {
			panic(err)
		}

		usersForms = append(usersForms, user)
	}

	//fmt.Printf("index --> user : %v\n", customer)
	var dataUsersForms = DataFormPage{Customer: customer, Users: usersForms}
	t.ExecuteTemplate(w, "usersForms", dataUsersForms)
}

func create(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/create.html", "templates/header.html", "templates/footer.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	//fmt.Printf("create --> user IsAuthorized: %v\n", customer.IsAuthorized)
	t.ExecuteTemplate(w, "create", customer)
}

func showPost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// w.WriteHeader(http.StatusOK)
	// fmt.Fprintf(w, "Id: %v\n", vars["id"])

	t, err := template.ParseFiles("templates/show.html", "templates/header.html", "templates/footer.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	//Выборка данных
	res, err := db.Query(fmt.Sprintf("SELECT * FROM articles WHERE id = '%s';", vars["id"]))
	if err != nil {
		panic(err)
	}

	article = Article{}
	for res.Next() {
		var post Article
		err = res.Scan(&post.Id, &post.Title, &post.Anons, &post.Text, &post.UserId)
		if err != nil {
			panic(err)
		}

		article = post
	}
	//fmt.Printf("showPost --> user : %v\n", customer)
	t.ExecuteTemplate(w, "show", article)
}

func showUserForm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r) //Вытаскиваем все параметры из запроса
	// w.WriteHeader(http.StatusOK)
	// fmt.Fprintf(w, "Id: %v\n", vars["id"])

	t, err := template.ParseFiles("templates/userForm.html", "templates/header.html", "templates/footer.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	//Выборка данных
	res, err := db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = '%s';", vars["id"]))
	if err != nil {
		panic(err)
	}

	userInfo = User{}
	for res.Next() {
		var user User
		err = res.Scan(&user.Id, &user.Name, &user.Age, &user.Surname, &user.Sex, &user.City, &user.Hobbies, &user.Email, &user.Password)
		if err != nil {
			panic(err)
		}

		userInfo = user
	}

	userInfo.IsAuthorized = customer.IsAuthorized
	//fmt.Printf("showPost --> user : %v\n", customer)
	t.ExecuteTemplate(w, "userForm", userInfo)
}

func saveArticle(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	anons := r.FormValue("anons")
	text := r.FormValue("text")
	userId := customer.Id

	if title == "" || anons == "" || text == "" {
		fmt.Fprintf(w, "Не все данные заполненны")
	} else {
		db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
		if err != nil {
			panic(err)
		}

		defer db.Close()

		//Установка данных
		insert, err := db.Query(fmt.Sprintf("INSERT INTO articles (title, anons, text, user_id) VALUES('%s', '%s', '%s', '%s');", title, anons, text, userId))
		if err != nil {
			panic(err)
		}

		defer insert.Close()
	}

	//fmt.Printf("saveArticle --> user : %v\n", customer)
	// http.Redirect(w, r, "/", 301) //можно и так указать код ответа = 301
	http.Redirect(w, r, "#", http.StatusSeeOther)
}

func login(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/login.html", "templates/header.html", "templates/footer.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	//fmt.Printf("login --> user : %v\n", customer)
	t.ExecuteTemplate(w, "login", customer)
}

func logout(w http.ResponseWriter, r *http.Request) {
	customer.IsAuthorized = false
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	emailForm := r.FormValue("email")
	passwordForm := r.FormValue("password")
	//fmt.Printf("\nemail : %s\n", email)

	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	//Проверка наличие зарегистрированного пользователя
	res, err := db.Query(fmt.Sprintf("SELECT * FROM users WHERE email = '%s';", emailForm))
	if err != nil {
		panic(err)
	}

	//customer = User{}
	for res.Next() {
		var user User
		err = res.Scan(&user.Id, &user.Name, &user.Age, &user.Surname, &user.Sex, &user.City, &user.Hobbies, &user.Email, &user.Password)
		if err != nil {
			panic(err)
		}
		customer = user
	}

	if emailForm != customer.Email {
		//fmt.Fprintf(w, "email : %s - не найден. Зарегистрируйтесь\n", emailForm)
		fmt.Printf("\nemail : %s - не найден. Зарегистрируйтесь\n", emailForm)
		t, err := template.ParseFiles("templates/registrationForm.html", "templates/header.html", "templates/footer.html")
		if err != nil {
			fmt.Fprintln(w, err.Error())
		}

		t.ExecuteTemplate(w, "registrationForm", nil)
	} else if passwordForm != customer.Password {
		//fmt.Fprintf(w, "email : %s - не верный пароль\n", emailForm)
		fmt.Printf("\nemail : %s - не верный пароль\n", emailForm)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	} else {
		customer.IsAuthorized = true
		//fmt.Fprintf(w, "user : %v\n", customer)
		//fmt.Printf("user : %v\n", customer)
		//fmt.Printf("getUser --> user : %v\n", customer)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func registrationForm(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/registrationForm.html", "templates/header.html", "templates/footer.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	//fmt.Printf("registrationForm --> user : %v\n", customer)
	t.ExecuteTemplate(w, "registrationForm", nil)
}

func registration(w http.ResponseWriter, r *http.Request) {
	userName := r.FormValue("name")
	userAge := r.FormValue("age")
	userSurname := r.FormValue("surname")
	userSex := r.FormValue("sex")
	userCity := r.FormValue("city")
	userHobbies := r.FormValue("hobbies")
	userEmail := r.FormValue("email")
	userPassword := r.FormValue("password")

	if userName == "" || userAge == "" || userSurname == "" || userSex == "" || userCity == "" || userHobbies == "" || userEmail == "" || userPassword == "" {
		fmt.Fprintf(w, "Не все данные заполненны")
	} else {
		db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
		if err != nil {
			panic(err)
		}

		defer db.Close()

		//Установка данных
		insert, err := db.Query(fmt.Sprintf("INSERT INTO users (`name`, `age`, `surname`, `sex`, `city`, `hobbies`, `email`, `password`) VALUES('%s', '%s', '%s', '%s','%s', '%s', '%s', '%s');",
			userName, userAge, userSurname, userSex, userCity, userHobbies, userEmail, userPassword))
		if err != nil {
			panic(err)
		}

		defer insert.Close()
	}

	//fmt.Printf("registration --> user : %v\n", customer)
	http.Redirect(w, r, "#", http.StatusSeeOther)
}

func handleFunc() {
	rtr := mux.NewRouter()
	fmt.Println("Сервер запущен на http://localhost:8080")
	rtr.HandleFunc("/", index).Methods("GET") //указав Methods("GET") мы защищаем наш сервер от ввода запросов ч другими методами
	rtr.HandleFunc("/usersForms", usersForms).Methods("GET")
	rtr.HandleFunc("/create", create).Methods("GET")
	rtr.HandleFunc("/login", login)
	rtr.HandleFunc("/logout", logout)
	rtr.HandleFunc("/registration_form", registrationForm)
	rtr.HandleFunc("/registration", registration)
	rtr.HandleFunc("/get_user", getUser)
	rtr.HandleFunc("/save_article", saveArticle).Methods("POST")
	rtr.HandleFunc("/post/{id:[0-9]+}", showPost).Methods("GET", "PUT")
	rtr.HandleFunc("/userForm/{id:[0-9]+}", showUserForm).Methods("GET")

	http.Handle("/", rtr)
	//обработчик всех файлов со стилями в папке /static
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	http.ListenAndServe(":8080", nil)
}

func main() {
	customer = User{}
	handleFunc()
}

//todo Почему-то если ввести не верный пароль существующему пользаку - все равно выводится вся информация по нему
//todo при нажатии на User_Id в статьях сделать переход на анкету пользователя
//todo сделать чтобы выводилось по 10 статей на главной
