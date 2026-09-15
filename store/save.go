package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func SaveSuite(ctx context.Context, db *sql.DB, suite Suite) error {
	tx, err := db.BeginTx(ctx, nil)
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
		_, err = tx.ExecContext(ctx, `INSERT INTO residents(suite_id,first_name,last_name,is_child,child_age,phone_cell,phone_home,phone_business,medical_notes,fire_notes) VALUES(?,?,?,?,?,?,?,?,?,?)`, suite.SuiteID, value.FirstName, value.LastName, value.IsChild, value.ChildAge, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.MedicalNotes, value.FireNotes)
		if err != nil {
			return err
		}
	}
	for _, value := range suite.EmergencyContacts {
		_, err = tx.ExecContext(ctx, `INSERT INTO emergency_contacts(suite_id,contact_name,relationship,address,phone_cell,phone_home,phone_business,notes) VALUES(?,?,?,?,?,?,?,?)`, suite.SuiteID, value.ContactName, value.Relationship, value.Address, value.PhoneCell, value.PhoneHome, value.PhoneBusiness, value.Notes)
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
