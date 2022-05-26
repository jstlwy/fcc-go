// Handles the database operations for the URL Shortener API. 
package main

import (
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"os"
	"strconv"
)

var urlCollection *mongo.Collection

type urlDBRecord struct {
	ID			 primitive.ObjectID `bson:"_id,omitempty"`
	OriginalURL  string             `bson:"original_url"`
	ShortURL     string             `bson:"short_url"`
	TimesVisited int                `bson:"times_visited"`
}

type urlReceipt struct {
	OriginalURL string `json:"original_url" bson:"original_url"`
	ShortURL    string `json:"short_url" bson:"short_url"`
}


// Get a pointer to the URL collection
func initURLCollection() {
	log.Println("Getting reference to URL collection.")
	urlCollection = mongoClient.Database(os.Getenv("DB_NAME")).Collection(os.Getenv("COLLECTION_U"))
	if urlCollection == nil {
		log.Fatal("Failed to get pointer to URL collection.\n")
	}
}


// Takes a pre-verified URL, creates a short URL for it,
// and inserts both into the database.
// Returns a JSON object containing both, e.g.: 
// { original_url: "https://freeCodeCamp.org",
//      short_url: 1 }
func insertURL(newURL string) []byte {
	funcName := "insertURL"

	// Get the current size of the database
	dbSize, err := urlCollection.CountDocuments(context.TODO(), bson.D{})
	if err != nil {
		log.Printf("Error in %s with Collection.CountDocuments: %s\n", funcName, err)
		errMsg := ErrorMessage{Content: "failed when counting database"}
		errMsgJSON, err := json.Marshal(errMsg)
		if err != nil {
			log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		}
		return errMsgJSON
	}
	// Now convert the database size to base 36.
	// This value will serve as the short URL.
	shortURL := strconv.FormatInt(dbSize, 36)

	// Now add the new record to the database.
	newDoc := urlDBRecord{
		OriginalURL: newURL,
		ShortURL: shortURL,
		TimesVisited: 0,
	}
	log.Printf("Attempting to add this record to the database:\n%+v\n", newDoc)
	insertResult, err := urlCollection.InsertOne(context.TODO(), newDoc)

	// Check whether the insert operation was successful
	if err != nil && mongo.IsDuplicateKeyError(err) {
		// This URL is already in the database, so find its record
		var oldDoc urlReceipt
		err = urlCollection.FindOne(context.TODO(), bson.M{"original_url":newURL}).Decode(&oldDoc)
		if err != nil {
			log.Printf("Error in %s with Collection.FindOne: %s\n", funcName, err)
		}
		log.Printf("Duplicate URL. Short URL: %s\n", oldDoc.ShortURL)
		// Convert it to JSON and return it
		oldDocJSON, err := json.Marshal(oldDoc)
		if err != nil {
			log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		}
		return oldDocJSON
	} else if err != nil {
		// Handle any other errors that may have occurred
		log.Printf("Error in %s with Collection.InsertOne: %s\n", funcName, err)
		errMsg := ErrorMessage{Content: "failed when inserting into database"}
		errMsgJSON, err := json.Marshal(errMsg)
		if err != nil {
			log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		}
		return errMsgJSON
	}

	log.Printf("New URL document inserted: %v\n", insertResult.InsertedID)

	// Finally, return JSON object showing original and short URLs
	receipt := urlReceipt{
		OriginalURL: newURL,
		ShortURL: shortURL,
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		errMsg := ErrorMessage{Content: "failed when marshaling to JSON"}
		errMsgJSON, err := json.Marshal(errMsg)
		if err != nil {
			log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		}
		return errMsgJSON
	}
	return receiptJSON
}


// Search for a short URL and return its corresponding original URL.
func getOriginalURL(sURL string) string {
	log.Printf("Attempting to retrieve original URL for: %s\n", sURL)
	funcName := "getOriginalURL"

	// Execute the search for the URL
	var foundDoc urlDBRecord
	err := urlCollection.FindOne(context.TODO(), bson.M{"short_url": sURL}).Decode(&foundDoc)
	if err != nil {
		log.Printf("Error in %s with Collection.FindOne: %s\n", funcName, err)
		return ""
	}

	//log.Printf("Found document: %+v\n", foundDoc)

	// Increment this URL's "times_visited" parameter
	filter := bson.M{"_id": foundDoc.ID}
	command := bson.M{"$inc": bson.M{"times_visited": 1}}
	//result, err := urlCollection.UpdateOne(context.TODO(), filter, command)
	_, err = urlCollection.UpdateOne(context.TODO(), filter, command)
	if err != nil {
		log.Printf("Error in %s with Collection.UpdateOne: %s\n", funcName, err)
	} else {
		log.Println("Successfully incremented its times_visited counter.")
		//log.Printf("matched = %d, modified = %d", result.MatchedCount, result.ModifiedCount)
	}

	return foundDoc.OriginalURL
}

