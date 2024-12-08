package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"time"
	"html/template"
	"net/http"
	"encoding/json"
	"log"
	"os" // добавьте этот импорт, если его нет

	"github.com/gorilla/mux"

	_ "github.com/go-sql-driver/mysql"
)

type Article struct {
	Id                         uint16
	Title, Anons, Text, UserId string
}

type User struct {
	Id, Birthday, Name, Surname, Sex, City, Hobbies, Email, Password string
	IsAuthorized                                                	 bool
}

type DataPage struct {
	Customer User
	Posts    []Article
}

type DataFormPage struct {
	Customer User
	Users    []User
}

// Глобальные переменные для БД
var masterDB *sql.DB
var replicaDBs []*sql.DB
var customer = User{}
var mutex sync.Mutex

var article = Article{}
var userInfo = User{}

func waitForDB(dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("время ожидания подключения к БД истекло")
		}

		db, err := sql.Open("mysql", dsn)
		if err == nil && db.Ping() == nil {
			log.Println("База данных готова")
			return nil
		}

		log.Println("Ожидание готовности базы данных...")
		time.Sleep(2 * time.Second)
	}
}

// Инициализация соединений с базами данных
func initDBConnections() {
	var err error

	// Соединение с Master
	masterDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	masterDB, err = sql.Open("mysql", masterDSN)
	if err != nil {
		log.Fatalf("Ошибка подключения к Master БД: %v", err)
	}

	// Хосты реплик
	replicaHosts := []string{"slave1", "slave2"} // Указываем хосты реплик из docker-compose.yml

	// Подключение к Replica
	for _, host := range replicaHosts {
		replicaDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			host,
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		)
		replicaDB, err := sql.Open("mysql", replicaDSN)
		if err != nil {
			log.Printf("Ошибка подключения к Replica (%s): %v", host, err)
			continue
		}
		// Проверка соединения
		if err = replicaDB.Ping(); err != nil {
			log.Printf("Ошибка соединения с Replica (%s): %v", host, err)
			continue
		}
		replicaDBs = append(replicaDBs, replicaDB)
	}

	if len(replicaDBs) == 0 {
		log.Fatal("Нет доступных реплик для чтения")
	}

	log.Println("Соединения с базами данных успешно инициализированы")
}

// Получить соединение с Master для записи
func getMasterDB() *sql.DB {
	return masterDB
}

// Получить соединение с Replica для чтения
func getReplicaDB() *sql.DB {
    mutex.Lock()
    defer mutex.Unlock()

    if len(replicaDBs) == 0 {
        log.Println("Нет доступных реплик для чтения")
        return nil
    }

    rand.Seed(time.Now().UnixNano())
    db := replicaDBs[rand.Intn(len(replicaDBs))]

    // Проверяем соединение
    if err := db.Ping(); err != nil {
        log.Printf("Реплика недоступна: %v", err)
        return nil
    }

    return db
}


