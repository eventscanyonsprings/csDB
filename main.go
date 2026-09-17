package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"canyon-springs-residents/store"

	"github.com/go-pdf/fpdf"
	_ "modernc.org/sqlite"
)

//go:embed templates/login.html templates/units.html templates/edit.html templates/fire.html templates/admin.html
var templateFS embed.FS

var suitePattern = regexp.MustCompile(`^[A-Za-z0-9]{3,4}$`)

type App struct {
	db       *sql.DB
	loginTpl *template.Template
	unitsTpl *template.Template
	editTpl  *template.Template
	fireTpl  *template.Template
	adminTpl *template.Template
}

type Suite = store.Suite
type Resident = store.Resident
type EmergencyContact = store.EmergencyContact
type Vehicle = store.Vehicle
type Owner = store.Owner

type SearchResult struct {
	SuiteID   string `json:"suite_id"`
	Name      string `json:"name"`
	OwnerName string `json:"owner_name"`
	Plates    string `json:"plates"`
	Spots     string `json:"spots"`
	Lockers   string `json:"lockers"`
}

func main() {
	databasePath := os.Getenv("CANYON_DB")
	if databasePath == "" {
		databasePath = "canyon-springs.db"
	}
	db, err := store.Open(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	loginTpl := template.Must(template.ParseFS(templateFS, "templates/login.html"))
	unitsTpl := template.Must(template.ParseFS(templateFS, "templates/units.html"))
	editTpl := template.Must(template.ParseFS(templateFS, "templates/edit.html"))
	fireTpl := template.Must(template.ParseFS(templateFS, "templates/fire.html"))
	adminTpl := template.Must(template.ParseFS(templateFS, "templates/admin.html"))
	app := &App{db: db, loginTpl: loginTpl, unitsTpl: unitsTpl, editTpl: editTpl, fireTpl: fireTpl, adminTpl: adminTpl}
	mux := http.NewServeMux()
	mux.Handle("/login", loadUser(app)(http.HandlerFunc(app.handleLogin)))
	mux.Handle("/logout", loadUser(app)(http.HandlerFunc(app.handleLogout)))
	mux.Handle("/units", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleUnitsPage))))
	mux.Handle("/edit", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleEditPage))))
	mux.Handle("/report/fire", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleFirePage))))
	mux.Handle("/report/fire.json", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleFireJSON))))
	mux.Handle("/admin", loadUser(app)(requireAdmin(app)(http.HandlerFunc(app.handleAdminPage))))
	mux.Handle("/admin/users", loadUser(app)(requireAdmin(app)(http.HandlerFunc(app.handleAdminUsers))))
	mux.Handle("/admin/users/", loadUser(app)(requireAdmin(app)(http.HandlerFunc(app.handleAdminUserAction))))
	mux.Handle("/", loadUser(app)(http.HandlerFunc(app.handleIndex)))
	mux.Handle("/api/search", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleSearch))))
	mux.Handle("/api/units", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleUnits))))
	mux.Handle("/export/spreadsheet", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleSpreadsheet))))
	mux.Handle("/suite/pdf/", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handlePDF))))
	mux.Handle("/suite/", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleSuite))))
	mux.Handle("/suite/save", loadUser(app)(requireAuth(app)(http.HandlerFunc(app.handleSave))))

	address := os.Getenv("CANYON_ADDR")
	if address == "" {
		address = ":8081"
	}
	server := &http.Server{Addr: address, Handler: logging(mux)}
	log.Printf("Canyon Springs Residents listening on http://%s", address)
	log.Fatal(server.ListenAndServe())
}

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if userCount(app) == 0 {
		http.Redirect(w, r, "/units", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("handleLogin: method=%s", r.Method)
	if r.Method == http.MethodPost {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		log.Printf("handleLogin: username=%q", username)
		if username == "" || password == "" {
			app.loginTpl.Execute(w, map[string]any{"error": "Username and password are required."})
			return
		}
		count := userCount(app)
		log.Printf("handleLogin: userCount=%d", count)
		if count == 0 {
			app.loginTpl.Execute(w, map[string]any{"error": "No users configured. Use tools/create-admin to create one."})
			return
		}
		found, isAdm, err := store.FindUser(r.Context(), app.db, username, password)
		log.Printf("handleLogin: found=%v isAdmin=%v err=%v", found, isAdm, err)
		if err != nil || !found {
			app.loginTpl.Execute(w, map[string]any{"error": "Invalid username or password."})
			return
		}
		token, err := store.CreateSession(r.Context(), app.db, username)
		log.Printf("handleLogin: session created err=%v", err)
		if err != nil {
			app.loginTpl.Execute(w, map[string]any{"error": "Could not create session."})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 7 * 24 * 60 * 60})
		http.Redirect(w, r, "/units", http.StatusFound)
		return
	}
	app.loginTpl.Execute(w, nil)
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil && cookie.Value != "" {
		store.DeleteSession(r.Context(), app.db, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (app *App) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	users, err := store.ListUsers(r.Context(), app.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	app.adminTpl.Execute(w, map[string]any{"users": users, "currentUser": getUser(r)})
}

func (app *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	isAdmin := r.FormValue("is_admin") == "1"
	if username == "" || password == "" {
		http.Error(w, "Username and password are required.", http.StatusBadRequest)
		return
	}
	exists, _ := store.UserExists(r.Context(), app.db, username)
	if exists {
		http.Error(w, "User already exists.", http.StatusConflict)
		return
	}
	if err := store.CreateUser(r.Context(), app.db, username, password, isAdmin); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (app *App) handleAdminUserAction(w http.ResponseWriter, r *http.Request) {
	prefix := "/admin/users/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	username := strings.Trim(strings.TrimSuffix(rest, "/toggle-admin"), "/")
	if username == "" || strings.Contains(username, "/") {
		http.NotFound(w, r)
		return
	}
	currentUser := getUser(r)
	switch {
	case strings.HasSuffix(rest, "/toggle-admin"):
		if username == currentUser {
			http.Error(w, "You cannot change your own admin status.", http.StatusForbidden)
			return
		}
		admin, _ := store.IsAdmin(r.Context(), app.db, username)
		store.SetAdmin(r.Context(), app.db, username, !admin)
	case strings.HasSuffix(rest, "/password"):
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		password := r.FormValue("password")
		if password == "" {
			http.Error(w, "Password is required.", http.StatusBadRequest)
			return
		}
		store.UpdatePassword(r.Context(), app.db, username, password)
	case r.Method == http.MethodDelete || r.Method == http.MethodPost && strings.HasSuffix(rest, "/delete"):
		if username == currentUser {
			http.Error(w, "You cannot delete your own account.", http.StatusForbidden)
			return
		}
		store.DeleteUser(r.Context(), app.db, username)
	default:
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Accept") == "application/json" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (app *App) handleUnitsPage(w http.ResponseWriter, r *http.Request) {
	if err := app.unitsTpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) handleEditPage(w http.ResponseWriter, r *http.Request) {
	if err := app.editTpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) handleFirePage(w http.ResponseWriter, r *http.Request) {
	if err := app.fireTpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type fireResident struct {
	Name          string `json:"name"`
	PhoneCell     string `json:"phone_cell"`
	PhoneHome     string `json:"phone_home"`
	PhoneBusiness string `json:"phone_business"`
	FireNotes     string `json:"fire_notes"`
}

type fireUnit struct {
	SuiteID           string             `json:"suite_id"`
	Residents         []fireResident     `json:"residents"`
	EmergencyContacts []EmergencyContact `json:"emergency_contacts"`
}

func (app *App) handleFireJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.QueryContext(r.Context(), `SELECT suite_id FROM suites`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	report := []fireUnit{}
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
		unit := fireUnit{SuiteID: suite.SuiteID}
		for _, res := range suite.Residents {
			if strings.TrimSpace(res.FireNotes) == "" {
				continue
			}
			name := strings.TrimSpace(strings.TrimSpace(res.FirstName) + " " + strings.TrimSpace(res.LastName))
			if name == "" {
				name = "(name not given)"
			}
			unit.Residents = append(unit.Residents, fireResident{Name: name, PhoneCell: res.PhoneCell, PhoneHome: res.PhoneHome, PhoneBusiness: res.PhoneBusiness, FireNotes: res.FireNotes})
		}
		if len(unit.Residents) == 0 {
			continue
		}
		unit.EmergencyContacts = suite.EmergencyContacts
		report = append(report, unit)
	}
	sort.Slice(report, func(i, j int) bool { return suiteLess(report[i].SuiteID, report[j].SuiteID) })
	writeJSON(w, report)
}

func suiteLess(a, b string) bool {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr == nil && berr == nil {
		return ai < bi
	}
	return a < b
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
			FROM residents WHERE suite_id=s.suite_id), ''),
		COALESCE((SELECT owner_name FROM owners WHERE suite_id=s.suite_id), ''),
		COALESCE((SELECT group_concat(plate_number, ', ') FROM vehicles WHERE suite_id=s.suite_id AND COALESCE(plate_number,'')<>''), ''),
		COALESCE((SELECT group_concat(spot_number, ', ') FROM parking_spots WHERE suite_id=s.suite_id AND COALESCE(spot_number,'')<>''), ''),
		COALESCE((SELECT group_concat(locker_number, ', ') FROM lockers WHERE suite_id=s.suite_id AND COALESCE(locker_number,'')<>''), '')
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
		if err := rows.Scan(&result.SuiteID, &result.Name, &result.OwnerName, &result.Plates, &result.Spots, &result.Lockers); err != nil {
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
		header = append(header, fmt.Sprintf("Resident %d - First Name", i), fmt.Sprintf("Resident %d - Last Name", i), fmt.Sprintf("Resident %d - Child?", i), fmt.Sprintf("Resident %d - Age", i), fmt.Sprintf("Resident %d - Phone # - Cell", i), fmt.Sprintf("Resident %d - Phone # - Home", i), fmt.Sprintf("Resident %d - Phone # - Business", i), fmt.Sprintf("Resident %d - Medical Notes", i), fmt.Sprintf("Resident %d - Fire Dept Notes", i))
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
		header = append(header, fmt.Sprintf("Emergency Contact %d - Name", i), fmt.Sprintf("Emergency Contact %d - Relationship", i), fmt.Sprintf("Emergency Contact %d - Address", i), fmt.Sprintf("Emergency Contact %d - Phone # - Cell", i), fmt.Sprintf("Emergency Contact %d - Phone # - Home", i), fmt.Sprintf("Emergency Contact %d - Phone # - Business", i), fmt.Sprintf("Emergency Contact %d - Notes", i))
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
				row = append(row, value.FirstName, value.LastName, strconv.FormatBool(value.IsChild), age, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.MedicalNotes, value.FireNotes)
			} else {
				row = append(row, "", "", "", "", "", "", "", "", "")
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
				row = append(row, value.ContactName, value.Relationship, value.Address, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.Notes)
			} else {
				row = append(row, "", "", "", "", "", "", "")
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
		// width is the TOTAL width for this field (label + value).
		// Size the label to its text so short labels (e.g. "Owner:")
		// don't steal space from long values like the address.
		pdf.SetFont("Arial", "B", 7)
		need := pdf.GetStringWidth(label+":  ") + 2
		labelW := need
		if labelW > width*0.5 {
			labelW = width * 0.5
		}
		if labelW < 14 {
			labelW = 14
		}
		valueW := width - labelW
		pdf.CellFormat(labelW, 4.5, label+":", "B", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(valueW, 4.5, value, "B", 0, "L", false, 0, "")
	}
	// Landscape Letter usable width = 279.4 - 8 - 8 = ~263.4mm.
	// Keep every row at or under that so nothing runs off the page.
	const pageW = 263.0
	line := func() { pdf.Ln(5) }
	yesNo := func(b bool) string {
		if b {
			return "Yes"
		}
		return "No"
	}

	section("SUITE DETAILS")
	field("Door Entercode", suite.EnterCode, 58)
	field("Backup Key Set #", suite.BackupKeysetNum, 58)
	field("Owner Lives In Unit", yesNo(suite.OwnerIsResident), 55)
	line()
	ownerName := strings.TrimSpace(suite.Owner.OwnerName)
	ownerAddr := strings.TrimSpace(suite.Owner.Address)
	ownerCell := strings.TrimSpace(suite.Owner.PhoneCell)
	ownerHome := strings.TrimSpace(suite.Owner.PhoneHome)
	ownerBiz := strings.TrimSpace(suite.Owner.PhoneBusiness)
	if suite.OwnerIsResident && ownerName == "" && ownerAddr == "" && ownerCell == "" && ownerHome == "" && ownerBiz == "" {
		// No separate owner row is stored when the owner lives in the
		// unit (see store/save.go) — show the first resident so the
		// printout is not blank.
		if len(suite.Residents) > 0 {
			ownerName = strings.TrimSpace(strings.TrimSpace(suite.Residents[0].FirstName) + " " + strings.TrimSpace(suite.Residents[0].LastName))
			if ownerName == "" {
				ownerName = "Same as resident"
			} else {
				ownerName += " (resident)"
			}
			ownerAddr = "Same as unit"
			ownerCell = suite.Residents[0].PhoneCell
			ownerHome = suite.Residents[0].PhoneHome
			ownerBiz = suite.Residents[0].PhoneBusiness
		} else {
			ownerName = "Same as resident"
			ownerAddr = "Same as unit"
		}
	}
	// Owner name and address each get the full row so long
	// addresses are never clipped off the right edge.
	field("Owner", ownerName, pageW)
	line()
	field("Owner Address", ownerAddr, pageW)
	line()
	phoneW := pageW / 3
	field("Owner Phone # - Cell", ownerCell, phoneW)
	field("Owner Phone # - Home", ownerHome, phoneW)
	field("Owner Phone # - Business", ownerBiz, phoneW)
	line()

	section("RESIDENTS")
	// Weights are scaled to exactly fill the printable width so the
	// underlines neither fall short nor run off the page.
	residentWeights := []float64{32, 32, 12, 10, 30, 30, 30, 54, 54}
	residentWidths := scaleWidths(residentWeights, pageW)
	residentHeaders := []string{"First Name", "Last Name", "Child", "Age", "Phone # - Cell", "Phone # - Home", "Phone # - Business", "Medical Notes", "Fire Dept Notes"}
	pdf.SetFont("Arial", "B", 6)
	for i, header := range residentHeaders {
		pdf.CellFormat(residentWidths[i], 4, header, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(4)
	if len(suite.Residents) == 0 {
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(pageW, 4, "None listed", "B", 1, "L", false, 0, "")
	}
	for _, value := range suite.Residents {
		age := ""
		if value.ChildAge != nil {
			age = strconv.Itoa(*value.ChildAge)
		}
		values := []string{value.FirstName, value.LastName, yesNo(value.IsChild), age, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.MedicalNotes, value.FireNotes}
		pdf.SetFont("Arial", "", 6)
		for i, text := range values {
			pdf.CellFormat(residentWidths[i], 4, text, "B", 0, "L", false, 0, "")
		}
		pdf.Ln(4)
	}

	pdf.Ln(2)
	section("VEHICLES / PARKING / LOCKERS")
	vehicleParts := make([]string, 0, len(suite.Vehicles))
	for i, value := range suite.Vehicles {
		vehicleParts = append(vehicleParts, fmt.Sprintf("%d) %s / %s / %s", i+1, value.MakeModel, value.Year, value.PlateNumber))
	}
	if len(vehicleParts) == 0 {
		vehicleParts = []string{"None listed"}
	}
	parkingText := strings.Join(suite.ParkingSpots, ", ")
	if parkingText == "" {
		parkingText = "—"
	}
	lockersText := strings.Join(suite.Lockers, ", ")
	if lockersText == "" {
		lockersText = "—"
	}
	// First line: Vehicles label + 1st vehicle + Parking + Lockers.
	// Subsequent vehicles stack below, indented to align under the 1st.
	pdf.SetFont("Arial", "B", 7)
	pdf.CellFormat(18, 4.5, "Vehicles:", "B", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(109, 4.5, vehicleParts[0], "B", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 7)
	pdf.CellFormat(15, 4.5, "Parking:", "B", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(55, 4.5, parkingText, "B", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 7)
	pdf.CellFormat(15, 4.5, "Lockers:", "B", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(51, 4.5, lockersText, "B", 0, "L", false, 0, "")
	pdf.Ln(4.5)
	for _, text := range vehicleParts[1:] {
		pdf.CellFormat(18, 4.5, "", "B", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(109, 4.5, text, "B", 0, "L", false, 0, "")
		pdf.CellFormat(136, 4.5, "", "B", 0, "L", false, 0, "")
		pdf.Ln(4.5)
	}
	pdf.Ln(0.5)

	section("EMERGENCY CONTACTS")
	contactWeights := []float64{38, 30, 58, 34, 34, 34, 56}
	contactWidths := scaleWidths(contactWeights, pageW)
	contactHeaders := []string{"Name", "Relationship", "Address", "Phone # - Cell", "Phone # - Home", "Phone # - Business", "Notes"}
	pdf.SetFont("Arial", "B", 6)
	for i, header := range contactHeaders {
		pdf.CellFormat(contactWidths[i], 4, header, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(4)
	if len(suite.EmergencyContacts) == 0 {
		pdf.SetFont("Arial", "", 7)
		pdf.CellFormat(pageW, 4, "None listed", "B", 1, "L", false, 0, "")
	}
	for _, value := range suite.EmergencyContacts {
		values := []string{value.ContactName, value.Relationship, value.Address, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.Notes}
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
	if err := app.editTpl.Execute(w, suite); err != nil {
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
	if !suite.OwnerIsResident && strings.TrimSpace(suite.Owner.OwnerName) == "" {
		http.Error(w, "Owner name is required when the owner does not live in the unit", 400)
		return
	}
	if !suite.OwnerIsResident && strings.TrimSpace(suite.Owner.PhoneCell) == "" && strings.TrimSpace(suite.Owner.PhoneHome) == "" && strings.TrimSpace(suite.Owner.PhoneBusiness) == "" {
		http.Error(w, "At least one owner phone number is required when the owner does not live in the unit", 400)
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
	return store.LoadSuite(ctx, app.db, id)
}

func (app *App) saveSuite(ctx context.Context, suite Suite) error {
	return store.SaveSuite(ctx, app.db, suite)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

// scaleWidths scales relative column weights so they sum to exactly
// total, fixing rounding drift on the last column. This keeps table
// underlines flush with the printable width: no short lines, no
// overflow off the page.
func scaleWidths(weights []float64, total float64) []float64 {
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if sum <= 0 {
		return weights
	}
	out := make([]float64, len(weights))
	acc := 0.0
	for i, w := range weights {
		if i == len(weights)-1 {
			out[i] = total - acc
		} else {
			out[i] = w / sum * total
			acc += out[i]
		}
	}
	return out
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
