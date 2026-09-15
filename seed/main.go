package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"canyon-springs-residents/store"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "dummy.db", "sqlite file to create and seed (never use the real database)")
	force := flag.Bool("force", false, "overwrite existing suites in the target database")
	flag.Parse()
	if strings.TrimSpace(*dbPath) == "" {
		log.Fatal("-db is required")
	}
	lower := strings.ToLower(filepath.Base(*dbPath))
	if strings.Contains(lower, "canyon-springs") {
		log.Fatal("refusing to seed the real database file (canyon-springs.db); use -db dummy.db")
	}
	if _, err := os.Stat(*dbPath); err == nil && !*force {
		log.Fatalf("target %s already exists; delete it or re-run with -force", *dbPath)
	}
	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if !*force {
		n, err := store.SuiteCount(ctx, db)
		if err != nil {
			log.Fatal(err)
		}
		if n > 0 {
			log.Fatalf("target %s already has %d suites; re-run with -force to overwrite", *dbPath, n)
		}
	}
	if err := seedDatabase(ctx, db); err != nil {
		log.Fatal(err)
	}
	log.Printf("seeded 132 dummy apartments in %s", *dbPath)
}

func seedDatabase(ctx context.Context, db *sql.DB) error {
	firstNames := []string{"Alex", "Blair", "Casey", "Drew", "Elliot", "Frankie", "Gray", "Harper", "Indigo", "Jordan", "Kai", "Logan"}
	lastNames := []string{"Bennett", "Carter", "Diaz", "Ellis", "Foster", "Grant", "Hayes", "Irwin", "Jensen", "Kim", "Lane", "Morgan"}

	for floor := 1; floor <= 11; floor++ {
		for apartment := 1; apartment <= 12; apartment++ {
			suiteID := strconv.Itoa(floor*100 + apartment)
			index := (floor-1)*12 + apartment - 1
			ownerIsResident := index%4 != 0
			suite := store.Suite{
				SuiteID:         suiteID,
				DateUpdated:     time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				EnterCode:       fmt.Sprintf("%04d", 1000+index),
				BackupKeysetNum: fmt.Sprintf("K-%03d", index+1),
				OwnerIsResident: ownerIsResident,
				Residents: []store.Resident{{
					FirstName:    firstNames[index%len(firstNames)],
					LastName:     lastNames[(index/len(firstNames))%len(lastNames)],
					PhoneCell:    fmt.Sprintf("555-010-%04d", index+1),
					MedicalNotes: "Dummy data - do not contact",
					FireNotes:    "Dummy fire dept note",
				}},
				EmergencyContacts: []store.EmergencyContact{{
					ContactName:  fmt.Sprintf("Emergency Contact %d", index+1),
					Relationship: "Friend",
					Address:      "100 Example Street",
					PhoneCell:    fmt.Sprintf("555-020-%04d", index+1),
					Notes:        fmt.Sprintf("Dummy note for emergency contact %d", index+1),
				}},
				Vehicles: []store.Vehicle{{
					MakeModel:   "Example Sedan",
					Year:        "2024",
					PlateNumber: fmt.Sprintf("DUMMY%03d", index+1),
				}},
				ParkingSpots: []string{fmt.Sprintf("P-%03d", index+1)},
				Lockers:      []string{fmt.Sprintf("L-%03d", index+1)},
			}
			if !ownerIsResident {
				suite.Owner = store.Owner{
					OwnerName:     fmt.Sprintf("Owner %d", index+1),
					Address:       "100 Example Street",
					PhoneCell:     fmt.Sprintf("555-030-%04d", index+1),
					PhoneHome:     fmt.Sprintf("555-031-%04d", index+1),
					PhoneBusiness: fmt.Sprintf("555-032-%04d", index+1),
				}
			}
			if err := store.SaveSuite(ctx, db, suite); err != nil {
				return fmt.Errorf("seed apartment %s: %w", suiteID, err)
			}
		}
	}
	return nil
}
