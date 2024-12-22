package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os" // добавьте этот импорт, если его нет
	"sync"
	"time"

	"github.com/gorilla/mux"

	_ "github.com/go-sql-driver/mysql"
)

type Article struct {
	Id                         uint16
	Title, Anons, Text, UserId string
}

type User struct {
	Id, Birthday, Name, Surname, Sex, City, Hobbies, Email, Password string
	IsAuthorized                                                     bool
}

type DataPage struct {
	Customer User
	Posts    []Article
}

type DataOnePage struct {
	Customer User
	Post     Article
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
	t, err := template.ParseFiles(
		"templates/index.html", "templates/header.html",
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
	t, err := template.ParseFiles(
		"templates/usersForms.html", "templates/header.html",
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

func postCreate(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles(
		"templates/postCreate.html", "templates/header.html", "templates/footer.html")

	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	//fmt.Printf("postCreate --> user IsAuthorized: %v\n", customer.IsAuthorized)
	t.ExecuteTemplate(w, "postCreate", customer)
}

func editPostForm(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id := vars["id"]

    if id == "" {
        http.Error(w, "Некорректный ID", http.StatusBadRequest)
        return
    }

    db := getReplicaDB()
    if db == nil {
        http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
        return
    }

    var article Article
    err := db.QueryRow("SELECT id, title, anons, text FROM nickopolis.articles WHERE id = ?", id).Scan(
        &article.Id, &article.Title, &article.Anons, &article.Text,
    )
    if err != nil {
        log.Printf("Ошибка выборки статьи ID %s: %v", id, err)
        http.Error(w, "Статья не найдена", http.StatusNotFound)
        return
    }

    tmpl, err := template.ParseFiles("templates/editPost.html", "templates/header.html", "templates/footer.html")
    if err != nil {
        log.Printf("Ошибка загрузки шаблона: %v", err)
        http.Error(w, "Ошибка загрузки страницы", http.StatusInternalServerError)
        return
    }

	// fmt.Printf("--> Id: %d, Title: %s, Anons:%s, Text:%s,\n", article.Id, article.Title, article.Anons, article.Text)

	var data = DataOnePage{Customer: customer, Post: article}
    tmpl.ExecuteTemplate(w, "editPost", data)
}

func postUpdate(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id := vars["id"]

    if id == "" {
        http.Error(w, "Некорректный ID", http.StatusBadRequest)
        return
    }

	userId := r.FormValue("userId")
    title := r.FormValue("title")
    anons := r.FormValue("anons")
    text := r.FormValue("text")

    if title == "" || anons == "" || text == "" {
        http.Error(w, "Все поля должны быть заполнены", http.StatusBadRequest)
        return
    }

    db := getMasterDB()
    if db == nil {
        http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
        return
    }

    query := "UPDATE articles SET title = ?, anons = ?, text = ?, user_id = ? WHERE id = ?"
    stmt, err := db.Prepare(query)
    if err != nil {
        log.Printf("Ошибка подготовки SQL-запроса: %v", err)
        http.Error(w, "Ошибка редактирования статьи", http.StatusInternalServerError)
        return
    }
    defer stmt.Close()

    _, err = stmt.Exec(title, anons, text, userId, id)
    if err != nil {
        log.Printf("Ошибка выполнения SQL-запроса: %v", err)
        http.Error(w, "Ошибка редактирования статьи", http.StatusInternalServerError)
        return
    }

    log.Printf("Статья ID %s успешно обновлена пользователем с userId: %s", id, userId)
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

func postDelete(w http.ResponseWriter, r *http.Request) {
	// Get ID from URL
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Missing or invalid ID", http.StatusBadRequest)
		return
	}

	db := getMasterDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	// Prepare and execute the delete statement
	_, err := db.Exec("DELETE FROM nickopolis.articles WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Ошибка удаления статьи", http.StatusInternalServerError)
		return
	}

	log.Printf("Статья c ID: %s удалена!", id)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func showPost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// w.WriteHeader(http.StatusOK)
	// fmt.Fprintf(w, "Id: %v\n", vars["id"])

	t, err := template.ParseFiles(
		"templates/showPost.html", "templates/header.html", "templates/footer.html")

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
	res, err := db.Query("SELECT id, title, anons, text, user_id FROM articles WHERE id = ?", vars["id"])
	if err != nil {
		http.Error(w, "Ошибка выполнения запроса к базе данных", http.StatusInternalServerError)
		return
	}
	defer res.Close()

	var article = Article{}
	for res.Next() {
		var post Article
		err = res.Scan(&post.Id, &post.Title, &post.Anons, &post.Text, &post.UserId)
		if err != nil {
			panic(err)
		}

		article = post
	}

	//fmt.Printf("showPost --> user : %v\n", customer)

	var data = DataOnePage{Customer: customer, Post: article}
	t.ExecuteTemplate(w, "showPost", data)
}

func postFeed(w http.ResponseWriter, r *http.Request) {
	userID := customer.Id
	// log.Printf("Текущий userID: %v", userID)

	// Проверяем, что userID имеет значение
	if userID == "" {
		log.Println("userID не задан. Перенаправление на страницу входа.")
		http.Redirect(w, r, "/login", http.StatusSeeOther) // Перенаправление на /login
		return
	}

    db := getReplicaDB()
    if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
    }

    // SQL-запрос для выборки постов друзей
    query := `
        SELECT a.id, a.title, a.text, a.user_id
        FROM articles a
        JOIN friends f ON f.friend_id = a.user_id
        WHERE f.user_id = ?
        ORDER BY a.id DESC
    `
    res, err := db.Query(query, userID)
    if err != nil {
        log.Printf("Ошибка выполнения SQL-запроса: %v", err)
        http.Error(w, "Ошибка загрузки ленты", http.StatusInternalServerError)
        return
    }
    defer res.Close()


	var posts = []Article{} //Чтоб не дублировались одни и теже посты при обновлении страницы
	for res.Next() {
		var post Article
		err = res.Scan(&post.Id, &post.Title, &post.Text, &post.UserId)
		if err != nil {
			log.Printf("Ошибка обработки данных: %v", err)
            http.Error(w, "Ошибка загрузки ленты", http.StatusInternalServerError)
            return
		}
		// log.Printf("Посты друзей: %+v", post)
		posts = append(posts, post)
	}

	var data = DataPage{Customer: customer, Posts: posts}

    // Отображение шаблона
    tmpl, err := template.ParseFiles("templates/postFeed.html", "templates/header.html", "templates/footer.html")
    if err != nil {
        log.Printf("Ошибка загрузки шаблона: %v", err)
        http.Error(w, "Ошибка отображения страницы", http.StatusInternalServerError)
        return
    }

    tmpl.ExecuteTemplate(w, "postFeed", data)
}

func showUserForm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r) //Вытаскиваем все параметры из запроса
	// w.WriteHeader(http.StatusOK)
	// fmt.Fprintf(w, "Id: %v\n", vars["id"])

	t, err := template.ParseFiles(
		"templates/userForm.html", "templates/header.html", "templates/footer.html")

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
	t.ExecuteTemplate(w, "userForm", userInfo)
}

