package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"database/sql"

	"github.com/asaskevich/govalidator"
	_ "github.com/lib/pq"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

func GenerateCode() string {
	b := make([]byte, 6)
	for i, cache, remain := 5, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

//go:embed index.html
var tmplSource string
var tmpl *template.Template

func main() {
	log.Println("Booting up...")

	var err error
	tmpl, err = template.New("index.html").Parse(tmplSource)
	if err != nil {
		panic(err)
	}

	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URI"))
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
		code       text not null,
		url        text not null,
		created_at timestamp with time zone not null,
		author     text not null,
		hits       bigint not null
	);`)
	if err != nil {
		panic(err)
	}

	bind := os.Getenv("SHORTEN_BIND")
	if bind == "" {
		bind = "127.0.0.1:4488"
	}
	panic(http.ListenAndServe(bind, &Handler{db: db}))
}

type Handler struct {
	db *sql.DB
}

func (h *Handler) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		if req.Method == "POST" {
			h.CreateHandler(wr, req)
		} else {
			h.HomepageHandler(wr, req)
		}
		return
	}

	h.RedirectHandler(wr, req)
}

func (h *Handler) HomepageHandler(wr http.ResponseWriter, req *http.Request) {
	url := req.URL.Query().Get("url")
	if url != "" {
		code, err := h.GetCode(url, req.RemoteAddr)
		if err != nil {
			log.Println(err)
			wr.WriteHeader(http.StatusInternalServerError)
			wr.Write([]byte(err.Error()))
		}
		wr.Write([]byte("https://" + os.Getenv("SHORTEN_HOST") + "/" + code))
	}

	Render(wr, map[string]string{})
}

func (h *Handler) CreateHandler(wr http.ResponseWriter, req *http.Request) {
	ip := req.RemoteAddr
	if strings.HasPrefix(ip, "127.0.0.1") {
		proxyIp := strings.Split(req.Header.Get("X-Forwarded-For"), ",")[0]
		if proxyIp != "" {
			ip = proxyIp
		}
	}

	req.ParseForm()
	code, err := h.GetCode(req.Form.Get("url"), ip)
	if err != nil {
		Render(wr, map[string]string{"error": err.Error()})
	} else {
		Render(wr, map[string]string{"code": code})
	}
}

func (h *Handler) RedirectHandler(wr http.ResponseWriter, req *http.Request) {
	code := req.URL.Path[1:]
	var url string
	var hits uint64
	if err := h.db.QueryRow(`SELECT url, hits FROM urls WHERE code = $1`, code).Scan(&url, &hits); err != nil {
		if err != sql.ErrNoRows {
			log.Println("hits query error:", err)
		}
		wr.Header().Set("Location", "/")
		wr.WriteHeader(http.StatusFound)
		return
	}

	go func() {
		_, _ = h.db.Exec(`UPDATE urls SET hits = $1 WHERE code = $2`, hits+1, code)
	}()

	wr.Header().Set("Location", url)
	wr.WriteHeader(http.StatusMovedPermanently)
}

func (h *Handler) CodeExists(code string) bool {
	var exists bool
	err := h.db.QueryRow(`SELECT 1 FROM urls WHERE code = $1 LIMIT 1`, code).Scan(&exists)
	if err == nil {
		return true
	}
	if err != sql.ErrNoRows {
		log.Println("check code error:", err)
	}

	return false
}

func (h *Handler) GetCode(url string, ip string) (string, error) {
	if !strings.HasPrefix(url, "http") || !govalidator.IsURL(url) {
		return "", fmt.Errorf("invalid URL")
	}

	var code string
	err := h.db.QueryRow(`SELECT code FROM urls WHERE url = $1 LIMIT 1`, url).Scan(&code)
	if err == nil {
		return code, nil
	}
	if err != sql.ErrNoRows {
		log.Println("sql error:", err)
		return "", fmt.Errorf("query: %w", err)
	}

	code = GenerateCode()
	for h.CodeExists(code) {
		code = GenerateCode()
	}

	_, err = h.db.Exec(
		`INSERT INTO urls (code, url, created_at, author, hits) VALUES ($1, $2, $3, $4, $5)`,
		code, url, time.Now(), ip, 0,
	)
	if err != nil {
		return "", err
	}
	return code, nil
}

func Render(wr http.ResponseWriter, data map[string]string) {
	wr.Header().Set("Content-Type", "text/html")
	data["host"] = os.Getenv("SHORTEN_HOST")
	data["mail"] = os.Getenv("SHORTEN_MAIL")
	err := tmpl.Execute(wr, data)
	if err != nil {
		log.Println("error writing template:", err)
	}
}
