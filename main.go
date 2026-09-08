package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	_ "modernc.org/sqlite"
)

//go:embed templates/index.html templates/index2.html templates/index3.html
var templateFS embed.FS

var suitePattern = regexp.MustCompile(`^[A-Za-z0-9]{3,4}$`)

type App struct {
	db       *sql.DB
	tpl      *template.Template
	testTpl2 *template.Template
	testTpl  *template.Template
}

type Suite struct {
	SuiteID           string             `json:"suite_id"`
	DateUpdated       string             `json:"date_updated"`
	EnterCode         string             `json:"entercode"`
	BackupKeysetNum   string             `json:"backup_keyset_num"`
	OwnerIsResident   bool               `json:"owner_is_resident"`
	Residents         []Resident         `json:"residents"`
	EmergencyContacts []EmergencyContact `json:"emergency_contacts"`
	Vehicles          []Vehicle          `json:"vehicles"`
	ParkingSpots      []string           `json:"parking_spots"`
	Lockers           []string           `json:"lockers"`
	Owner             Owner              `json:"owner"`
}

type Resident struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	IsChild       bool   `json:"is_child"`
	ChildAge      *int   `json:"child_age"`
	PhoneCell     string `json:"phone_cell"`
	PhoneHome     string `json:"phone_home"`
	PhoneBusiness string `json:"phone_business"`
	MedicalNotes  string `json:"medical_notes"`
}

type EmergencyContact struct {
	ContactName   string `json:"contact_name"`
	Relationship  string `json:"relationship"`
	Address       string `json:"address"`
	PhoneCell     string `json:"phone_cell"`
	PhoneHome     string `json:"phone_home"`
	PhoneBusiness string `json:"phone_business"`
}

type Vehicle struct {
	MakeModel   string `json:"make_model"`
	Year        string `json:"year"`
	PlateNumber string `json:"plate_number"`
}

type Owner struct {
	OwnerName     string `json:"owner_name"`
	Address       string `json:"address"`
	PhoneCell     string `json:"phone_cell"`
	PhoneHome     string `json:"phone_home"`
	PhoneBusiness string `json:"phone_business"`
}

type SearchResult struct {
	SuiteID string `json:"suite_id"`
	Name    string `json:"name"`
}

