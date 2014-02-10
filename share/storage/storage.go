// Package storage provides a service for storing files with access constraints.
package storage

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Constraints for the use of an uploaded file.
type Constraints struct {
	sync.RWMutex
	// Expire is the date after which the file should be inaccessible. Use zero
	// time to make the file never expire.
	Expire time.Time
	// Downloads is the number of times the file can be served. If 0, unlimited.
	Downloads int
	// Public specifies should the file be in a public list.
	Public bool
	// Delete specifies whether to delete the file from a server when it becomes unaccessible.
	Delete bool
}

func (c *Constraints) expired() bool {
	return !c.Expire.IsZero() && c.Expire.Before(time.Now())
}

// Storage is an interface for file storage.
type Storage interface {
	// Add stores a file with given constraints. If a file with a given name was
	// already stored, the old one is replaced.
	Add(file io.Reader, name string, c Constraints) error
	// List returns a list of public files which can be Served.
	List() []string
	// Remove removes the file from the storage.
	Remove(name string)
	// Serve is used to reply to HTTP request with the contents of stored file.
	// If a file is not stored or unaccessible due to Constraints, Serve returns
	// an error.
	Serve(w http.ResponseWriter, r *http.Request, name string) error
}

// NewDirectory returns a Storage which uses file system directory for storing
// its files. If name is empty, creates a temporary directory.
func NewDirectory(name string) (Storage, error) {
	if name == "" {
		var err error
		name, err = ioutil.TempDir("", "go-share")
		if err != nil {
			return nil, fmt.Errorf("error creating a temporary directory: %v", err)
		}
	} else {
		dir, err := os.Open(name)
		if err != nil {
			return nil, fmt.Errorf("error opening directory %s: %v", name, err)
		}
		defer dir.Close()
		if fi, err := dir.Stat(); err != nil {
			return nil, fmt.Errorf("could not determine directory %s file type: %v", name, err)
		} else if fi.IsDir() == false {
			return nil, fmt.Errorf("%v is not a directory", name)
		}
	}
	return &directory{
		name:  name,
		files: make(map[string]*Constraints),
	}, nil
}

// list stores a file list.
type list struct {
	sync.RWMutex
	// dirty is set to true only when storage is Locked,
	// and set to false when list is Locked and storage is RLocked.
	dirty bool
	slice []string
}

type directory struct {
	sync.RWMutex
	name  string
	files map[string]*Constraints
	list
}

func (d *directory) Add(file io.Reader, name string, c Constraints) error {
	if path.Base(name) != name {
		return fmt.Errorf("invalid file name %s", name)
	}
	flags := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	d.Lock()
	defer d.Unlock()
	if _, ok := d.files[name]; ok {
		log.Println("Warning: will overwrite existing file.")
	} else {
		flags = flags | os.O_EXCL
	}
	f, err := os.OpenFile(path.Join(d.name, name), flags, 0666)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, file)
	if err != nil {
		return err
	}
	d.files[name] = &c
	if c.Public {
		d.list.dirty = true
	}
	if !c.Expire.IsZero() {
		time.AfterFunc(c.Expire.Sub(time.Now()), func() { d.remove(name) })
	}
	return nil
}

// remove handles removal after expiration or when the download limit is reached.
func (d *directory) remove(name string) {
	d.Lock()
	defer d.Unlock()
	constraints, ok := d.files[name]
	if !ok {
		log.Println("File", name, "already removed.")
		return
	}
	constraints.Lock() // wait until the file is served
	defer constraints.Unlock()
	if constraints.Downloads != -1 && !constraints.expired() {
		log.Println("File", name, "will not be removed.")
		return
	}
	if constraints.Delete {
		if err := os.Remove(path.Join(d.name, name)); err != nil {
			log.Printf("Could not remove file %s: %s\n", name, err)
		}
	}
	if constraints.Public {
		d.list.dirty = true
	}
	delete(d.files, name)
}

func (d *directory) Remove(name string) {
	d.Lock()
	defer d.Unlock()
	if c := d.files[name]; c != nil && c.Public {
		d.list.dirty = true
	}
	delete(d.files, name)
}

func (d *directory) List() []string {
	d.list.RLock()
	for d.list.dirty {
		d.list.RUnlock()
		d.list.Lock()
		if d.list.dirty {
			d.RLock()
			if l := len(d.files); l > cap(d.list.slice) {
				d.list.slice = make([]string, 0, l)
			} else {
				d.list.slice = d.list.slice[:0]
			}
			for name, c := range d.files {
				if c.Public {
					d.list.slice = append(d.list.slice, name)
				}
			}
			sort.Strings(d.list.slice)
			d.list.dirty = false
			d.RUnlock() // Must go after dirty = false, see struct comment.
		}
		d.list.Unlock()
		d.list.RLock()
	}
	l := make([]string, len(d.list.slice))
	copy(l, d.list.slice)
	d.list.RUnlock()
	return l
}

func (d *directory) Serve(w http.ResponseWriter, r *http.Request, name string) error {
	if strings.Contains(name, "/") || name == "" {
		return errors.New("Invalid file name")
	}
	d.RLock()
	constraints, ok := d.files[name]
	d.RUnlock()
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
			defer d.remove(name)
		case -1:
			constraints.Unlock()
			return errors.New("Download limit exceeded")
		default:
			constraints.Downloads--
		}
		http.ServeFile(w, r, path.Join(d.name, name))
		constraints.Unlock()
		return nil
	}
	http.ServeFile(w, r, path.Join(d.name, name))
	constraints.RUnlock()
	return nil
}

func (d *directory) String() string {
	return fmt.Sprint("Storage in directory ", d.name)
}
