# Darwin

Database schema evolution library for Go

# Example

```go
package main

import (
	"database/sql"
	"log"

	"github.com/farrukhny/darwin"
	_ "github.com/go-sql-driver/mysql"
)

var schema = `-- Version: 1.0
-- Description: Create table products
CREATE TABLE user (
	user_id   UUID,
	name         TEXT,
	email         TEXT,
	password     TEXT,
	date_created TIMESTAMP,
	date_updated TIMESTAMP,

	PRIMARY KEY (user_id)
);`

func main() {
	database, err := sql.Open("mysql", "root:@/darwin")

	if err != nil {
		log.Fatal(err)
	}

	driver := darwin.NewGenericDriver(database, darwin.MySQLDialect{})

	d := darwin.New(driver, darwin.ParseMigrations(schema))
	err = d.Migrate()

	if err != nil {
		log.Println(err)
	}
}
```

`func ParseMigrationsDirFiles(fsys embed.FS, dirName string) ([]Migration, error)` can be used to parse SQL files from embed.FS

# LICENSE

The MIT License (MIT)

Copyright (c) 2016 Claudemiro

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
