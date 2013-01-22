/*
	File sharing server.
*/
package share

import (
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func newError(msg string, cause error) error {
	if cause == nil {
		return errors.New(msg)
	}
	return errors.New(msg + ": " + cause.Error())
}

// Constraints for the use of an uploaded file.
// Expire – the date after which the file should be inaccessible. Use zero time
// to make the file never expire.
// Downloads – the number of times the file can be downloaded. If 0, unlimited.
// Public – should the file be in a public list.
// Delete – whether to delete the file from a server when it becomes
// unaccessible.
type Constraints struct {
	sync.RWMutex
	Expire         time.Time
	Downloads      int
	Public, Delete bool
}

func (c *Constraints) expired() bool {
	return !c.Expire.IsZero() && c.Expire.Before(time.Now())
}

// File sharing server.
type Server struct {
	sync.RWMutex
	directory, password string
	data                map[string]*Constraints
	list                list
}

type list struct {
	sync.RWMutex
	dirty bool // is set to true only when Server is Lock()ed,
	// and set to false when list is Lock()ed and Server RLock()ed
	slice []string
}

// Creates a new server which stores uploaded files in a given directory.
// Uploads are secured using a password.
func New(directory, password string) (*Server, error) {
	dir, err := os.Open(directory)
	if err != nil {
		return nil, newError("Error opening directory", err)
	}
	defer dir.Close()
	if fi, err := dir.Stat(); err != nil {
		return nil, newError("Could not determine directory file type", err)
	} else if fi.IsDir() == false {
		return nil, newError(directory+" is not a directory", nil)
	}
	return &Server{directory: directory, password: password,
		data: make(map[string]*Constraints),
		list: list{slice: make([]string, 0)}}, nil
}

// Starts the server which listens on a given port.
func (s *Server) Start(port int) error {
	return http.ListenAndServe(":"+strconv.Itoa(port), s)
}

// Handles HTTP requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		if r.FormValue("upload") != "" {
			executeTemplate(tmplMessage, &w, s.handleAdd(r))
		} else {
			s.list.RLock()
			if s.list.dirty {
				s.list.RUnlock()
				s.list.Lock()
				if s.list.dirty {
					s.RLock()
					s.list.slice = s.list.slice[:0]
					for name, c := range s.data {
						if c.Public {
							s.list.slice = append(s.list.slice, name)
						}
					}
					sort.Strings(s.list.slice)
					s.list.dirty = false
					s.RUnlock()
				}
				s.list.Unlock()
				s.list.RLock()
			}
			executeTemplate(tmplList, &w, s.list.slice)
			s.list.RUnlock()
		}
	} else if err := s.serve(&w, r, r.URL.Path[1:]); err != nil {
		log.Printf("Could not serve %s: %s\n", r.URL.Path[1:], err)
		http.NotFound(w, r)
	}
}

func executeTemplate(t *template.Template, w *http.ResponseWriter, data interface{}) {
	if err := t.Execute(*w, data); err != nil {
		log.Printf("Error executing template %s: %s", t.Name(), err)
	}
}

func (s *Server) handleAdd(r *http.Request) string {
	if r.FormValue("password") != s.password {
		log.Println("Wrong password:", r.FormValue("password"))
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
	c := Constraints{Public: r.FormValue("public") != "",
		Delete: r.FormValue("delete") != ""}
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
	err = s.add(file, name, c)
	if err != nil {
		log.Println("Could not add the file:", err)
		return "Error."
	}
	return "Direct link: http://" + r.Host + "/" + url.QueryEscape(name)
}

// Serves the file if it is accessible.
func (s *Server) serve(w *http.ResponseWriter, r *http.Request, file string) error {
	if strings.Contains("/", file) || file == "" {
		return errors.New("Invalid file name")
	}
	s.RLock()
	constraints, ok := s.data[file]
	s.RUnlock()
	if !ok {
		return errors.New("File not stored")
	}
	constraints.RLock()
	if constraints.expired() {
		return errors.New("File recently expired")
	}
	if constraints.Downloads > 0 { // have to modify the value
		constraints.RUnlock()
		constraints.Lock()
		switch constraints.Downloads {
		case 1:
			constraints.Downloads = -1
			defer s.remove(file)
		case -1:
			constraints.Unlock()
			return errors.New("Download limit exceeded")
		default:
			constraints.Downloads--
		}
		http.ServeFile(*w, r, s.directory+"/"+file)
		constraints.Unlock()
		return nil
	}
	http.ServeFile(*w, r, s.directory+"/"+file)
	constraints.RUnlock()
	return nil
}

func (s *Server) remove(file string) {
	s.Lock()
	defer s.Unlock()
	constraints, ok := s.data[file]
	if !ok {
		log.Println("File", file, "already removed.")
		return
	}
	constraints.Lock() // wait until the file is served
	defer constraints.Unlock()
	if constraints.Downloads != -1 && !constraints.expired() {
		log.Println("File", file, "will not be removed.")
		return
	}
	if constraints.Delete {
		if err := os.Remove(s.directory + "/" + file); err != nil {
			log.Printf("Could not remove file %s: %s\n", file, err)
		}
	}
	delete(s.data, file)
	s.list.dirty = true
}

func (s *Server) add(file io.Reader, name string, c Constraints) error {
	if strings.Contains("/", name) {
		return errors.New("Invalid file name")
	}
	flags := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	s.Lock()
	defer s.Unlock()
	if _, ok := s.data[name]; ok {
		log.Println("Warning: will overwrite existing file.")
	} else {
		flags = flags | os.O_EXCL
	}
	f, err := os.OpenFile(s.directory+"/"+name, flags, 0666)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, file)
	if err != nil {
		return err
	}
	s.data[name] = &c
	s.list.dirty = true
	if !c.Expire.IsZero() {
		time.AfterFunc(c.Expire.Sub(time.Now()), func() { s.remove(name) })
	}
	return nil
}
