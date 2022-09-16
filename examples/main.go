package main

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go"
)

// TODO: add in ability to do INFO command on database
// TODO: should let users specify a selector other than '*' for select statements
// TODO: set up docker container so it also allows the client to sign in
func main() {

	println("Step 1: Connect to SurrealDB")
	db, newErr := surrealdb.New("ws://localhost:8000/rpc")
	if newErr != nil {
		panic(newErr)
	}
	defer db.Close()

	println("Step 2: Sign In")
	_, signInErr := db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	if signInErr != nil {
		panic(signInErr)
	}

	println("Step 3: Set Namespace and Database")
	_, useErr := db.Use("test", "test")
	if useErr != nil {
		panic(useErr)
	}

	println("Step 4: Check If There Are Rows Already In The Database. If Rows Exist, Delete Them")
	rows, selectOneErr := db.SchemalessSelect("company")
	if selectOneErr != nil {
		panic(selectOneErr)
	}

	if len(rows) > 0 {
		ids := make([]string, 0)
		for _, row := range rows {
			idString := fmt.Sprintf("%v", row["id"])
			println("          found existing company: " + idString)
			ids = append(ids, idString)
		}

		for _, id := range ids {
			_, deletionErr := db.Delete(id)
			if deletionErr != nil {
				panic(deletionErr)
			}
			println("          deleted company: " + id)
		}
	}

	println("Step 5: Check To Make Sure The Deletions Worked")
	rowsAfterDeletion, selectTwoErr := db.SchemalessSelect("company")
	if selectTwoErr != nil {
		panic(selectTwoErr)
	}

	if len(rowsAfterDeletion) != 0 {
		panic("there should be no rows in the database for the 'company' table!")
	}

	println("Step 6: Create A Row In The 'company' Table")
	_, createOneErr := db.Create("company:100", map[string]interface{}{
		"name":           "new company 100",
		"initial_shares": "100",
	})
	if createOneErr != nil {
		panic(createOneErr)
	}
	println("          created company:100")

	println("Step 7: Check To Make Sure The Row Was Created")
	companiesInDatabaseOne, err := db.SchemalessSelect("company")
	if err != nil {
		panic(err)
	}

	for _, row := range companiesInDatabaseOne {
		companyName := fmt.Sprintf("%v", row["name"])
		println("          found row in company table: " + companyName)
	}

	println("Step 8: Prove The 'company' Table Is Schemaless. Create A Row Without 'initial_shares'")
	_, createTwoErr := db.Create("company:200", map[string]interface{}{
		"name": "new company 200",
	})
	if createTwoErr != nil {
		panic(createTwoErr)
	}
	println("          created company:200")

	println("Step 9: Check The New Row Was Created In The 'company' Table")
	rowsAfterThirdCreate, createThreeErr := db.SchemalessSelect("company")
	if createThreeErr != nil {
		panic(createThreeErr)
	}
	for _, row := range rowsAfterThirdCreate {
		print("          found row in company table: ")
		fmt.Println(row)
	}

}
