// Loads the environment variables from a .env file.
package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

const filename string = ".env"

func loadEnvVars() {
	log.Println("Loading environment variables.")

	// Open the .env file
	file, openErr := os.Open(filename)
    if openErr != nil {
		log.Fatal("Error when opening .env file: %s\n", openErr)
    }
    defer file.Close()

	// Create a scanner with which to read from the file line by line 
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
		// Get the current line
		currentLine := scanner.Text()
		// Get the index of the first occurrence of "=".
		// Key names should never contain this character,
		// so this will tell us where keys end and values begin
		boundary := strings.Index(currentLine, "=")
		key := currentLine[:boundary]
		value := currentLine[boundary+1:]
		// Save the key and value in the environment variables
		setEnvErr := os.Setenv(key, value)
		if setEnvErr != nil {
			log.Fatal("Error when adding environment variable: %s\n", setEnvErr)
		}
    }
    if scanErr := scanner.Err(); scanErr != nil {
		log.Fatal("Error when scanning .env file: %s\n", scanErr)
	}
}

