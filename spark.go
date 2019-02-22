package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/cors"
)

var (
	address = flag.String("address", "0.0.0.0", "Listening address")
	port    = flag.String("port", "8080", "Listening port")
	origin  = flag.String("origin", "*", "CORS Origin")
	sslPort = flag.String("sslPort", "10433", "SSL listening port")
	path    = flag.String("path", "/", "URL path")
	deny    = flag.String("deny", "", "Sesitive directory or file patterns to be denied when serving directory (comma sperated)")
	status  = flag.Int("status", 200, "Returned HTTP status code")
	cert    = flag.String("cert", "cert.pem", "SSL certificate path")
	key     = flag.String("key", "key.pem", "SSL private Key path")
)

type bytesHandler []byte

func (h bytesHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(*status)
	w.Write(h)
}

func isDenied(path, denyList string) bool {
	if len(denyList) == 0 {
		return false
	}
	for _, pathElement := range strings.Split(path, string(filepath.Separator)) {
		for _, denyElement := range strings.Split(denyList, ",") {
			match, err := filepath.Match(strings.TrimSpace(denyElement), pathElement)
			if err != nil {
				log.Print("error matching file path element: ", err)
			}
			if match {
				return true
			}
		}
	}
	return false
}

type protectdFileSystem struct {
	fs http.FileSystem
}

func (pfs protectdFileSystem) Open(path string) (http.File, error) {
	if isDenied(path, *deny) {
		return nil, os.ErrPermission
	}
	return pfs.fs.Open(path)
}

func corsMiddleware(origin string) *cors.Cors {
	return cors.New(cors.Options{
		AllowedOrigins: []string{origin},
		AllowedMethods: []string{"GET", "POST", "DELETE", "PUT", "PATCH"},
		AllowedHeaders: []string{"Accept", "Access-Token", "Authorization", "Content-Type",
			"Version", "X-Api-Key", "Origin", "Recaptcha-Token"},
		AllowCredentials: true,
	})
}

func main() {
	flag.Parse()
	listen := *address + ":" + *port
	listenTLS := *address + ":" + *sslPort
	body := flag.Arg(0)
	if body == "" {
		body = "."
	}
	var handler http.Handler
	if fi, err := os.Stat(body); err == nil {
		switch mode := fi.Mode(); {
		case mode.IsDir():
			if *deny == "" {
				log.Print("Warning: serving files without any filter!")
			}
			handler = http.StripPrefix(
				*path,
				corsMiddleware(*origin).Handler(
					http.FileServer(protectdFileSystem{http.Dir(body)}),
				),
			)
		case mode.IsRegular():
			if content, err := ioutil.ReadFile(body); err != nil {
				log.Fatal("Error reading file: ", err)
			} else {
				handler = corsMiddleware(*origin).Handler(bytesHandler(content))
			}
		}
	} else {
		handler = corsMiddleware(*origin).Handler(bytesHandler(body))
	}
	http.Handle(*path, handler)
	go func() {
		if _, err := os.Stat(*cert); err != nil {
			return
		}
		if _, err := os.Stat(*key); err != nil {
			return
		}
		log.Fatal(http.ListenAndServeTLS(listenTLS, *cert, *key, nil))
	}()
	log.Printf("Serving %s on %s%s...", body, listen, *path)
	log.Fatal(http.ListenAndServe(listen, nil))
}
