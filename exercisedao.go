// This code handles the database operations for the Exercise Tracker API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"os"
	"strconv"
	"time"
)

var exerciseCollection *mongo.Collection

type ExerciseUser struct {
	ID		 string `json:"_id" bson:"_id"`
	Username string `json:"username" bson:"username"`
}

type ExerciseRecord struct {
	Description string    `json:"description" bson:"description"`
	Duration    int       `json:"duration" bson:"duration"`
	Date        time.Time `json:"date" bson:"date"`
}

type ExerciseUserRecord struct {
	ID       string           `json:"_id" bson:"_id"`
	Username string           `json:"username" bson:"username"`
	Log		 []ExerciseRecord `json:"log,omitempty" bson:"log"`
}

type ExerciseAddedReceipt struct {
	ID			string    `json:"_id" bson:"_id"`
	Username	string    `json:"username" bson:"username"`
	Description string    `json:"description" bson:"description"`
	Duration    int       `json:"duration" bson:"duration"`
	Date        time.Time `json:"date" bson:"date"`
}

// Important stages in the aggregation pipeline that don't change.
// These get used if the user specifies a date range or a limit.
var (
	unwindStage bson.M = bson.M{"$unwind": "$log"}

	sortStage = bson.M{"$sort": bson.M{"log.date": 1}}

	regroupStage = bson.M{
		"$group": bson.M{
			"_id": "$_id",
			"username": bson.M{"$first": "$username"},
			"count": bson.M{"$first": "$count"},
			"log": bson.M{"$push": "$log"},
		},
	}
)


// Connect to the MongoDB database and get a reference to the exercise collection
func initExerciseCollection() {
	log.Println("Getting reference to exercise collection.")
	exerciseCollection = mongoClient.Database(os.Getenv("DB_NAME")).Collection(os.Getenv("COLLECTION_E"))
	if exerciseCollection == nil {
		log.Fatal("Failed to get pointer to exercise collection.\n")
	}
}


// Add a new user to the database, then return its ID
func createExerciseUser(uname string) []byte {
	log.Printf("Attempting to create new exercise user with username %q.\n", uname)
	funcName := "createExerciseUser"

	// Attempt to create a new record for the user.
	insertResult, err := exerciseCollection.InsertOne(context.TODO(), bson.M{"username": uname})
	if err != nil {
		log.Printf("Error in %s with Collection.InsertOne: %s\n", funcName, err)
		// The username is likely already taken, so try to find that user
		var foundUser ExerciseUser
		err = exerciseCollection.FindOne(context.TODO(), bson.M{"username": uname}).Decode(&foundUser)
		if err != nil {
			log.Printf("Error in %s with Collection.FindOne: %s\n", funcName, err)
			errorMessage := `{"error":"unable to create or find user with username` + uname + `"}`
			return []byte(errorMessage)
		}
		// Return the existing user's username and ID
		foundUserJSON, err := json.Marshal(foundUser)
		if err != nil {
			log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		}
		return foundUserJSON
	}

	// Insert was successful, so return the username with its newly created ID
	var newUser ExerciseUser
	newUser.Username = uname
	newUser.ID = fmt.Sprintf("%v", insertResult.InsertedID)
	newUserJSON, err := json.Marshal(newUser)
	if err != nil {
		log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
	}
	return newUserJSON
}


// Return the records of every user in the database
func getAllExerciseData() []byte {
	log.Println("Attempting to retrieve all exercise user data.")
	funcName := "getAllExerciseDate"

	// Execute a search with an empty filter interface
	// to get the entire contents of the database
	cursor, err := exerciseCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Printf("Error in %s with Collection.Find: %s\n", funcName, err)
		return []byte(`{"error":"Collection.Find failed"}`)
	}

	// Use the cursor to transfer all the contents into this slice of structs
	var userCollection []ExerciseUserRecord
	err = cursor.All(context.TODO(), &userCollection)
	if err != nil {
		log.Printf("Error in %s with Cursor.All: %s\n", funcName, err)
		return []byte(`{"error":"Cursor.All failed"}`)
	}

	// Convert the slice of structs to JSON
	userCollectionAsJSON, err := json.Marshal(userCollection)
	if err != nil {
		log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
		return []byte(`{"error":"json.Marshal failed"}`)
	}

	log.Printf("%d users' records will be returned.\n", len(userCollection))
	return userCollectionAsJSON
}


