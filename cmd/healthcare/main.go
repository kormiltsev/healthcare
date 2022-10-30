// 1. searcher() goes to dir ./files/*, try to read files .json, .tcx, .xml, .gpx
// 2. interface it and choose right reader()
// 3. reader() makes json to structure
// 4. reader() ask DB() to check for dubles and write new date in accordance table
package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	// read evn
	godotenv.Load()
	myToken := os.Getenv("TOKEN")
	if myToken == "" {
		log.Printf("input telegram bot unique token:") //in case no .env file, ask for TOKEN in terminal
		fmt.Fscan(os.Stdin, &myToken)
	}
	// check DB is available
	DBstart()
	// println all tables available

	// goes to dir
	Dirviewer() // starts reader(fileadres)
}
