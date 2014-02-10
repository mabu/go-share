// Package share contains a file sharing server.
package share

import (
	"bytes"
	"crypto/sha1"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mabu/go-share/share/storage"
)

// Server is a file sharing server.
type Server struct {
	passwordHash []byte
	storage      storage.Storage
}

// New creates a new server which stores uploaded files in s.
// Uploads are secured using a password.
func New(s storage.Storage, password string) *Server {
	return &Server{
		passwordHash: hash(password),
		storage:      s,
	}
}

// Start starts the server which listens on a given port.
func (s *Server) Start(port int) error {
	return http.ListenAndServe(":"+strconv.Itoa(port), s)
}

// ServerHTTP handles HTTP requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		if r.FormValue("upload") != "" {
			executeTemplate(tmplMessage, w, s.handleAdd(r))
		} else {
			executeTemplate(tmplList, w, s.storage.List())
		}
	} else {
		name := r.URL.Path[1:]
		if err := s.storage.Serve(w, r, name); err != nil {
			log.Printf("Could not serve %s: %s\n", name, err)
			http.NotFound(w, r)
		}
	}
}

func executeTemplate(t *template.Template, w http.ResponseWriter, data interface{}) {
	if err := t.Execute(w, data); err != nil {
		log.Printf("Error executing template %s: %s", t.Name(), err)
	}
}

func (s *Server) handleAdd(r *http.Request) string {
	if !bytes.Equal(hash(r.FormValue("password")), s.passwordHash) {
		log.Println("Wrong password.")
		return "Wrong password."
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println("Error parsing the file:", err)
		return "Error."
	}
	name := r.FormValue("name")
	if name == "" {
		name = header.Filename
	}
	if name == "" {
		log.Println("Error: no file name.")
		return "Error: no file name."
	}
	c := storage.Constraints{
		Public: r.FormValue("public") != "",
		Delete: r.FormValue("delete") != "",
	}
	if t := r.FormValue("expire"); t != "" {
		utc, err := time.Parse("2006-01-02 15:04:05", t)
		if err != nil {
			log.Println("Error parsing expire:", err)
			return "Error: invalid expiration date."
		}
		y, m, d := utc.Date()
		H, M, S := utc.Clock()
		c.Expire = time.Date(y, m, d, H, M, S, utc.Nanosecond(), time.Local)
	}
	if d := r.FormValue("downloads"); d != "" {
		if c.Downloads, err = strconv.Atoi(d); err != nil {
			log.Println("Error parsing downloads:", err)
			return "Error: invalid number of downloads."
		} else if c.Downloads < 1 {
			log.Println("Invalid number of downloads:", c.Downloads)
			return "Error: number of downloads should be positive."
		}
	}
	err = s.storage.Add(file, name, c)
	if err != nil {
		log.Println("Could not add the file:", err)
		return "Error."
	}
	return "Direct link: http://" + r.Host + "/" + url.QueryEscape(name)
}

func hash(password string) []byte {
	h := sha1.New()
	h.Write([]byte(password))
	return h.Sum(nil)
}
