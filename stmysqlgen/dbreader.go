// dbreader.go
package stmysqlgen

import (
	"fmt"

	"strings"
)

func genDBImport(packagename string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "package %s\nimport (\n\t%s\n\t%s\n\t%s\n\t%s\n\t%s\n)\n", packagename, `"fmt"`, `"strings"`, `"time"`, `"database/sql"`, `_ "github.com/go-sql-driver/mysql"`)
	builder.WriteString("var _ time.Time\n")
	return builder.String()
}

func genDBCreate() string {
	if len(Tables) == 0 {
		return ""
	}
	table := Tables[0]

	var builder strings.Builder
	fmt.Fprintf(&builder, "func DB_%s_Create(user, pwd, ip string, port int) (sql.Result, error) {\n", table.DB)
	builder.WriteString(`	var conurl strings.Builder
	fmt.Fprintf(&conurl, "%s:%s@tcp(%s:%d)/", user, pwd, ip, port)

	var err error
	db, err := sql.Open("mysql", conurl.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
`)
	fmt.Fprintf(&builder, "\tsqlcmd := \"CREATE DATABASE IF NOT EXISTS %s;\"\n", table.DB)
	builder.WriteString("\treturn db.Exec(sqlcmd)\n")
	builder.WriteString("}\n")
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
	builder.WriteString("\tCharset string\n")
	builder.WriteString("\tConnectTimeout uint32\n")
	builder.WriteString("\tReadTimeout uint32\n")
	builder.WriteString("\tWriteTimeout uint32\n")

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

func genDBConfig() string {
	if len(Tables) == 0 {
		return ""
	}
	table := Tables[0]

	dbname := "DB_" + table.DB
	var builder strings.Builder
	fmt.Fprintf(&builder, "func (db *%s) SetCharset(c string) {\n\t db.Charset = c\n}\n", dbname)
	fmt.Fprintf(&builder, "func (db *%s) SetConnectTimeout(t uint32) {\n\t db.ConnectTimeout = t\n}\n", dbname)
	fmt.Fprintf(&builder, "func (db *%s) SetReadTimeout(t uint32) {\n\t db.ReadTimeout = t\n}\n", dbname)
	fmt.Fprintf(&builder, "func (db *%s) SetWriteTimeout(t uint32) {\n\t db.WriteTimeout = t\n}\n", dbname)
	return builder.String()
}

func genDBConnect() string {
	if len(Tables) == 0 {
		return ""
	}
	table := Tables[0]

	dbname := "DB_" + table.DB
	var builder strings.Builder
	fmt.Fprintf(&builder, "func (db *%s) Open(user, pwd, addr string) error {\n", dbname)
	code := `	if db.ConnectTimeout == 0 {
		db.ConnectTimeout = 5
	}
	var conurl strings.Builder
	fmt.Fprintf(&conurl, "%s:%s@tcp(%s)/%s?parseTime=true&timeout=%ds", user, pwd, addr, "` + table.DB + `", db.ConnectTimeout)
	if db.Charset != "" {
		fmt.Fprintf(&conurl, "&charset=%s", db.Charset)
	}
	if db.ReadTimeout != 0 {
		fmt.Fprintf(&conurl, "&readTimeout=%ds", db.ReadTimeout)
	}
	if db.WriteTimeout != 0 {
		fmt.Fprintf(&conurl, "&writeTimeout=%ds", db.WriteTimeout)
	}
	
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
