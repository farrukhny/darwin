package darwin

import (
	"bufio"
	"crypto/md5"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Status is a migration status value
type Status int

const (
	// Ignored means that the migrations was not applied to the database
	Ignored Status = iota
	// Applied means that the migrations was successfully applied to the database
	Applied
	// Pending means that the migrations is a new migration and it is waiting to be applied to the database
	Pending
	// Error means that the migration could not be applied to the database
	Error
)

func (s Status) String() string {
	switch s {
	case Ignored:
		return "IGNORED"
	case Applied:
		return "APPLIED"
	case Pending:
		return "PENDING"
	case Error:
		return "ERROR"
	default:
		return "INVALID"
	}
}

// A global mutex
var mutex = &sync.Mutex{}

// Migration represents a database migrations.
type Migration struct {
	Version     float64
	Description string
	Script      string
}

// Checksum calculate the Script md5
func (m Migration) Checksum() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(m.Script)))
}

// MigrationInfo is a struct used in the infoChan to inform clients about
// the migration being applied.
type MigrationInfo struct {
	Status    Status
	Error     error
	Migration Migration
}

// Darwin is a helper struct to access the Validate and migration functions
type Darwin struct {
	driver     Driver
	migrations []Migration
}

// Validate if the database migrations are applied and consistent
func (d Darwin) Validate() error {
	return Validate(d.driver, d.migrations)
}

// Migrate executes the missing migrations in database
func (d Darwin) Migrate() error {
	return Migrate(d.driver, d.migrations)
}

// Info returns the status of all migrations
func (d Darwin) Info() ([]MigrationInfo, error) {
	return Info(d.driver, d.migrations)
}

// New returns a new Darwin struct
func New(driver Driver, migrations []Migration) Darwin {
	return Darwin{
		driver:     driver,
		migrations: migrations,
	}
}

// ParseMigrationsDirFiles takes a embed file system and directory name with sql
// files that represents a text formatted set of migrations and parse them.
func ParseMigrationsDirFiles(fsys embed.FS, dirName string) ([]Migration, error) {
	var migrations []Migration

	files, err := fsys.ReadDir(dirName)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		readFile, err := fsys.ReadFile(dirName + "/" + file.Name())
		if err != nil {
			return nil, err
		}

		migs := ParseMigrations(string(readFile))
		migrations = append(migrations, migs...)
	}

	return migrations, nil
}

// ParseMigrations takes a string that represents a text formatted set
// of migrations and parse them for use.
func ParseMigrations(s string) []Migration {
	var migrations []Migration

	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Split(bufio.ScanLines)

	var m Migration
	var script string
	for scanner.Scan() {
		v := scanner.Text()
		switch {
		case strings.HasPrefix(strings.Replace(strings.ToLower(v), " ", "", 1), "--version:"):

			m.Script = script
			migrations = append(migrations, m)

			m = Migration{}
			script = ""

			f, err := strconv.ParseFloat(strings.TrimSpace(v[11:]), 64)
			if err != nil {
				fmt.Printf("Error: %v", err)
				return nil
			}
			m.Version = f

		case strings.HasPrefix(strings.Replace(strings.ToLower(v), " ", "", 1), "--description:"):
			m.Description = strings.TrimSpace(v[15:])

		default:
			script += v + "\n"
		}
	}

	m.Script = script
	migrations = append(migrations, m)

	return migrations[1:]
}

// DuplicateMigrationVersionError is used to report when the migration list has duplicated entries
type DuplicateMigrationVersionError struct {
	Version float64
}

func (d DuplicateMigrationVersionError) Error() string {
	return fmt.Sprintf("Multiple migrations have the version number %f.", d.Version)
}

// IllegalMigrationVersionError is used to report when the migration has an illegal Version number
type IllegalMigrationVersionError struct {
	Version float64
}

func (i IllegalMigrationVersionError) Error() string {
	return fmt.Sprintf("Illegal migration version number %f.", i.Version)
}

// RemovedMigrationError is used to report when a migration is removed from the list
type RemovedMigrationError struct {
	Version float64
}

func (r RemovedMigrationError) Error() string {
	return fmt.Sprintf("Migration %f was removed", r.Version)
}

// InvalidChecksumError is used to report when a migration was modified
type InvalidChecksumError struct {
	Version float64
}

func (i InvalidChecksumError) Error() string {
	return fmt.Sprintf("Invalid cheksum for migration %f", i.Version)
}

