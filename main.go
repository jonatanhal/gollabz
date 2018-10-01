// General TODO's
// - split up source-files, ie. project.go, state.go, etc.
// - 

package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"

	"html/template"
	"net/http"
	"path/filepath"
	"io/ioutil"
)

const csp = "default-src 'none'; "+
	"script-src 'none'; "+
	"connect-src 'none'; "+
	"style-src 'none'; " +
	"img-src 'none'; " +
	"media-src 'none';"

type Project struct {
	Name string
	TODOs map[string]bool
	Files []*File
}

func (p *Project) UpdateFilelist() error {
	return filepath.Walk(fmt.Sprintf("%s/%s/%s",pwd,"projs",p.Name), func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !f.IsDir() {
			if !needleInHaystack(f.Name(),p.Files) {
				log.Printf("[Project{%s}.UpdateFilelist] adding file %s\n", p.Name, f.Name())
				ff := new(File)
				ff.Name = f.Name()
				ff.Project = p.Name
				b, err := ioutil.ReadFile(fmt.Sprintf("%s/%s/%s/%s",pwd,"projs",p.Name,ff.Name))
				if err != nil {
					ff.Contents = fmt.Sprintf("%s",b)
				}
				p.Files = append(p.Files,ff)
			}
		}
		return nil
			
	})
}

type State struct {
	Projects []*Project
}

func (s *State) GetProjectFile(project, file string) (fh *File, err error) {
	for _, p := range s.Projects {
		if p.Name == project {
			for _,v := range p.Files {
				if v.Name == file {
					return v, nil
				}
			}
		}
	}
	return &File{}, fmt.Errorf("Project %s file %s: Not found", project, file)
}

func (s *State) GetProject(name string) (p *Project, err error) {
	for _, p = range s.Projects {
		if p.Name == name {
			return
		}
	}
	return &Project{}, fmt.Errorf("No such project: '%s'")
}

type File struct {
	sync.RWMutex
	Name string
	Project string
	Contents string
}

func (f *File) Save() error {
	//TODO
	panic("Not implemented")
}

var (
	tmplFile *template.Template
	tmplIndex *template.Template
	tmplProject *template.Template
	state *State
	pwd string

)
// string-based `in-slice` checker
func needleInHaystack(needle string, haystack []*File) bool {
	for _, n := range haystack {
		if n.Name == needle {
			return true
		}
	}
	return false
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Security-Policy",csp)
	tmplIndex.ExecuteTemplate(w,"index.html",state)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Security-Policy",csp)
	
	// we only accept POST's
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	p := path.Clean(r.URL.Path)
	match, err := path.Match("/u/*/*",p)
	if err != nil {
		log.Println("[updateHandler] err:",err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !match {
		log.Println("[updateHandler] no match bro!",p)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	pp,n := path.Split(p)
	pr := path.Base(p[:len(pp)-1])

	// TODO:
	// - check for locked file
	// - actually update content
	f := &File{
		Name: n,
		Project: pr,
		Contents: "asdzxcasd",
	}
	f.Save()
	// HTTP 205:
	// The server successfully processed the request, but is not
	// returning any content. Unlike a 204 response, this response
	// requires that the requester reset the document view.
	w.WriteHeader(http.StatusResetContent)
	w.Header().Add("Location",fmt.Sprintf("/f/%s/%s",pr,n))
	return
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Security-Policy",csp)
	p := path.Clean(r.URL.Path)
	match, err := path.Match("/f/*/*",p)
	if err != nil {
		log.Println("[fileHandler] err:",err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !match {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	pp,n := path.Split(p)
	pr := path.Base(p[:len(pp)-1])
	log.Printf("[fileHandler] Debug: p{%s},n{%s},pr{%s},p[:len(p)]{%s}\n",pp,n,pr,p[:len(pp)-1])
	f := File{
		Name: n,
		Project: pr,
		Contents: "asdzxc",
	}
	tmplFile.ExecuteTemplate(w,"file.html",f)
}

func projectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Security-Policy",csp)
	// basename out the project name
	name := path.Base(r.URL.Path)
	p,err := state.GetProject(name)
	if err != nil {
		log.Printf("[ProjectHandler] Project{%s} not found!\n",name)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	log.Printf("[ProjectHandler] Serving up Project{%s}\n",p.Name)
	err = tmplProject.ExecuteTemplate(w,"project.html",p)
	if err != nil {
		log.Printf("[ProjectHandler] Executing the project template went to shit: %s\n",err)
	}
}

func initProject(path string, f os.FileInfo, err error) error {
	if err != nil {
		log.Fatal(err)
	}

	if f.IsDir() && f.Name() != "projs" {
		// global variable fulhack
		p := new(Project)
		p.Name = f.Name()
		state.Projects = append(state.Projects, p)
		log.Printf("[initProject] loaded '%s', refreshing the filelist\n",p.Name)
		err = p.UpdateFilelist()
		if err != nil {
			log.Printf("error from project{%s}.UpdateFilelist(): %s\n",p.Name,err)
		}
	}

	return nil
}

func main() {
	pwd = os.Getenv("PWD")
	if pwd == "" {
		panic("WTF")
	}

	var err error
	tmplIndex, err = template.New("index").ParseFiles("static/index.html")
	if err != nil {
		panic(err)
	}
	tmplProject, err = template.New("project").ParseFiles("static/project.html")
	if err != nil {
		panic(err)
	}
	tmplFile, err = template.New("file").ParseFiles("static/file.html")
	if err != nil {
		panic(err)
	}

	state = &State{}

	err = filepath.Walk(fmt.Sprintf("%s/%s",pwd,"projs/"), initProject)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/",defaultHandler)
	http.HandleFunc("/p/",projectHandler)
	http.HandleFunc("/f/",fileHandler)
	http.HandleFunc("/u/",updateHandler)
	log.Fatal(http.ListenAndServe(":8080",nil))
}
