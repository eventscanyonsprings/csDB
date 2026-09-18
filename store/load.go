package store

import (
	"context"
	"database/sql"
)

func LoadSuite(ctx context.Context, db *sql.DB, id string) (Suite, error) {
	var suite Suite
	err := db.QueryRowContext(ctx, `SELECT suite_id, COALESCE(date_updated,''), COALESCE(entercode,''), COALESCE(backup_keyset_num,''), owner_is_resident FROM suites WHERE suite_id=?`, id).Scan(&suite.SuiteID, &suite.DateUpdated, &suite.EnterCode, &suite.BackupKeysetNum, &suite.OwnerIsResident)
	if err != nil {
		return suite, err
	}
	suite.Residents = []Resident{}
	suite.EmergencyContacts = []EmergencyContact{}
	suite.Vehicles = []Vehicle{}
	suite.ParkingSpots = []string{}
	suite.Lockers = []string{}
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(first_name,''),COALESCE(last_name,''),is_child,child_age,COALESCE(phone_cell,''),COALESCE(phone_home,''),COALESCE(phone_business,''),COALESCE(medical_notes,''),COALESCE(fire_notes,'') FROM residents WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value Resident
		if err := rows.Scan(&value.FirstName, &value.LastName, &value.IsChild, &value.ChildAge, &value.PhoneCell, &value.PhoneHome, &value.PhoneBusiness, &value.MedicalNotes, &value.FireNotes); err != nil {
			rows.Close()
			return suite, err
		}
		suite.Residents = append(suite.Residents, value)
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `SELECT contact_name,relationship,address,phone_cell,phone_home,phone_business,COALESCE(notes,'') FROM emergency_contacts WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value EmergencyContact
		if err := rows.Scan(&value.ContactName, &value.Relationship, &value.Address, &value.PhoneCell, &value.PhoneHome, &value.PhoneBusiness, &value.Notes); err != nil {
			rows.Close()
			return suite, err
		}
		suite.EmergencyContacts = append(suite.EmergencyContacts, value)
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `SELECT make_model,year,plate_number FROM vehicles WHERE suite_id=? ORDER BY id`, id)
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
	rows, err = db.QueryContext(ctx, `SELECT spot_number FROM parking_spots WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value string
		rows.Scan(&value)
		suite.ParkingSpots = append(suite.ParkingSpots, value)
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `SELECT locker_number FROM lockers WHERE suite_id=? ORDER BY id`, id)
	if err != nil {
		return suite, err
	}
	for rows.Next() {
		var value string
		rows.Scan(&value)
		suite.Lockers = append(suite.Lockers, value)
	}
	rows.Close()
	db.QueryRowContext(ctx, `SELECT owner_name,address,phone_cell,phone_home,phone_business FROM owners WHERE suite_id=?`, id).Scan(&suite.Owner.OwnerName, &suite.Owner.Address, &suite.Owner.PhoneCell, &suite.Owner.PhoneHome, &suite.Owner.PhoneBusiness)
	return suite, nil
}
