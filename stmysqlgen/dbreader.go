// dbreader.go
package main

import (
	"fmt"

	"strings"
)

func genDBImport(packagename string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "package %s\nimport (\n\t%s\n\t%s\n\t%s\n\t%s\n)\n", packagename, `"fmt"`, `"strings"`, `"database/sql"`, `_ "github.com/go-sql-driver/mysql"`)
	return builder.String()
}
func genDBStruct() string {
	if len(Tables) == 0 {
		return ""
	}
	table := Tables[0]

	var builder strings.Builder
	fmt.Fprintf(&builder, "type DB_%s struct{\n", table.DB)
	builder.WriteString("\tDB *sql.DB\n")
	for _, t := range Tables {
		builder.WriteString("\tT_")
		builder.WriteString(t.Name)
		builder.WriteString(" ")
		builder.WriteString("*T_" + t.DB + "_" + t.Name)
		builder.WriteString("\n")
	}
	builder.WriteString("}\n")
	return builder.String()
}

func genDBConnect() string {
	if len(Tables) == 0 {
		return ""
	}
	table := Tables[0]

	dbname := "DB_" + table.DB
	var builder strings.Builder
	fmt.Fprintf(&builder, "func (db *%s) Open(user, pwd, ip string, port int) error {\n", dbname)
	code := `	var conurl strings.Builder
	fmt.Fprintf(&conurl, "%s:%s@tcp(%s:%d)/%s", user, pwd, ip, port, "` + table.DB + `")
	var err error
	db.DB, err = sql.Open("mysql", conurl.String())
	if err != nil {
		return err
	}
`
	builder.WriteString(code)
	for _, t := range Tables {
		builder.WriteString("\tdb.T_")
		builder.WriteString(t.Name)
		builder.WriteString(" = ")
		builder.WriteString("&T_" + t.DB + "_" + t.Name)
		builder.WriteString("{DB:db.DB}\n")
	}
	builder.WriteString("\treturn nil\n}\n")

	fmt.Fprintf(&builder, "func (db *%s) Close() {\n\t db.DB.Close()\n}\n", dbname)

	return builder.String()
}
