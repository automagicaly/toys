package announcement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type AnnouncementType uint8

const (
	Default AnnouncementType = 0
	Report  AnnouncementType = 1 << (iota - 1)
	Confirmation
	TeacherNote
	ParentNote
	Urgent
	Event
	Reserved_0
	Reserved_1
)

type VersionNumber uint8

const (
	AlfaVersion VersionNumber = iota
)

type TenantID uint64
type UserID uint64
type ClassID uint64
type AnnouncementID uint64

type Announcement struct {
	ID        AnnouncementID
	Version   VersionNumber
	Type      AnnouncementType
	Timestamp time.Time
	Tenant    TenantID
	Parent    UserID
	Teacher   UserID
	Class     ClassID
	Message   string
	Metadata  map[string]any
}

type AnnouncementRepository struct {
	db *sql.DB
}

func NewAnnouncementRepository(db *sql.DB) *AnnouncementRepository {
	return &AnnouncementRepository{
		db: db,
	}
}

func (repo *AnnouncementRepository) Save(a *Announcement) error {
	if a.ID != Missing {
		return errors.New("Announcement already marked as saved")
	}

	query := `
		INSERT INTO announcements(version, type, timestamp, tenant_id, parent_id, teacher_id, class_id, message, metadata) VALUES (?,?,?,?,?,?,?,?,?);
	`
	serializedMetadata := ""
	var err error = nil
	if a.Metadata != nil {
		jsonData, err := json.Marshal(a.Metadata)
		DieOnError(err)
		serializedMetadata = string(jsonData)
	}
	result, err := repo.db.Exec(
		query,
		a.Version,
		a.Type,
		a.Timestamp.Unix(),
		a.Tenant,
		a.Parent,
		a.Teacher,
		a.Class,
		a.Message,
		serializedMetadata,
	)
	DieOnError(err)

	id, err := result.LastInsertId()
	DieOnError(err)
	a.ID = AnnouncementID(id)

	return err
}

func (repo *AnnouncementRepository) List(tenant TenantID, parent UserID, teacher UserID, class ClassID) (*[]Announcement, error) {
	query := `
		SELECT id 
		FROM announcements 
		WHERE 1=1 
			AND tenant_id = ? 
	`
	if tenant == Missing {
		return nil, errors.New("Tenant ID is missing")
	}

	if parent != Missing {
		query += " AND parent_id = ?"
	}

	if teacher != Missing {
		query += " AND teacher_id = ?"
	}

	if class != Missing {
		query += " AND class_id = ?"
	}
	println(query)

	rows, err := repo.db.Query(query, tenant, parent, teacher, class)
	DieOnError(err)

	if rows == nil {
		return nil, errors.New("Could not complete query to retrieve announcement list")
	}
	defer rows.Close()

	for rows.Next() {
		var a Announcement
		rows.Scan(&a.ID)
		println(a.ID)
	}

	return nil, nil
}

func (repo *AnnouncementRepository) create_annoucements_table() error {
	_, err := repo.db.Exec(`
		CREATE TABLE announcements (
			id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL,
			type INTEGER NOT NULL,
			timestamp INTEGER NOT NULL, 
			tenant_id INTERGER NOT NULL,
			parent_id INTERGER,
			teacher_id INTEGER,
			class_id INTEGER,
			message TEXT NOT NULL,
			metadata TEXT
		);
	`)

	return err
}

func (repo *AnnouncementRepository) Init() {
	exists, err := check_if_table_exists(repo.db, "announcements")
	DieOnError(err)
	if !exists {
		err = repo.create_annoucements_table()
		DieOnError(err)
	}
}

func check_if_table_exists(db *sql.DB, table_name string) (bool, error) {
	query := `
		SELECT
			name
		FROM 
			sqlite_master
		WHERE 1 = 1
			AND type = 'table'
			AND name = ?
	`

	rows, err := db.Query(query, table_name)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows == nil {
		return false, errors.New("DB returned a nil row set")
	}

	return rows.Next(), nil
}
