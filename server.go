// A reimplementation of all freeCodeCamp back-end projects in pure Go
// (i.e. besides the MongoDB API, only using the standard library).
package main

import (
	"context"
	"encoding/json"
	"fmt"
    "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type ErrorMessage struct {
	Content string `json:"error"`
}

type WhoamiStruct struct {
	IpAddress string `json:"ipaddress"`
	Language  string `json:"language"`
	UserAgent string `json:"software"`
}

type DateStruct struct {
	UNIXDate int64  `json:"unix"`
	UTCDate  string `json:"utc"`
}

type FileMetadataStruct struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64 `json:"size"`
}

var mongoClient *mongo.Client


func init() {
	loadEnvVars()
	var err error
	mongoClient, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("DB_URI")))
	if err != nil {
		log.Fatal("Error when connecting to MongoDB: %s\n", err)
	}
	initURLCollection()
	initExerciseCollection()
}


func main() {
	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("./static"))
	//mux.Handle("/static/", http.StripPrefix("/static", fs))
	mux.Handle("/", fs)

	// Simple APIs that only return JSON
	mux.HandleFunc("/request/", getRequestInfo)
	mux.HandleFunc("/whoami/", getVisitorInfo)
	mux.HandleFunc("/hello/", sendJSONGreeting)
	mux.HandleFunc("/date/", getDate)

	// File metadata API
	mux.HandleFunc("/file/analyze/", getFileMetadata)

	// URL shortener API
	mux.HandleFunc("/shorturl/new/", createShortURL)
	mux.HandleFunc("/shorturl/go/", openShortURL)

	// Exercise tracker API
	mux.HandleFunc("/exercise/users/", handleExerciseUsersPath)

	// Ensure that the program closes the database connection when shutting down
	defer func() {
		log.Printf("Closing connection to MongoDB.\n")
		err := mongoClient.Disconnect(context.TODO())
		if err != nil {
			log.Printf("error when disconnecting from MongoDB: %s\n", err)
		}
	}()

	port := "8000"
	log.Printf("Starting app on port %s.\n", port)
	err := http.ListenAndServe("localhost:" + port, mux)
	log.Fatal(err)
}


// Prints everything in the HTTP request object.
func getRequestInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request for HTTP request object headers.\n")
	//w.Header().Set("Content-Type", "application/json")
	//w.WriteHeader(http.StatusCreated)

	fmt.Fprintf(w, "%s %s %s\n", r.Method, r.URL, r.Proto)

	fmt.Fprintf(w, "\nHEADERS\n")
	for key, value := range r.Header {
		fmt.Fprintf(w, "%q: %q\n", key, value)
	}

	fmt.Fprintf(w, "\nHost: %q\n", r.Host)
	fmt.Fprintf(w, "RemoteAddr: %q\n", r.RemoteAddr)

	if err := r.ParseForm(); err != nil {
		log.Print(err)
	}
	fmt.Fprintf(w, "\nFORM VALUES\n")
	for key, value := range r.Form {
		fmt.Fprintf(w, "%q: %q\n", key, value)
	}

	fmt.Fprintf(w, "\nURL path: %q\n", r.URL.Path)
}


// Responds with a simple greeting in JSON format.
func sendJSONGreeting(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request for JSON greeting.\n")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"greeting":"Hello, world!"}`)
}


// Returns a JSON object containing the visitor's
// IP address, accept-language, and user-agent
func getVisitorInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request for visitor's info.\n")

	// Extract all relevant info from the request object
	ipAddr, _, _ := net.SplitHostPort(r.RemoteAddr)
	var response WhoamiStruct
	response.IpAddress = ipAddr
	response.Language = r.Header.Get("Accept-Language")
	response.UserAgent = r.Header.Get("User-Agent")
	fmt.Printf("%+v\n", response)

	// Encode it in JSON and send it back to the user
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error in getVisitorInfo when encoding JSON: %s\n", err)
	}
}