// Add a single exercise to an existing user's log
func addExerciseToUser(userID string, desc string, duration string, date string) []byte {
	log.Println("Attempting to add an exercise to a user.")
	funcName := "addExerciseToUser"

	// Make sure the ID is a valid MongoDB ObjectID
	if !primitive.IsValidObjectID(userID) {
		return []byte(`{"error":"invalid id"}`)
	}
	// Now convert the ID string to an actual MongoDB ObjectID
	userIDObject, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("Error in %s with primitive.ObjectIDFromHex: %s\n", funcName, err)
		return []byte(`{"error":"invalid id"}`)
	}

	// Convert the duration string to an int
	durationValue, err := strconv.Atoi(duration)
	if err != nil {
		log.Printf("Error in %s with strconv.Atoi: %s\n", funcName, err)
		return []byte(`{"error":"invalid duration"}`)
	}

	// Convert the date string to a Time object
	var dateObject time.Time
	if len(date) > 0 {
		dateObject, err = time.Parse("2006-01-02", date)
		if err != nil {
			log.Printf("Error in %s with time.Parse: %s\n", funcName, err)
			return []byte(`{"error":"invalid date"}`)
		}
	} else {
		dateObject = time.Now()
	}

	// Save all the above exercise data in an object.
	// This object will be added to the user's log in the database.
	newExercise := ExerciseRecord{
		Description: desc,
		Duration: durationValue,
		Date: dateObject,
	}
	log.Printf("Adding exercise: %+v\n", newExercise)

	// Note that FindOneAndUpdate returns the document "as it appeared before updating"
	var updatedDoc ExerciseUserRecord
	err = exerciseCollection.FindOneAndUpdate(
		context.TODO(),
		bson.M{"_id": userIDObject},
		bson.M{"$push": bson.M{"log": newExercise}},
	).Decode(&updatedDoc)
	if err != nil {
		log.Printf("Error in %s with Collection.FindOneAndUpdate: %s\n", funcName, err)
		errorString := `{"error":"unable to add exercise to` + userID + `"}`
		return []byte(errorString)
	}

	// Return to the user a combination of
	// the user object and new exercise object
	var receipt ExerciseAddedReceipt
	receipt.ID = updatedDoc.ID
	receipt.Username = updatedDoc.Username
	receipt.Description = desc
	receipt.Duration = durationValue
	receipt.Date = dateObject
	receiptInJSON, err := json.Marshal(receipt)
	if err != nil {
		log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
	}
	return receiptInJSON
}


