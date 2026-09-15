package store

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
	FireNotes     string `json:"fire_notes"`
}

type EmergencyContact struct {
	ContactName   string `json:"contact_name"`
	Relationship  string `json:"relationship"`
	Address       string `json:"address"`
	PhoneCell     string `json:"phone_cell"`
	PhoneHome     string `json:"phone_home"`
	PhoneBusiness string `json:"phone_business"`
	Notes         string `json:"notes"`
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