// Returns a JSON object containing the current date or a user-specified date
// in both UNIX format (seconds since epoch) and RFC1123 format.
// Example:
// { "unix": 1451001600000,
//    "utc": "Fri, 25 Dec 2015 00:00:00 GMT" }
func getDate(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request for the time in JSON.\n")
	funcName := "getDate"

	dateParam := strings.TrimPrefix(r.URL.Path, "/date/")
	var response DateStruct
	dateCouldBeParsed := false

	// If the user passed a date, validate it
	if len(dateParam) > 0 {
		// The user can pass a date in one of the two following formats:
		// 1. %Y-%m-%d, e.g. 2015-12-25
		// 2. Seconds since epoch, e.g. 1451001600000

		// Check first for the latter type by trying to convert the date to an int64.
		if seconds, err := strconv.ParseInt(dateParam, 10, 64); err == nil {
			// Successfully converted to int, so this should be seconds since epoch
			parsedTime := time.Unix(seconds, 0)
			if err != nil {
				log.Printf("Error in %s: %s\n", funcName, err)
			} else {
				response.UNIXDate = parsedTime.Unix()
				response.UTCDate = parsedTime.Format(time.RFC1123)
				dateCouldBeParsed = true
			}
		} else {
			// Failed at converting to int, so this might be a %Y-%m-%d date
			parsedTime, err := time.Parse("2006-01-02", dateParam)
			if err != nil {
				log.Printf("Error in %s: %s\n", funcName, err)
			} else {
				response.UNIXDate = parsedTime.Unix()
				response.UTCDate = parsedTime.Format(time.RFC1123)
				dateCouldBeParsed = true
			}
		}
	}

	// If the user didn't pass a date,
	// or the date that was passed was invalid,
	// just return the current date
	if !dateCouldBeParsed {
		currentTime := time.Now()
		response.UNIXDate = currentTime.Unix()
		response.UTCDate = currentTime.Format(time.RFC1123)
	}

	// Print to the console for debug purposes
	log.Printf("%+v\n", response)

	// Finally, send it to the user as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
	}
}


// Processes a file uploaded by the user and returns a JSON object
// with the file's original name, [MIME] type, and size.
func getFileMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Access denied", http.StatusMethodNotAllowed)
	}

	log.Printf("Request for file metadata.\n")
	funcName := "getFileMetadata"

	// Load the body of the request
	const maxUploadSize = 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	err := r.ParseMultipartForm(maxUploadSize)
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
	}

	// Extract the uploaded file from the request body
	filename := "upfile"
	file, fileHeader, err := r.FormFile(filename)
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
	}
	defer file.Close()

	// Get the file type
	var contentTypeArray []string = fileHeader.Header["Content-Type"]
	var contentType string
	if len(contentTypeArray) > 0 {
		contentType = contentTypeArray[0]
	} else {
		contentType = "unknown"
	}

	// Save some of the file's metadata in a struct
	var fileInfo FileMetadataStruct
	fileInfo.Name = fileHeader.Filename
	fileInfo.Type = contentType
	fileInfo.Size = fileHeader.Size
	log.Printf("%+v\n", fileInfo)

	// Send the metadata to the visitor as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(fileInfo)
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
	}
}


// Given a URL, creates a short URL and sends it to the user in a JSON object
func createShortURL(w http.ResponseWriter, r *http.Request) {
	log.Println("Request to create short URL.")
	funcName := "createShortURL"

	// Prepare to send the results back to the visitor as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// Read in the HTML form data
	if err := r.ParseForm(); err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
		fmt.Fprintf(w, `{"error":"unable to parse form"}`)
		return
	}

	// Get the URL from the form data
	originalURL := r.Form.Get("url")
	log.Printf("Before formatting: %s\n", originalURL)
	// The URL needs to start with "http://" in order to be parsed correctly,
	// and "https://" causes errors.
	originalURL = strings.TrimPrefix(originalURL, "https://")
	if !strings.HasPrefix(originalURL, "http://") {
		originalURL = "http://" + originalURL
	}
	log.Printf("After formatting: %s\n", originalURL)

	// Check if the format of the URL is valid
	urlObject, err := url.Parse(originalURL)
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
		fmt.Fprintf(w, `{"error":"invalid url"}`)
		return
	}
	log.Println("Successfully parsed URL.")

	// See if the hostname is valid by trying to look it up via DNS
	addresses, err := net.LookupHost(urlObject.Hostname())
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
		fmt.Fprintf(w, `{"error":"invalid hostname"}`)
		return
	}
	log.Printf("Found addresses for %s: %v\n", urlObject.Hostname(), addresses)

	// Dial the original URL
	/*
	conn, err := net.Dial("tcp", urlObject.Hostname() + ":http")
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
	} else {
		conn.Close()
		log.Println("Got a response from the server when dialing the URL.")
	}
	*/

	// Attempt to add it to the database
	resultJSON := insertURL(strings.TrimPrefix(originalURL, "http://"))
	w.Write(resultJSON)
}


