package darwin

// MySQLDialect a Dialect configured for MySQL
type MySQLDialect struct{}

// CreateTableSQL returns the SQL to create the schema table
func (m MySQLDialect) CreateTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS db_version
                (
                    id             INT          auto_increment,
                    version        FLOAT        NOT NULL,
                    description    VARCHAR(255) NOT NULL,
                    checksum       VARCHAR(32)  NOT NULL,
                    applied_at     INT          NOT NULL,
                    execution_time FLOAT        NOT NULL,
                    UNIQUE         (version),
                    PRIMARY KEY    (id)
                ) ENGINE=InnoDB CHARACTER SET=utf8;`
}

// InsertSQL returns the SQL to insert a new migration in the schema table
func (m MySQLDialect) InsertSQL() string {
	return `INSERT INTO db_version
                (
                    version,
                    description,
                    checksum,
                    applied_at,
                    execution_time
                )
            VALUES (?, ?, ?, ?, ?);`
}

// AllSQL returns a SQL to get all entries in the table
func (m MySQLDialect) AllSQL() string {
	return `SELECT 
                version,
                description,
                checksum,
                applied_at,
                execution_time
            FROM 
                db_version
            ORDER BY version ASC;`
}