func main() {
	seed := flag.Bool("seed", false, "populate the database with deterministic dummy data and exit")
	flag.Parse()
	databasePath := os.Getenv("CANYON_DB")
	if databasePath == "" {
		databasePath = "canyon-springs.db"
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := initializeDatabase(db); err != nil {
		log.Fatal(err)
	}

	tpl := template.Must(template.ParseFS(templateFS, "templates/index.html"))
	testTpl2 := template.Must(template.ParseFS(templateFS, "templates/index2.html"))
	testTpl := template.Must(template.ParseFS(templateFS, "templates/index3.html"))
	app := &App{db: db, tpl: tpl, testTpl2: testTpl2, testTpl: testTpl}
	if *seed {
		if err := seedDatabase(app); err != nil {
			log.Fatal(err)
		}
		log.Printf("seeded 132 dummy apartments in %s", databasePath)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/index2.html", app.handleTestIndex2)
	mux.HandleFunc("/index3.html", app.handleTestIndex)
	mux.HandleFunc("/api/search", app.handleSearch)
	mux.HandleFunc("/api/units", app.handleUnits)
	mux.HandleFunc("/export/spreadsheet", app.handleSpreadsheet)
	mux.HandleFunc("/suite/pdf/", app.handlePDF)
	mux.HandleFunc("/suite/", app.handleSuite)
	mux.HandleFunc("/suite/save", app.handleSave)

	address := os.Getenv("CANYON_ADDR")
	if address == "" {
		address = ":8081"
	}
	server := &http.Server{Addr: address, Handler: logging(mux)}
	log.Printf("Canyon Springs Residents listening on http://%s", address)
	log.Fatal(server.ListenAndServe())
}

func initializeDatabase(db *sql.DB) error {
	_, err := db.Exec(`PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS suites (
	suite_id TEXT PRIMARY KEY, date_updated TEXT, entercode TEXT,
	backup_keyset_num TEXT, owner_is_resident BOOLEAN NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS residents (
	id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE,
	first_name TEXT, last_name TEXT, is_child BOOLEAN NOT NULL DEFAULT 0, child_age INTEGER,
	phone_cell TEXT, phone_home TEXT, phone_business TEXT, medical_notes TEXT
);
CREATE TABLE IF NOT EXISTS emergency_contacts (
	id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE,
	contact_name TEXT, relationship TEXT, address TEXT, phone_cell TEXT, phone_home TEXT, phone_business TEXT, medical_notes TEXT
);
CREATE TABLE IF NOT EXISTS vehicles (
	id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE,
	make_model TEXT, year TEXT, plate_number TEXT
);
CREATE TABLE IF NOT EXISTS parking_spots (
	id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE, spot_number TEXT
);
CREATE TABLE IF NOT EXISTS lockers (
	id INTEGER PRIMARY KEY AUTOINCREMENT, suite_id TEXT NOT NULL REFERENCES suites(suite_id) ON DELETE CASCADE, locker_number TEXT
);
CREATE TABLE IF NOT EXISTS owners (
	suite_id TEXT PRIMARY KEY REFERENCES suites(suite_id) ON DELETE CASCADE, owner_name TEXT, address TEXT,
	phone_cell TEXT, phone_home TEXT, phone_business TEXT
);`)
	if err != nil {
		return err
	}
	var residentMedicalNotesColumn string
	err = db.QueryRow(`SELECT name FROM pragma_table_info('residents') WHERE name='medical_notes'`).Scan(&residentMedicalNotesColumn)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`ALTER TABLE residents ADD COLUMN medical_notes TEXT`)
	}
	return err
}

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := app.tpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) handleTestIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/index3.html" {
		http.NotFound(w, r)
		return
	}
	if err := app.testTpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) handleTestIndex2(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/index2.html" {
		http.NotFound(w, r)
		return
	}
	if err := app.testTpl2.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, []SearchResult{})
		return
	}
	like := "%" + query + "%"
	rows, err := app.db.QueryContext(r.Context(), `SELECT DISTINCT s.suite_id,
		COALESCE((SELECT group_concat(first_name || ' ' || last_name, ', ') FROM residents WHERE suite_id=s.suite_id), '')
		FROM suites s LEFT JOIN residents resident ON resident.suite_id=s.suite_id
		WHERE s.suite_id LIKE ? OR resident.first_name LIKE ? OR resident.last_name LIKE ?
		ORDER BY s.suite_id LIMIT 12`, like, like, like)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	results := []SearchResult{}
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.SuiteID, &result.Name); err == nil {
			results = append(results, result)
		}
	}
	writeJSON(w, results)
}

func (app *App) handleUnits(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `SELECT s.suite_id,
		COALESCE((SELECT group_concat(trim(first_name || ' ' || last_name), ', ')
			FROM residents WHERE suite_id=s.suite_id), '')
		FROM suites s
		ORDER BY CAST(s.suite_id AS INTEGER), s.suite_id`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.SuiteID, &result.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, results)
}

