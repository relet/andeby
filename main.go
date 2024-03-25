package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

const (
	screenx = 482
	screeny = 1072
)

// a macro is an adb swipe  executed when a given has is encountered
type Macro struct {
	X1        int
	Y1        int
	X2        int
	Y2        int
	Direction float32
	Strength  int
}

// a hashtable of macros
var macros = map[string]Macro{}

func ReadMacroDB() {
	// open sqlite table macros.db
	sqlite, err := sql.Open("sqlite3", "macros.db")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlite.Close()

	// create macro table if it does not exist
	if _, err := sqlite.Exec("CREATE TABLE IF NOT EXISTS macros (hash TEXT PRIMARY KEY, x1 INTEGER, y1 INTEGER, x2 INTEGER, y2 INTEGER, direction REAL, strength INTEGER)"); err != nil {
		log.Fatal(err)
	}

	// read all rows from the table
	rows, err := sqlite.Query("SELECT * FROM macros")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// iterate over each row and add the macro to the hashtable
	for rows.Next() {
		var hash string
		var x1 int
		var y1 int
		var x2 int
		var y2 int
		var direction float32
		var strength int
		if err := rows.Scan(&hash, &x1, &y1, &x2, &y2, &direction, &strength); err != nil {
			log.Fatal(err)
		}
		macros[hash] = Macro{x1, y1, x2, y2, direction, strength}
	}
}

func UpdateMacroDB(hash string, m Macro) {
	// update sqlite table macros.db
	sqlite, err := sql.Open("sqlite3", "macros.db")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlite.Close()

	strDirection := fmt.Sprintf("%.3f", m.Direction)

	// insert the macro into the table, update on conflict
	stmt, err := sqlite.Prepare("INSERT OR REPLACE INTO macros (hash, x1, y1, x2, y2, direction, strength) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(hash, m.X1, m.Y1, m.X2, m.Y2, strDirection, m.Strength); err != nil {
		log.Fatal(err)
	}
}

func RecordContact(id string, hash string, x1 int, y1 int, x2 int, y2 int) {
	fmt.Printf("recording contact %s from hash %s - x1: %d, y1: %d, x2: %d, y2: %d\n", id, hash, x1, y1, x2, y2)

	// calculate the direction and strength of the swipe
	strength := int(math.Sqrt(math.Pow(float64(x2-x1), 2) + math.Pow(float64(y2-y1), 2)))
	var direction float32
	direction = 0
	if strength > 0 {
		direction = float32(math.Atan2(float64(y2-y1), float64(x2-x1)) * 180 / math.Pi)
	}

	// add the macro to the hashtable
	macros[hash] = Macro{x1, y1, x2, y2, direction, strength}
	UpdateMacroDB(hash, Macro{x1, y1, x2, y2, direction, strength})

	fmt.Printf("added macro: %s - POS %d %d - STR %d - DIR %f\n", hash, x1, y1, strength, direction)
}

// Analyze checks if the given line contains a hash and executes the corresponding macro
func ScrcpyAnalyze(line string) {
	hash := line
	if hash == GetCurrentHash() {
		return
	}
	SetCurrentHash(hash)

	if macro, ok := macros[hash]; ok {
		// execute the macro
		if macro.Strength < 5 {
			fmt.Println("tap", macro.X1, macro.Y1)
			cmd := exec.Command("adb", "shell", "input", "tap", strconv.Itoa(macro.X1), strconv.Itoa(macro.Y1))
			if err := cmd.Run(); err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Println("swipe", macro.X1, macro.Y1, macro.X2, macro.Y2)
			cmd := exec.Command("adb", "shell", "input", "swipe", strconv.Itoa(macro.X1), strconv.Itoa(macro.Y1), strconv.Itoa(macro.X2), strconv.Itoa(macro.Y2), "500")
			if err := cmd.Run(); err != nil {
				log.Fatal(err)
			}
		}
	}
}

const (
	MT_MAX    = 4095
	WM_WIDTH  = 1080
	WM_HEIGHT = 2400
)

var contactId string
var contactHash string
var currentHash string
var startX int
var startY int
var endX int
var endY int

func SetContactId(id string) {
	contactId = id
}
func SetContactHash(hash string) {
	contactHash = hash
}
func GetContactId() string {
	return contactId
}
func GetContactHash() string {
	return contactHash
}
func GetCurrentHash() string {
	return currentHash
}
func SetCurrentHash(hash string) {
	currentHash = hash
}

func GeteventAnalyze(line string) {
	split := strings.Split(line, " ")
	if len(split) < 3 {
		return
	}
	// convert each from hex to int
	type_, err := strconv.ParseInt(split[0], 16, 64)
	if err != nil {
		log.Fatal(err)
	}
	code, err := strconv.ParseInt(split[1], 16, 64)
	if err != nil {
		log.Fatal(err)
	}
	value, err := strconv.ParseInt(split[2], 16, 64)
	if err != nil {
		log.Fatal(err)
	}

	// get contact id and record current hash at this point in time
	if type_ == 3 && code == 0x39 {
		if value == 0xffffffff {
			RecordContact(GetContactId(), GetContactHash(), startX, startY, endX, endY)
			SetContactHash("")
			SetContactId("")
			return
		}
		cur_hash := GetCurrentHash()
		SetContactId(fmt.Sprintf("%x", value))
		SetContactHash(cur_hash)
		fmt.Printf("contact id: %s - recorded hash: %s\n", fmt.Sprintf("%x", value), cur_hash)
		startX = -1
		startY = -1
	}

	// if the type is 3 (EV_ABS) and the code is 0 (ABS_X) or 1 (ABS_Y)
	if type_ == 3 && (code == 0x35 || code == 0x36) {
		// normalize the value to be between 0 and 1
		normalizedValue := float32(value) / MT_MAX
		// if the code is 0 (ABS_X), multiply by the width of the window
		if code == 0x35 {
			normalizedValue *= WM_WIDTH
			endX = int(normalizedValue)
			if startX == -1 {
				startX = endX
			}
		} else {
			// if the code is 1 (ABS_Y), multiply by the height of the window
			normalizedValue *= WM_HEIGHT
			endY = int(normalizedValue)
			if startY == -1 {
				startY = endY
			}
		}
	} else {
		//print integers as hex
		//fmt.Println("unknown type: ", line, fmt.Sprintf("%x", type_), fmt.Sprintf("%x", code), fmt.Sprintf("%x", value))
	}
}

func main() {
	ReadMacroDB()

	// execute the command "adb shell getevent /dev/input/event9" and read each line from stdout
	cmd1 := exec.Command("adb", "shell", "getevent", "/dev/input/event9")
	stdout, err := cmd1.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd1.Start(); err != nil {
		log.Fatal(err)
	}
	scanner1 := bufio.NewScanner(stdout)

	// execute the command "scrcpy" and read each line from stdout
	cmd2 := exec.Command("scrcpy")
	stdout, err = cmd2.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd2.Start(); err != nil {
		log.Fatal(err)
	}
	scanner2 := bufio.NewScanner(stdout)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for scanner2.Scan() {
			ScrcpyAnalyze(scanner2.Text())
		}
	}()
	wg.Add(1)
	go func() {
		for scanner1.Scan() {
			GeteventAnalyze(scanner1.Text())
		}
	}()
	wg.Wait()
}
