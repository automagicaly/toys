package repos

import (
	"database/sql"
	"encoding/json"
	"errors"

	aux "lorde.tech/toys/agenda"
	"lorde.tech/toys/agenda/entities"
)

type MessageRepository struct {
	db *sql.DB
}

func New(db *sql.DB) *MessageRepository {
	return &MessageRepository{
		db: db,
	}
}

func (repo *MessageRepository) Save(msg *entities.Message) error {
	if msg.ID != entities.Missing {
		return errors.New("Announcement already marked as saved")
	}

	query := `
		INSERT INTO announcements(version, type, timestamp, tenant_id, parent_id, teacher_id, class_id, message, metadata) VALUES (?,?,?,?,?,?,?,?,?);
	`
	serializedMetadata := ""
	var err error = nil
	if msg.Metadata != nil {
		jsonData, err := json.Marshal(msg.Metadata)
		aux.DieOnError(err)
		serializedMetadata = string(jsonData)
	}
	result, err := repo.db.Exec(
		query,
		msg.Version,
		msg.Type,
		msg.Timestamp.Unix(),
		msg.Tenant,
		msg.Parent,
		msg.Teacher,
		msg.Class,
		msg.Content,
		serializedMetadata,
	)
	aux.DieOnError(err)

	id, err := result.LastInsertId()
	aux.DieOnError(err)
	msg.ID = entities.MessageID(id)

	return err
}

func (repo *MessageRepository) List(tenant TenantID, parent UserID, teacher UserID, class ClassID) (*[]Message, error) {
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
		var a Message
		rows.Scan(&a.ID)
		println(a.ID)
	}

	return nil, nil
}

func (repo *MessageRepository) create_annoucements_table() error {
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

func (repo *MessageRepository) Init() {
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