func saveArticle(w http.ResponseWriter, r *http.Request) {
	// Получение данных из формы
	title := r.FormValue("title")
	anons := r.FormValue("anons")
	text := r.FormValue("text")
	userId := customer.Id

	// Проверка заполненности данных
	if title == "" || anons == "" || text == "" {
		http.Error(w, "Не все данные заполнены", http.StatusBadRequest)
		return
	}

	// Получение подключения к базе данных
	db := getMasterDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	// Подготовленный запрос для безопасного добавления данных
	query := "INSERT INTO articles (title, anons, text, user_id) VALUES (?, ?, ?, ?)"
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Printf("Ошибка подготовки SQL-запроса: %v", err)
		http.Error(w, "Ошибка сохранения статьи", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	// Выполнение запроса
	_, err = stmt.Exec(title, anons, text, userId)
	if err != nil {
		log.Printf("Ошибка выполнения SQL-запроса: %v", err)
		http.Error(w, "Ошибка сохранения статьи", http.StatusInternalServerError)
		return
	}

	// Перенаправление на главную страницу после успешного сохранения
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func login(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles(
		"templates/login.html", "templates/header.html", "templates/footer.html")

	if err != nil {
		log.Printf("Ошибка загрузки шаблона: %v", err)
		http.Error(w, "Ошибка загрузки шаблона", http.StatusInternalServerError)
		return
	}

	// Выполняем шаблон, если он успешно загружен
	if err := t.ExecuteTemplate(w, "login", customer); err != nil {
		log.Printf("Ошибка выполнения шаблона: %v", err)
		http.Error(w, "Ошибка выполнения шаблона", http.StatusInternalServerError)
		return
	}
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
		t, err := template.ParseFiles(
			"templates/registrationForm.html", "templates/header.html", "templates/footer.html")

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
	t, err := template.ParseFiles(
		"templates/registrationForm.html", "templates/header.html", "templates/footer.html")

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
			"id":       fmt.Sprint(id),
			"name":     userName,
			"surname":  userSurname,
			"birthday": birthday,
			"sex":      sex,
			"city":     city,
			"hobbies":  hobbies,
			"email":    email,
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
	t, err := template.ParseFiles(
		"templates/users.html", "templates/header.html", "templates/footer.html")

	if err != nil {
		fmt.Fprintln(w, err.Error())
	}

	t.ExecuteTemplate(w, "users", customer)
}

func friends(w http.ResponseWriter, r *http.Request) {
	userID := customer.Id

	// Проверяем, что userID имеет значение
	if userID == "" { // Если строка
		log.Println("userID не задан. Перенаправление на страницу входа.")
		http.Redirect(w, r, "/login", http.StatusSeeOther) // Перенаправление на /login
		return
	}

	db := getReplicaDB()
	if db == nil {
		http.Error(w, "База данных временно недоступна", http.StatusInternalServerError)
		return
	}

	query := `
        SELECT id, name, surname, sex, city, hobbies, email, birthday
        FROM nickopolis.users
        WHERE id IN (SELECT friend_id FROM nickopolis.friends WHERE user_id = ?)
    `
	rows, err := db.Query(query, userID)
	if err != nil {
		log.Printf("Ошибка выполнения SQL-запроса: %v", err)
		http.Error(w, "Ошибка выполнения запроса к базе данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var friends []User
	for rows.Next() {
		var friend User
		if err := rows.Scan(&friend.Id, &friend.Name, &friend.Surname, &friend.Sex, &friend.City, &friend.Hobbies, &friend.Email, &friend.Birthday); err != nil {
			log.Printf("Ошибка чтения результата: %v", err)
			http.Error(w, "Ошибка обработки данных", http.StatusInternalServerError)
			return
		}
		friends = append(friends, friend)
	}

	data := DataFormPage{
		Customer: customer,
		Users:    friends,
	}

	t, err := template.ParseFiles(
		"templates/friends.html", "templates/header.html", "templates/footer.html")

	if err != nil {
		log.Printf("Ошибка загрузки шаблона: %v", err)
		http.Error(w, "Ошибка загрузки шаблона", http.StatusInternalServerError)
		return
	}

	if err := t.ExecuteTemplate(w, "friends", data); err != nil {
		log.Printf("Ошибка выполнения шаблона: %v", err)
		http.Error(w, "Ошибка выполнения шаблона", http.StatusInternalServerError)
	}
}

func handleFriendAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Проверка авторизации
	userID := customer.Id
	if userID == "" {
		log.Println("userID не задан. Перенаправление на страницу входа.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	action := r.FormValue("action")
	friendID := r.FormValue("friend_id")

	if friendID == "" {
		log.Println("friend_id не указан.")
		http.Error(w, "friend_id обязателен", http.StatusBadRequest)
		return
	}

	db := getMasterDB() // Получаем соединение с базой данных (Master)
	if db == nil {
		log.Println("База данных недоступна")
		http.Error(w, "Ошибка соединения с базой данных", http.StatusInternalServerError)
		return
	}

	switch action {
	case "add":
		_, err := db.Exec("INSERT INTO nickopolis.friends (user_id, friend_id) VALUES (?, ?)", userID, friendID)
		if err != nil {
			log.Printf("Ошибка добавления друга: %v", err)
			http.Error(w, "Ошибка добавления друга", http.StatusInternalServerError)
			return
		}
		log.Printf("Друг с ID %s добавлен для пользователя %s", friendID, userID)

	case "delete":
		_, err := db.Exec("DELETE FROM nickopolis.friends WHERE user_id = ? AND friend_id = ?", userID, friendID)
		if err != nil {
			log.Printf("Ошибка удаления друга: %v", err)
			http.Error(w, "Ошибка удаления друга", http.StatusInternalServerError)
			return
		}
		log.Printf("Друг с ID %s удалён для пользователя %s", friendID, userID)

	default:
		http.Error(w, "Неверное действие", http.StatusBadRequest)
		return
	}

	// Перенаправление обратно на страницу /friends
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

func handleFunc() {
	rtr := mux.NewRouter()
	fmt.Println("Сервер запущен на http://localhost:80")
	// Обработчик для favicon.ico
	rtr.HandleFunc("/", index).Methods("GET") //указав Methods("GET") мы защищаем наш сервер от ввода запросов c другими методами
	rtr.HandleFunc("/usersForms", usersForms).Methods("GET")
	rtr.HandleFunc("/login", login)
	rtr.HandleFunc("/logout", logout)
	rtr.HandleFunc("/registration_form", registrationForm)
	rtr.HandleFunc("/registration", registration)
	rtr.HandleFunc("/get_user", getUser)
	rtr.HandleFunc("/post/{id:[0-9]+}", showPost).Methods("GET", "PUT")
	rtr.HandleFunc("/post/create", postCreate).Methods("GET")
	rtr.HandleFunc("/post/edit/{id:[0-9]+}", editPostForm).Methods("GET") 	// Редактирование статьи
	rtr.HandleFunc("/post/update/{id:[0-9]+}", postUpdate).Methods("POST")  // Обновление статьи
	rtr.HandleFunc("/post/delete/{id:[0-9]+}", postDelete).Methods("POST")
	rtr.HandleFunc("/post/feed", postFeed).Methods("GET")				// Просмотр статей друзей
	rtr.HandleFunc("/save_article", saveArticle).Methods("POST")
	rtr.HandleFunc("/userForm/{id:[0-9]+}", showUserForm).Methods("GET")
	rtr.HandleFunc("/users", users).Methods("GET")
	rtr.HandleFunc("/users/search", searchUserHandler).Methods("GET")
	rtr.HandleFunc("/friends", friends).Methods("GET")
	rtr.HandleFunc("/friends/action", handleFriendAction).Methods("POST")

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