// Return all the exercises for a specific user matching the given search criteria
func getExerciseLogsFromUser(userID string, fromDate string, toDate string, limit string) []byte {
	log.Printf("Attempting to retrieve exercise logs for %s.\n", userID)
	log.Printf("{_id: %s, from: %s, to: %s, limit: %s}\n", userID, fromDate, toDate, limit)
	funcName := "getExerciseLogsFromUser"

	// Validate the ID string
	if !primitive.IsValidObjectID(userID) {
		log.Println("Invalid user ID.")
		return []byte(`{"error":"invalid id"}`)
	}
	userIDObject, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Println("Unable to convert to ObjectID.")
		return []byte(`{"error":"invalid id"}`)
	}

	// Initialize the aggregation pipeline
	var pipe []bson.M

	// Create the stage that tries to find the user
	matchStage := bson.M{
		"$match": bson.M{
			"$and": bson.A{
				bson.M{"_id": userIDObject},
				bson.M{"log": bson.M{"$exists": true}},
			},
		},
	}
	pipe = append(pipe, matchStage)

	// Create the stage that counts the size of the user's exercise log
	addFieldsStage := bson.M{
		"$addFields": bson.M{
			"count": bson.M{"$size": "$log"},
		},
	}
	pipe = append(pipe, addFieldsStage)

	// Validate the "from" date parameter
	var fromDateObj time.Time
	fromDateWasValid := false
	if len(fromDate) > 0 {
		fromDateObj, err = time.Parse("2006-01-02", fromDate)
		fromDateWasValid = (err == nil)
	}

	// Validate the "to" date parameter
	var toDateObj time.Time
	toDateWasValid := false
	if len(toDate) > 0 {
		toDateObj, err = time.Parse("2006-01-02", toDate)
		toDateWasValid = (err == nil)
	}

	// Validate the "limit" parameter
	var limitVal int
	limitWasValid := false
	if len(limit) > 0 {
		limitVal, err = strconv.Atoi(limit)
		limitWasValid = (err == nil)
	}

	// Only continue if at least one of the 3 parameters was valid.
	// All of these require the use of an unwind stage.
	if fromDateWasValid || toDateWasValid || limitWasValid {
		// Unwind the log array and sort by log date
		pipe = append(pipe, unwindStage, sortStage)

		if fromDateWasValid && toDateWasValid {
			// from_date <= x <= to_date
			matchDate := bson.M{
				"$match": bson.M{
					"$and": bson.A{
						bson.M{"log.date": bson.M{"$gte": fromDateObj}},
						bson.M{"log.date": bson.M{"$lte": toDateObj}},
					},
				},
			}
			pipe = append(pipe, matchDate)
		} else if fromDateWasValid {
			// from_date <= x
			matchDate := bson.M{
				"$match": bson.M{
					"log.date": bson.M{"$gte": fromDateObj},
				},
			}
			pipe = append(pipe, matchDate)
		} else if toDateWasValid {
			// x <= toDate
			matchDate := bson.M{
				"$match": bson.M{
					"log.date": bson.M{"$lte": toDateObj},
				},
			}
			pipe = append(pipe, matchDate)
		}

		// The limit parameter determines how many entries
		// in the user's exercise log will be returned
		if limitWasValid {
			pipe = append(pipe, bson.M{"$limit": limitVal})
		}

		// Undo the unwind operation
		pipe = append(pipe, regroupStage)
	}

	// Execute the search
	cursor, err := exerciseCollection.Aggregate(context.TODO(), pipe)
	if err != nil {
		log.Printf("Error in %s with Collection.Aggregate: %s\n", funcName, err)
	}

	// Initialize a byte slice that will hold the JSON to be returned
	var docJSON []byte

	// Get the resulting document from the cursor
	if cursor.Next(context.TODO()) {
		var doc ExerciseUserRecord
		if err = cursor.Decode(&doc); err != nil {
			log.Printf("Error in %s with Cursor.Decode: %s\n", funcName, err)
			errorString := `{"error":"Cursor.Decode failed"}`
			docJSON = []byte(errorString)
		} else {
			// Convert the document to JSON
			docJSON, err = json.Marshal(doc)
			if err != nil {
				log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
			}
		}
	}

	if err = cursor.Err(); err != nil {
		// Perhaps the user exists but hasn't added to his/her log yet.
		var foundDoc ExerciseUserRecord
		err = exerciseCollection.FindOne(context.TODO(), bson.M{"_id": userIDObject}).Decode(&foundDoc)
		if err != nil {
			log.Printf("Error in %s with Collection.FindOne: %s\n", funcName, err)
			return []byte(`{"error":"invalid user"}`)
		} else {
			// Convert the document to JSON
			docJSON, err = json.Marshal(foundDoc)
			if err != nil {
				log.Printf("Error in %s with json.Marshal: %s\n", funcName, err)
			}
		}
	}

	return docJSON
}