// Given a short URL, finds the corresponding original URL and redirects to it
func openShortURL(w http.ResponseWriter, r *http.Request) {
	shortURL := strings.TrimPrefix(r.URL.Path, "/shorturl/go/")
	log.Printf("Request for short URL: %s\n", shortURL)

	// Return if no URL was passed
	if len(shortURL) == 0 {
		http.NotFound(w, r)
	}

	originalURL := getOriginalURL(shortURL)
	log.Printf("Redirecting to: %s\n", originalURL)
	if !strings.HasPrefix(originalURL, "http://") {
		http.Redirect(w, r, "http://" + originalURL, 307)
	} else {
		http.Redirect(w, r, originalURL, 307)
	}
}


// For the Exercise Tracker API,
// process requests to add a user,
// add an exercise to a user's log,
// get exercise logs for a specific user,
// or get all the data in the database.
func handleExerciseUsersPath(w http.ResponseWriter, r *http.Request) {
	log.Println("Exercise API accessed.")
	funcName := "handleExerciseUsersPath"

	// Prepare to send JSON back to the visitor
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	//log.Printf("User's request URI: %s\n", r.URL.Path)
	requestDestination := strings.TrimPrefix(r.URL.Path, "/exercise/users/")
	//log.Printf("User's request destination: %s\n", requestDestination)

	if len(requestDestination) == 0 && r.Method == "GET" {
		// Get all user info
		userData := getAllExerciseData()
		w.Write(userData)
		return
	}

	// For every other option, the form data must be parsed.
	err := r.ParseForm()
	if err != nil {
		log.Printf("Error in %s: %s\n", funcName, err)
	}

	switch {
	case len(requestDestination) == 0 && r.Method == "POST":
		// Add a new user
		username := r.Form.Get("username")
		//log.Printf("Request to add new exercise user: %s\n", username)
		log.Println("Request to add new exercise user.")
		newUserRecord := createExerciseUser(username)
		w.Write(newUserRecord)
	case len(requestDestination) > 0 && r.Method == "GET":
		// Get exercise logs for a specific user
		// First, get the user ID from the URI
		slashIndex := strings.Index(requestDestination, "/")
		id := requestDestination[:slashIndex]
		// Next, extract the query parameters (if there were any)
		q := r.URL.Query()
		fromDate := q.Get("from")
		toDate := q.Get("to")
		numRecordsToReturn := q.Get("limit")
		log.Println("Request for logs for a specific exercise user.")
		//log.Printf("{_id: %s, from: %s, to: %s, limit: %s}\n", id, fromDate, toDate, numRecordsToReturn)
		logUpdatedReceipt := getExerciseLogsFromUser(id, fromDate, toDate, numRecordsToReturn)
		w.Write(logUpdatedReceipt)
	case len(requestDestination) > 0 && r.Method == "POST":
		// Add an exercise to a specific user's log
		// First, get the data from the form that the user posted
		id := r.Form.Get(":_id")
		description := r.Form.Get("description")
		duration := r.Form.Get("duration")
		date := r.Form.Get("date")
		log.Println("Request to add exercise to specific user's log:")
		log.Printf("{_id: %s, description: %s, duration: %s, date: %s}\n", id, description, duration, date)
		logAddedReceipt := addExerciseToUser(id, description, duration, date)
		w.Write(logAddedReceipt)
	default:
		http.NotFound(w, r)
	}
}