func index(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/index.html", "templates/header.html",
		"templates/footer.html", "templates/login.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
		return
	}

	// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	//Выборка данных
	res, err := db.Query("select * from articles;")
	if err != nil {
		panic(err)
	}

	defer res.Close()

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
		return
	}

	// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	//Выборка данных
	res, err := db.Query("select * from users ORDER BY RAND() LIMIT 10;")
	if err != nil {
		panic(err)
	}

	var usersForms = []User{} //Чтоб не дублировались одни и теже посты при обновлении страницы
	for res.Next() {
		var user User
		err = res.Scan(&user.Id, &user.Name, &user.Surname, &user.Sex, &user.City, &user.Hobbies, &user.Email, &user.Password, &user.Birthday)
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

	// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

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

	// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	//Выборка данных
	res, err := db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = '%s';", vars["id"]))
	if err != nil {
		panic(err)
	}

	userInfo = User{}
	for res.Next() {
		var user User
		err = res.Scan(&user.Id, &user.Name, &user.Surname, &user.Sex, &user.City, &user.Hobbies, &user.Email, &user.Password, &user.Birthday)
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
		// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
		db := getMasterDB()
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

	// fmt.Printf("login --> user : %v\n", customer)
	t.ExecuteTemplate(w, "login", customer)
}

func logout(w http.ResponseWriter, r *http.Request) {
	customer.IsAuthorized = false
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	emailForm := r.FormValue("email")
	passwordForm := r.FormValue("password")
	// fmt.Printf("\nemail : %s\n", emailForm)

	// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	//Проверка наличие зарегистрированного пользователя
	res, err := db.Query(fmt.Sprintf("SELECT * FROM users WHERE email = '%s';", emailForm))
	if err != nil {
		panic(err)
	}

	//customer = User{}
	for res.Next() {
		var user User
		err = res.Scan(&user.Id, &user.Name, &user.Surname, &user.Sex, &user.City, &user.Hobbies, &user.Email, &user.Password, &user.Birthday)
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
	userBirthday := r.FormValue("birthday")
	userSurname := r.FormValue("surname")
	userSex := r.FormValue("sex")
	userCity := r.FormValue("city")
	userHobbies := r.FormValue("hobbies")
	userEmail := r.FormValue("email")
	userPassword := r.FormValue("password")

	if userName == "" || userBirthday == "" || userSurname == "" || userSex == "" || userCity == "" || userHobbies == "" || userEmail == "" || userPassword == "" {
		fmt.Fprintf(w, "Не все данные заполненны")
	} else {
		// db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/nickopolis")
		db := getMasterDB()
		defer db.Close()

		//Установка данных
		insert, err := db.Query(fmt.Sprintf("INSERT INTO users (`name`, `birthday`, `surname`, `sex`, `city`, `hobbies`, `email`, `password`) VALUES('%s', '%s', '%s', '%s','%s', '%s', '%s', '%s');",
			userName, userBirthday, userSurname, userSex, userCity, userHobbies, userEmail, userPassword))
		if err != nil {
			panic(err)
		}

		defer insert.Close()
	}

	//fmt.Printf("registration --> user : %v\n", customer)
	http.Redirect(w, r, "#", http.StatusSeeOther)
}

func searchUserHandler(w http.ResponseWriter, r *http.Request) {
    // Извлечение параметров запроса
    name := r.URL.Query().Get("name")
    surname := r.URL.Query().Get("surname")

    if name == "" && surname == "" {
        http.Error(w, "Необходимо указать хотя бы одно из полей: name или surname", http.StatusBadRequest)
        return
    }

    // Формирование запроса с учетом переданных параметров
    query := "SELECT id, name, surname, birthday, sex, city, hobbies, email FROM users WHERE 1=1"
    var args []interface{}

    if name != "" {
        query += " AND name LIKE ?"
        args = append(args, name+"%")
    }
    if surname != "" {
        query += " AND surname LIKE ?"
        args = append(args, surname+"%")
    }

	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

    rows, err := db.Query(query, args...)
    if err != nil {
        http.Error(w, "Ошибка выполнения запроса к базе данных", http.StatusInternalServerError)
		log.Println(err)
        return
    }
    defer rows.Close()

    // Чтение результатов и формирование ответа
    var results []map[string]string
    for rows.Next() {
        var id int
        var userName, userSurname, birthday, sex, city, hobbies, email string
        if err := rows.Scan(&id, &userName, &userSurname, &birthday, &sex, &city, &hobbies, &email); err != nil {
            http.Error(w, "Ошибка чтения результата", http.StatusInternalServerError)
            log.Println(err)
            return
        }
        results = append(results, map[string]string{
            "id":      fmt.Sprint(id),
            "name":    userName,
            "surname": userSurname,
			"birthday": birthday,
			"sex": sex,
			"city": city,
			"hobbies": hobbies,
			"email": email,
        })
    }

	// Установка заголовков для ответа в формате JSON
	w.Header().Set("Content-Type", "application/json")

    if len(results) == 0 {
        fmt.Fprintln(w, "Пользователи не найдены")
        return
    }

	// Сериализация данных с отступами для json
	jsonData, err := json.MarshalIndent(results, "", "    ")
	if err != nil {
		http.Error(w, "Ошибка при создании JSON", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	w.Write(jsonData)
}

func users(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/users.html", "templates/header.html", "templates/footer.html")
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	t.ExecuteTemplate(w, "users", customer)
}

func handleFunc() {
	rtr := mux.NewRouter()
	fmt.Println("Сервер запущен на http://localhost:80")
	// Обработчик для favicon.ico
	rtr.HandleFunc("/", index).Methods("GET") //указав Methods("GET") мы защищаем наш сервер от ввода запросов c другими методами
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
	rtr.HandleFunc("/users", users).Methods("GET")
	rtr.HandleFunc("/users/search", searchUserHandler).Methods("GET")

	http.Handle("/", rtr)
	// Обработчик для favicon.ico
	http.Handle("/favicon.ico", http.FileServer(http.Dir("./static")))
	// Обработчик для папки css
    http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("./css"))))
	//обработчик всех файлов со стилями в папке /static
	// http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	http.ListenAndServe(":80", nil)
}

func main() {
    customer = User{}
    initDBConnections()
	dsn := "root:root@tcp(mysql-master:3306)/nickopolis"
	if err := waitForDB(dsn, 60*time.Second); err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	log.Println("Базы данных готовы, запускаю приложение...")

    handleFunc()
}

//todo при нажатии на User_Id в статьях сделать переход на анкету пользователя
//todo сделать чтобы выводилось по 10 статей на главной