func (app *App) handleSpreadsheet(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `SELECT suite_id FROM suites ORDER BY suite_id`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	suites := []Suite{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		suite, err := app.loadSuite(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		suites = append(suites, suite)
	}

	maxResidents, maxVehicles, maxParking, maxLockers, maxContacts := 0, 0, 0, 0, 0
	for _, suite := range suites {
		if len(suite.Residents) > maxResidents {
			maxResidents = len(suite.Residents)
		}
		if len(suite.Vehicles) > maxVehicles {
			maxVehicles = len(suite.Vehicles)
		}
		if len(suite.ParkingSpots) > maxParking {
			maxParking = len(suite.ParkingSpots)
		}
		if len(suite.Lockers) > maxLockers {
			maxLockers = len(suite.Lockers)
		}
		if len(suite.EmergencyContacts) > maxContacts {
			maxContacts = len(suite.EmergencyContacts)
		}
	}

	header := []string{"Suite Number", "Date Updated", "Door Entercode", "Backup Key Set #", "Owner Lives In Unit", "Owner Name", "Owner Address", "Owner Phone # - Cell", "Owner Phone # - Home", "Owner Phone # - Business"}
	for i := 1; i <= maxResidents; i++ {
		header = append(header, fmt.Sprintf("Resident %d - First Name", i), fmt.Sprintf("Resident %d - Last Name", i), fmt.Sprintf("Resident %d - Child?", i), fmt.Sprintf("Resident %d - Age", i), fmt.Sprintf("Resident %d - Phone # - Cell", i), fmt.Sprintf("Resident %d - Phone # - Home", i), fmt.Sprintf("Resident %d - Phone # - Business", i), fmt.Sprintf("Resident %d - Medical Notes", i))
	}
	for i := 1; i <= maxVehicles; i++ {
		header = append(header, fmt.Sprintf("Vehicle %d - Make & Model", i), fmt.Sprintf("Vehicle %d - Year", i), fmt.Sprintf("Vehicle %d - License Plate", i))
	}
	for i := 1; i <= maxParking; i++ {
		header = append(header, fmt.Sprintf("Parking Spot %d", i))
	}
	for i := 1; i <= maxLockers; i++ {
		header = append(header, fmt.Sprintf("Locker %d", i))
	}
	for i := 1; i <= maxContacts; i++ {
		header = append(header, fmt.Sprintf("Emergency Contact %d - Name", i), fmt.Sprintf("Emergency Contact %d - Relationship", i), fmt.Sprintf("Emergency Contact %d - Address", i), fmt.Sprintf("Emergency Contact %d - Phone # - Cell", i), fmt.Sprintf("Emergency Contact %d - Phone # - Home", i), fmt.Sprintf("Emergency Contact %d - Phone # - Business", i))
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="canyon-springs-residents.csv"`)
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(header); err != nil {
		return
	}
	for _, suite := range suites {
		row := []string{suite.SuiteID, suite.DateUpdated, suite.EnterCode, suite.BackupKeysetNum, strconv.FormatBool(suite.OwnerIsResident), suite.Owner.OwnerName, suite.Owner.Address, suite.Owner.PhoneCell, suite.Owner.PhoneHome, suite.Owner.PhoneBusiness}
		for i := 0; i < maxResidents; i++ {
			if i < len(suite.Residents) {
				value := suite.Residents[i]
				age := ""
				if value.ChildAge != nil {
					age = strconv.Itoa(*value.ChildAge)
				}
				row = append(row, value.FirstName, value.LastName, strconv.FormatBool(value.IsChild), age, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.MedicalNotes)
			} else {
				row = append(row, "", "", "", "", "", "", "", "")
			}
		}
		for i := 0; i < maxVehicles; i++ {
			if i < len(suite.Vehicles) {
				value := suite.Vehicles[i]
				row = append(row, value.MakeModel, value.Year, value.PlateNumber)
			} else {
				row = append(row, "", "", "")
			}
		}
		for i := 0; i < maxParking; i++ {
			if i < len(suite.ParkingSpots) {
				row = append(row, suite.ParkingSpots[i])
			} else {
				row = append(row, "")
			}
		}
		for i := 0; i < maxLockers; i++ {
			if i < len(suite.Lockers) {
				row = append(row, suite.Lockers[i])
			} else {
				row = append(row, "")
			}
		}
		for i := 0; i < maxContacts; i++ {
			if i < len(suite.EmergencyContacts) {
				value := suite.EmergencyContacts[i]
				row = append(row, value.ContactName, value.Relationship, value.Address, value.PhoneCell, value.PhoneHome, value.PhoneBusiness)
			} else {
				row = append(row, "", "", "", "", "", "")
			}
		}
		if err := csvWriter.Write(row); err != nil {
			return
		}
	}
	csvWriter.Flush()
}

func (app *App) handlePDF(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/suite/pdf/"), "/")
	suite, err := app.loadSuite(r.Context(), id)
	if err != nil {
		http.Error(w, "suite not found", http.StatusNotFound)
		return
	}

	pdf := fpdf.New("L", "mm", "Letter", "")
	pdf.SetMargins(8, 7, 8)
	pdf.SetAutoPageBreak(false, 7)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, "CANYON SPRINGS RESIDENT RECORD", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 8)
	pdf.CellFormat(0, 5, fmt.Sprintf("Suite %s    Updated %s", suite.SuiteID, suite.DateUpdated), "", 1, "L", false, 0, "")

	section := func(title string) {
		pdf.SetFillColor(23, 48, 66)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(0, 5, title, "", 1, "L", true, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	field := func(label, value string, width float64) {
		pdf.SetFont("Arial", "B", 7)
		pdf.CellFormat(width, 4.5, label+":", "B", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(width, 4.5, value, "B", 0, "L", false, 0, "")
	}
	line := func() { pdf.Ln(5) }

	section("SUITE DETAILS")
	field("Door Entercode", suite.EnterCode, 58)
	field("Backup Key Set #", suite.BackupKeysetNum, 58)
	field("Owner Lives In Unit", strconv.FormatBool(suite.OwnerIsResident), 55)
	field("Owner", suite.Owner.OwnerName, 70)
	field("Owner Address", suite.Owner.Address, 125)
	line()
	field("Owner Phone # - Cell", suite.Owner.PhoneCell, 90)
	field("Owner Phone # - Home", suite.Owner.PhoneHome, 90)
	field("Owner Phone # - Business", suite.Owner.PhoneBusiness, 90)
	line()

	section("RESIDENTS")
	residentWidths := []float64{38, 38, 13, 12, 35, 35, 35, 78}
	residentHeaders := []string{"First Name", "Last Name", "Child", "Age", "Phone # - Cell", "Phone # - Home", "Phone # - Business", "Medical Notes"}
	pdf.SetFont("Arial", "B", 6)
	for i, header := range residentHeaders {
		pdf.CellFormat(residentWidths[i], 4, header, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(4)
	if len(suite.Residents) == 0 {
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(284, 4, "None listed", "B", 1, "L", false, 0, "")
	}
	for _, value := range suite.Residents {
		age := ""
		if value.ChildAge != nil {
			age = strconv.Itoa(*value.ChildAge)
		}
		values := []string{value.FirstName, value.LastName, strconv.FormatBool(value.IsChild), age, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.MedicalNotes}
		pdf.SetFont("Arial", "", 6)
		for i, text := range values {
			pdf.CellFormat(residentWidths[i], 4, text, "B", 0, "L", false, 0, "")
		}
		pdf.Ln(4)
	}

	pdf.Ln(2)
	section("VEHICLES / PARKING / LOCKERS")
	left := "Vehicles:"
	for i, value := range suite.Vehicles {
		left += fmt.Sprintf(" %d) %s / %s / %s;", i+1, value.MakeModel, value.Year, value.PlateNumber)
	}
	field("", left, 150)
	right := fmt.Sprintf("Parking: %s    Lockers: %s", strings.Join(suite.ParkingSpots, ", "), strings.Join(suite.Lockers, ", "))
	field("", right, 134)
	line()

	section("EMERGENCY CONTACTS")
	contactWidths := []float64{42, 35, 75, 42, 42, 48}
	contactHeaders := []string{"Name", "Relationship", "Address", "Phone # - Cell", "Phone # - Home", "Phone # - Business"}
	pdf.SetFont("Arial", "B", 6)
	for i, header := range contactHeaders {
		pdf.CellFormat(contactWidths[i], 4, header, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(4)
	if len(suite.EmergencyContacts) == 0 {
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(284, 4, "None listed", "B", 1, "L", false, 0, "")
	}
	for _, value := range suite.EmergencyContacts {
		values := []string{value.ContactName, value.Relationship, value.Address, value.PhoneCell, value.PhoneHome, value.PhoneBusiness}
		pdf.SetFont("Arial", "", 6)
		for i, text := range values {
			pdf.CellFormat(contactWidths[i], 4, text, "B", 0, "L", false, 0, "")
		}
		pdf.Ln(4)
	}

	pdf.SetY(195)
	pdf.SetFont("Arial", "I", 6)
	pdf.CellFormat(0, 4, "Canyon Springs Residents", "", 0, "R", false, 0, "")
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="suite-%s.pdf"`, suite.SuiteID))
	w.Write(output.Bytes())
}

