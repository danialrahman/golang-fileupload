package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var port = "8080"
const MAX_UPLOAD_SIZE =  8 * 1024 * 1024 // 8MB

type Image struct {
    Id   int    `json:"id"`
    Path string `json:"path"`
    Filename string `json:"filename"`
    Size float32 `json:"size"`
    MimeType string `json:"mimetype"`
    Extension string `json:"extension"`
    Created string `json:"created_at"`
}

func uploadFileController(w http.ResponseWriter, r *http.Request){

	// Check upload size not exceeding limit
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		http.Error(w, "The uploaded file is too big (max: 8M) ", http.StatusBadRequest)
		return
	}

	// Retrieve file from posted form-data
	file, handler, err := r.FormFile("imageFile")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Specify file variables
	var fileName = handler.Filename
	var fileSize = handler.Size
	var fileExt = filepath.Ext(handler.Filename)
	var mimeType = handler.Header.Get("Content-type")

	// Only allow image file (jpeg/png)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		http.Error(w, "The provided file format is not allowed. Please upload a JPEG or PNG image", http.StatusBadRequest)
		return
	}

	// Write temporary file on server
	tempFile, err := ioutil.TempFile("uploaded",
	"upload-*"+fileExt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}
	tempFile.Write(fileBytes)
	
	// Upload to database
    db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/go_image_server")
    if err != nil {
        panic(err.Error())
    }
    defer db.Close()

	var image Image
	sqlStatement := "INSERT INTO images VALUES ( NULL, ?, ?, ?, ?, ?, NOW());"
    db.QueryRow(sqlStatement, tempFile.Name(), fileName, fmt.Sprint(fileSize), mimeType, fileExt).Scan(&image.Id);
		
	// Redirect back the page
	http.Redirect(w, r, r.Header.Get("Referer"), 302)
}

func fileListingController(w http.ResponseWriter, r *http.Request){
	// Fetch from database
    db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/go_image_server")
    if err != nil {
        panic(err.Error())
    }
    defer db.Close()

    results, err := db.Query("SELECT * FROM Images order by created_at DESC")
    if err != nil {
        panic(err.Error())
    }

	var imageCollections []Image
    for results.Next() {
        var image Image
        // for each row, scan the result into our image composite object
        err = results.Scan(
			&image.Id, 
			&image.Path, 
			&image.Filename, 
			&image.Size, 
			&image.MimeType, 
			&image.Extension, 
			&image.Created,
		)
        if err != nil {
            panic(err.Error())
        }
		imageCollections = append(imageCollections, image)
    }
		
	json.NewEncoder(w).Encode(imageCollections)
}


// File route resource
func fileResource(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
		case http.MethodGet:
			fileListingController(w, r)
		case http.MethodPost:
			uploadFileController(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}




func getTokenController(w http.ResponseWriter, r *http.Request){
	// Fetch from env
    var token = goDotEnvVariable("TOKEN")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

// token route resource
func tokenResource(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
		case http.MethodGet:
			getTokenController(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func authTokenMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check token
		if(r.FormValue("auth") != goDotEnvVariable("TOKEN")){
			http.Error(w, "Unauthorized", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        next.ServeHTTP(w, r)
    })
}

// Handle all routes
func setupRoutes() {
	mux := http.NewServeMux()
    mux.Handle("/file", authTokenMiddleware(http.HandlerFunc(fileResource)))
    mux.Handle("/token", http.HandlerFunc(tokenResource))
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	http.ListenAndServe(":"+port, mux)
}

// use godot package to load/read the .env file
func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

// Main
func main() {
	fmt.Println("----------------------------------------------")
	fmt.Println("Running image server on port "+port+"...")
	fmt.Print("---------------------------------------------- \n\n")

	setupRoutes()
}