// Validate if the database migrations are applied and consistent
func Validate(d Driver, migrations []Migration) error {
	sort.Sort(byMigrationVersion(migrations))

	if version, invalid := isInvalidVersion(migrations); invalid {
		return IllegalMigrationVersionError{Version: version}
	}

	if version, dup := isDuplicated(migrations); dup {
		return DuplicateMigrationVersionError{Version: version}
	}

	applied, err := d.All()

	if err != nil {
		return err
	}

	if version, removed := wasRemovedMigration(applied, migrations); removed {
		return RemovedMigrationError{Version: version}
	}

	if version, invalid := isInvalidChecksumMigration(applied, migrations); invalid {
		return InvalidChecksumError{Version: version}
	}

	return nil
}

// Info returns the status of all migrations
func Info(d Driver, migrations []Migration) ([]MigrationInfo, error) {
	var info []MigrationInfo
	records, err := d.All()

	if err != nil {
		return info, err
	}

	sort.Sort(sort.Reverse(byMigrationRecordVersion(records)))

	for _, migration := range migrations {
		info = append(info, MigrationInfo{
			Status:    getStatus(records, migration),
			Error:     nil,
			Migration: migration,
		})
	}

	return info, nil
}

func getStatus(inDatabase []MigrationRecord, migration Migration) Status {
	last := inDatabase[0]

	// Check Pending
	if migration.Version > last.Version {
		return Pending
	}

	// Check Ignored
	found := false

	for _, record := range inDatabase {
		if record.Version == migration.Version {
			found = true
		}
	}

	if !found {
		return Ignored
	}

	return Applied
}

// Migrate executes the missing migrations in database.
func Migrate(d Driver, migrations []Migration) error {
	mutex.Lock()
	defer mutex.Unlock()

	err := d.Create()

	if err != nil {
		return err
	}

	err = Validate(d, migrations)

	if err != nil {
		return err
	}

	planned, err := planMigration(d, migrations)

	if err != nil {
		return err
	}

	for _, migration := range planned {
		dur, err := d.Exec(migration.Script)

		if err != nil {
			return err
		}

		err = d.Insert(MigrationRecord{
			Version:       migration.Version,
			Description:   migration.Description,
			Checksum:      migration.Checksum(),
			AppliedAt:     time.Now(),
			ExecutionTime: dur,
		})

		if err != nil {
			return err
		}

	}

	return nil
}

func wasRemovedMigration(applied []MigrationRecord, migrations []Migration) (float64, bool) {
	versionMap := map[float64]Migration{}

	for _, migration := range migrations {
		versionMap[migration.Version] = migration
	}

	for _, migration := range applied {
		if _, ok := versionMap[migration.Version]; !ok {
			return migration.Version, true
		}
	}

	return 0, false
}

func isInvalidChecksumMigration(applied []MigrationRecord, migrations []Migration) (float64, bool) {
	versionMap := map[float64]MigrationRecord{}

	for _, migration := range applied {
		versionMap[migration.Version] = migration
	}

	for _, migration := range migrations {
		if m, ok := versionMap[migration.Version]; ok {
			if m.Checksum != migration.Checksum() {
				return migration.Version, true
			}
		}
	}

	return 0, false
}

func isInvalidVersion(migrations []Migration) (float64, bool) {
	for _, migration := range migrations {
		version := migration.Version

		if version < 0 {
			return version, true
		}
	}

	return 0, false
}

func isDuplicated(migrations []Migration) (float64, bool) {
	unique := map[float64]Migration{}

	for _, migration := range migrations {
		_, exists := unique[migration.Version]

		if exists {
			return migration.Version, true
		}

		unique[migration.Version] = migration
	}

	return 0, false
}

func planMigration(d Driver, migrations []Migration) ([]Migration, error) {
	records, err := d.All()

	if err != nil {
		return []Migration{}, err
	}

	// Apply all migrations
	if len(records) == 0 {
		return migrations, nil
	}

	// Which migrations needs to be applied
	var planned []Migration

	// Make sure the order is correct
	// Do not trust the driver.
	sort.Sort(sort.Reverse(byMigrationRecordVersion(records)))
	last := records[0]

	// Apply all migrations that are greater than the last migration
	for _, migration := range migrations {
		if migration.Version > last.Version {
			planned = append(planned, migration)
		}
	}

	// Make sure the order is correct
	sort.Sort(byMigrationVersion(planned))

	return planned, nil
}

type byMigrationVersion []Migration

func (b byMigrationVersion) Len() int           { return len(b) }
func (b byMigrationVersion) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byMigrationVersion) Less(i, j int) bool { return b[i].Version < b[j].Version }