func (app *App) handleSuite(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/suite/"), "/")
	suite, err := app.loadSuite(r.Context(), id)
	if err != nil {
		http.Error(w, "suite not found", http.StatusNotFound)
		return
	}
	if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("format") == "json" {
		writeJSON(w, suite)
		return
	}
	if err := app.tpl.Execute(w, suite); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (app *App) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var suite Suite
	if err := json.NewDecoder(r.Body).Decode(&suite); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	suite.SuiteID = strings.ToUpper(strings.TrimSpace(suite.SuiteID))
	if !suitePattern.MatchString(suite.SuiteID) {
		http.Error(w, "suite number must be 3-4 letters or numbers", 400)
		return
	}
	if err := app.saveSuite(r.Context(), suite); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	saved, err := app.loadSuite(r.Context(), suite.SuiteID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, saved)
}

func (app *App) loadSuite(ctx context.Context, id string) (Suite, error) {
	var suite Suite
	err := app.db.QueryRowContext(ctx, `SELECT suite_id, COALESCE(date_updated,''), COALESCE(entercode,''), COALESCE(backup_keyset_num,''), owner_is_resident FROM suites WHERE suite_id=?`, id).Scan(&suite.SuiteID, &suite.DateUpdated, &suite.EnterCode, &suite.BackupKeysetNum, &suite.OwnerIsResident)
	if err != nil {
		return suite, err
	}
	suite.Residents = []Resident{}
	suite.EmergencyContacts = []EmergencyContact{}
	suite.Vehicles = []Vehicle{}
	suite.ParkingSpots = []string{}
	suite.Lockers = []string{}
	rows, err := app.db.QueryContext(ctx, `SELECT first_name,last_name,is_child,child_age,phone_cell,phone_home,phone_business,medical_notes FROM residents WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value Resident
		if err := rows.Scan(&value.FirstName, &value.LastName, &value.IsChild, &value.ChildAge, &value.PhoneCell, &value.PhoneHome, &value.PhoneBusiness, &value.MedicalNotes); err != nil {
			rows.Close()
			return suite, err
		}
		suite.Residents = append(suite.Residents, value)
	}
	rows.Close()
	rows, err = app.db.QueryContext(ctx, `SELECT contact_name,relationship,address,phone_cell,phone_home,phone_business FROM emergency_contacts WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value EmergencyContact
		if err := rows.Scan(&value.ContactName, &value.Relationship, &value.Address, &value.PhoneCell, &value.PhoneHome, &value.PhoneBusiness); err != nil {
			rows.Close()
			return suite, err
		}
		suite.EmergencyContacts = append(suite.EmergencyContacts, value)
	}
	rows.Close()
	rows, err = app.db.QueryContext(ctx, `SELECT make_model,year,plate_number FROM vehicles WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value Vehicle
		if err := rows.Scan(&value.MakeModel, &value.Year, &value.PlateNumber); err != nil {
			rows.Close()
			return suite, err
		}
		suite.Vehicles = append(suite.Vehicles, value)
	}
	rows.Close()
	rows, err = app.db.QueryContext(ctx, `SELECT spot_number FROM parking_spots WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value string
		rows.Scan(&value)
		suite.ParkingSpots = append(suite.ParkingSpots, value)
	}
	rows.Close()
	rows, err = app.db.QueryContext(ctx, `SELECT locker_number FROM lockers WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value string
		rows.Scan(&value)
		suite.Lockers = append(suite.Lockers, value)
	}
	rows.Close()
	app.db.QueryRowContext(ctx, `SELECT owner_name,address,phone_cell,phone_home,phone_business FROM owners WHERE suite_id=?`, id).Scan(&suite.Owner.OwnerName, &suite.Owner.Address, &suite.Owner.PhoneCell, &suite.Owner.PhoneHome, &suite.Owner.PhoneBusiness)
	return suite, nil
}

func (app *App) saveSuite(ctx context.Context, suite Suite) error {
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO suites(suite_id,date_updated,entercode,backup_keyset_num,owner_is_resident) VALUES(?,?,?,?,?) ON CONFLICT(suite_id) DO UPDATE SET date_updated=excluded.date_updated,entercode=excluded.entercode,backup_keyset_num=excluded.backup_keyset_num,owner_is_resident=excluded.owner_is_resident`, suite.SuiteID, time.Now().Format(time.RFC3339), strings.TrimSpace(suite.EnterCode), strings.TrimSpace(suite.BackupKeysetNum), suite.OwnerIsResident)
	if err != nil {
		return err
	}
	for _, table := range []string{"residents", "emergency_contacts", "vehicles", "parking_spots", "lockers", "owners"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE suite_id=?", suite.SuiteID); err != nil {
			return err
		}
	}
	for _, value := range suite.Residents {
		_, err = tx.ExecContext(ctx, `INSERT INTO residents(suite_id,first_name,last_name,is_child,child_age,phone_cell,phone_home,phone_business,medical_notes) VALUES(?,?,?,?,?,?,?,?,?)`, suite.SuiteID, value.FirstName, value.LastName, value.IsChild, value.ChildAge, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.MedicalNotes)
		if err != nil {
			return err
		}
	}
	for _, value := range suite.EmergencyContacts {
		_, err = tx.ExecContext(ctx, `INSERT INTO emergency_contacts(suite_id,contact_name,relationship,address,phone_cell,phone_home,phone_business) VALUES(?,?,?,?,?,?,?)`, suite.SuiteID, value.ContactName, value.Relationship, value.Address, value.PhoneCell, value.PhoneHome, value.PhoneBusiness)
		if err != nil {
			return err
		}
	}
	for _, value := range suite.Vehicles {
		_, err = tx.ExecContext(ctx, `INSERT INTO vehicles(suite_id,make_model,year,plate_number) VALUES(?,?,?,?)`, suite.SuiteID, value.MakeModel, value.Year, value.PlateNumber)
		if err != nil {
			return err
		}
	}
	for _, value := range suite.ParkingSpots {
		if strings.TrimSpace(value) != "" {
			_, err = tx.ExecContext(ctx, `INSERT INTO parking_spots(suite_id,spot_number) VALUES(?,?)`, suite.SuiteID, value)
			if err != nil {
				return err
			}
		}
	}
	for _, value := range suite.Lockers {
		if strings.TrimSpace(value) != "" {
			_, err = tx.ExecContext(ctx, `INSERT INTO lockers(suite_id,locker_number) VALUES(?,?)`, suite.SuiteID, value)
			if err != nil {
				return err
			}
		}
	}
	if !suite.OwnerIsResident {
		_, err = tx.ExecContext(ctx, `INSERT INTO owners(suite_id,owner_name,address,phone_cell,phone_home,phone_business) VALUES(?,?,?,?,?,?)`, suite.SuiteID, suite.Owner.OwnerName, suite.Owner.Address, suite.Owner.PhoneCell, suite.Owner.PhoneHome, suite.Owner.PhoneBusiness)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func seedDatabase(app *App) error {
	firstNames := []string{"Alex", "Blair", "Casey", "Drew", "Elliot", "Frankie", "Gray", "Harper", "Indigo", "Jordan", "Kai", "Logan"}
	lastNames := []string{"Bennett", "Carter", "Diaz", "Ellis", "Foster", "Grant", "Hayes", "Irwin", "Jensen", "Kim", "Lane", "Morgan"}

	for floor := 1; floor <= 11; floor++ {
		for apartment := 1; apartment <= 12; apartment++ {
			suiteID := strconv.Itoa(floor*100 + apartment)
			index := (floor-1)*12 + apartment - 1
			ownerIsResident := index%4 != 0
			suite := Suite{
				SuiteID:         suiteID,
				DateUpdated:     time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				EnterCode:       fmt.Sprintf("%04d", 1000+index),
				BackupKeysetNum: fmt.Sprintf("K-%03d", index+1),
				OwnerIsResident: ownerIsResident,
				Residents: []Resident{{
					FirstName:    firstNames[index%len(firstNames)],
					LastName:     lastNames[(index/len(firstNames))%len(lastNames)],
					PhoneCell:    fmt.Sprintf("555-010-%04d", index+1),
					MedicalNotes: "Dummy data - do not contact",
				}},
				EmergencyContacts: []EmergencyContact{{
					ContactName:  fmt.Sprintf("Emergency Contact %d", index+1),
					Relationship: "Friend",
					Address:      "100 Example Street",
					PhoneCell:    fmt.Sprintf("555-020-%04d", index+1),
				}},
				Vehicles: []Vehicle{{
					MakeModel:   "Example Sedan",
					Year:        "2024",
					PlateNumber: fmt.Sprintf("DUMMY%03d", index+1),
				}},
				ParkingSpots: []string{fmt.Sprintf("P-%03d", index+1)},
				Lockers:      []string{fmt.Sprintf("L-%03d", index+1)},
			}
			if !ownerIsResident {
				suite.Owner = Owner{
					OwnerName: fmt.Sprintf("Owner %d", index+1),
					Address:   "100 Example Street",
					PhoneCell: fmt.Sprintf("555-030-%04d", index+1),
				}
			}
			if err := app.saveSuite(context.Background(), suite); err != nil {
				return fmt.Errorf("seed apartment %s: %w", suiteID, err)
			}
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
