// 1. searcher() goes to dir ./files/*, try to read files .json, .tcx, .xml, .gpx
// 2. interface it and choose right reader()
// 3. reader() makes json to structure
// 4. reader() ask DB() to check for dubles and write new date in accordance table

// this stage count 1,5 mln lines to 1,5 mln in pg in more than 15 minutes (dont know)
// possible to start goroutines to check doubles after every fire red
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/joho/godotenv"
)

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

// google fit ==============================
type AutoGenerated struct {
	DataSource string      `json:"Data Source"`
	DataPoints []DataPoint `json:"Data Points"`
}

type DataPoint struct {
	FitValue           []FitVal `json:"fitValue"`
	OriginDataSourceID string   `json:"originDataSourceId"`
	EndTimeNanos       int64    `json:"endTimeNanos"`
	DataTypeName       string   `json:"dataTypeName"`
	StartTimeNanos     int64    `json:"startTimeNanos"`
	ModifiedTimeMillis int64    `json:"modifiedTimeMillis"`
	RawTimestampNanos  int      `json:"rawTimestampNanos"`
}

type FitVal struct {
	Value Valu `json:"value"`
}

type Valu struct {
	IntVal int     `json:"intVal"`
	FpVal  float64 `json:"fpVal"`
}

// ========================================
// google fit to base ===========
type GoogleFit struct { // != AutoGenerated
	DataSource         string  `json:"Data Source"`
	IntVal             int     `json:"intVal"`
	FpVal              float64 `json:"fpVal"`
	OriginDataSourceID string  `json:"originDataSourceId"`
	EndTimeNanos       int64   `json:"endTimeNanos"`
	DataTypeName       string  `json:"dataTypeName"`
	StartTimeNanos     int64   `json:"startTimeNanos"`
	ModifiedTimeMillis int64   `json:"modifiedTimeMillis"`
	RawTimestampNanos  int     `json:"rawTimestampNanos"`
}

// ==============================
type errs struct {
	Err     []string
	Doubles []string
}

var er errs
var str AutoGenerated
var row DataPoint
var exitrow GoogleFit
var exitlist []GoogleFit

func main() {
	// read evn
	// connection DB =================================
	godotenv.Load()

	adr := os.Getenv("DBADR")
	usr := os.Getenv("DBUSER")
	pwd := os.Getenv("DBPWD")
	dbs := os.Getenv("DBTYPE")
	if adr == "" {
		log.Printf("ERR: ENV cant find DB specs in .env")
	}
	// connect to DB
	db := pg.Connect(&pg.Options{
		Addr:     adr,
		User:     usr,
		Password: pwd,
		Database: dbs,
	})
	defer db.Close()
	//===============================================
	// println all tables available
	//f := AutoGenerated{}
	//f.DBselect(db)
	// goes to dir
	// create if not exist ==
	err := db.CreateTable(&exitrow, &orm.CreateTableOptions{
		Temp:          false, // create temp table
		IfNotExists:   true,
		FKConstraints: true,
	})
	panicIf(err)
	// ======================
	// count total strings in DB ===
	var totalstringinDB []GoogleFit
	_, err = db.Query(&totalstringinDB, `SELECT data_source, origin_data_source_id, start_time_nanos, fp_val, int_val FROM google_fits`)
	fmt.Println("Total rows in DB before start = ", len(totalstringinDB))
	//===============================
	//
	fdir := "./data/googlefit/*.json"

	jsons, err := filepath.Glob(fdir)
	if err != nil {
		fmt.Println("ERR: DIR no json files in directory", err)
	}
	googleqty := len(jsons)
	googlered := 0
	strokqty := 0
	strokred := 0

	for _, filename := range jsons {
		valuetocheck := false
		{
			f, err := os.Open(filename)
			if err != nil {
				er.Err = append(er.Err, fmt.Sprint("ERR: FILE ", fdir, " Cant open file ", filename))
				continue
			}
			//defer f.Close()
			jsonParser := json.NewDecoder(f)
			var str AutoGenerated
			if err := jsonParser.Decode(&str); err != nil {
				er.Err = append(er.Err, fmt.Sprint("ERR: JSON ", fdir, " Cant decode json. File name is ", filename))
				f.Close()
				continue
			}
			f.Close()
			googlered += 1
			strokqty += len(str.DataPoints)
			exitrow.DataSource = str.DataSource
			// read what is in srting ===
			for i, dataPoints := range str.DataPoints { // array in one file
				if len(dataPoints.FitValue) != 1 { // if some difference
					er.Err = append(er.Err, fmt.Sprint("ERR: STRUCT changed: Google Fit 'fitValue' is more than 1 in array. File name is ", filename, " error string is ", i))
					break
				}
				fitValue := dataPoints.FitValue[0]
				val := fitValue.Value //.IntVal

				exitrow.IntVal = val.IntVal
				exitrow.FpVal = val.FpVal
				exitrow.OriginDataSourceID = dataPoints.OriginDataSourceID
				exitrow.StartTimeNanos = dataPoints.StartTimeNanos
				exitrow.EndTimeNanos = dataPoints.EndTimeNanos
				exitrow.DataTypeName = dataPoints.DataTypeName
				exitrow.ModifiedTimeMillis = dataPoints.ModifiedTimeMillis
				exitrow.RawTimestampNanos = dataPoints.RawTimestampNanos

				exitlist = append(exitlist, exitrow)
				strokred += 1
				// check if all of rows are 0. possible new parametr on Value
				valuetocheck = valuetocheck || val.IntVal != 0 || val.FpVal != 0.0
				// =========================
				// check for dubles =========
				for _, old := range totalstringinDB {
					if old.StartTimeNanos == dataPoints.StartTimeNanos && old.DataSource == str.DataSource {
						er.Doubles = append(er.Doubles, fmt.Sprint(filename, " Double ROW = ", i))
						exitlist = exitlist[:len(exitlist)-1]
						break
					}
				}
				// ==========================

			}
			if !valuetocheck {
				googlered -= 1
				er.Err = append(er.Err, fmt.Sprint("ERR: STRUCT error, may be empty, or error in struct type. File: ", filename))
			}
		}
		// if err != nil {
		// 	fmt.Println("Error parsing Google Fit", err)
		// }

	}
	fmt.Printf("Opened successfuly %d/%d files.\nRed successfuly %d/%d rows.\n", googlered, googleqty, strokred, strokqty)
	if googlered != googleqty || strokred != strokqty {
		for _, s := range er.Err {
			fmt.Println(s)
		}
	}
	if len(er.Doubles) != 0 {
		fmt.Println("Doubles:\n")
		for _, s := range er.Doubles {
			fmt.Println(s)
		}
	}
	// push to pg
	if len(exitlist) != 0 {
		err = db.Insert(&exitlist)
		if err != nil {
			panic(err)
		}
	}
	//===========
	fmt.Println("Rows were added = ", len(exitlist))
	// count total strings in DB (not work like this) need new connection
	// var totalstringinDBnew []GoogleFit
	// err = db.Model(&totalstringinDBnew).Select()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println("Total rows in DB as result = ", len(totalstringinDB))
	//Dirviewer() // starts reader(fileadres)
}
