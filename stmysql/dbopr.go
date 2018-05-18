package stmysql

import (
	"database/sql"
	"strconv"
	"strings"
)

func GetTableColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query("select * from " + table + " limit 1")
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return rows.Columns()
}

func newTable(tcfg *dbTable) []string {

	newstr := "CREATE TABLE " + tcfg.name + "(\n"
	isfirst := true
	for _, v := range tcfg.columns {
		if !v.export {
			continue
		}
		if !isfirst {
			newstr += ",\n"
		}
		newstr += v.name + " " + v.typ + " NOT NULL"
		if v.autoinc {
			newstr += " AUTO_INCREMENT"
		}
		isfirst = false
	}
	if tcfg.primarykey != "" {
		newstr += ",\nPRIMARY KEY " + tcfg.primarykey
	}
	if len(tcfg.index) > 0 {
		for _, v := range tcfg.index {
			newstr += ",\nINDEX" + v
		}
	}
	newstr += "\n) ENGINE=" + tcfg.engine
	if tcfg.autoincc != "" {
		newstr += " AUTO_INCREMENT=" + strconv.Itoa(int(tcfg.autoinci))
	}
	newstr += " DEFAULT CHARSET=" + tcfg.charset + ";"
	return []string{newstr}
}

/*
Oracle:
ALTER TABLE table_name DROP (column_name1, column_name2);

MS SQL:
ALTER TABLE table_name DROP COLUMN column_name1, column_name2

MySql:
ALTER TABLE table_name DROP column_name1, DROP column_name2;

Postgre SQL
ALTER TABLE table_name DROP COLUMN column_name1, DROP COLUMN column_name2;
*/
func checkTable(db *sql.DB, tcfg *dbTable) ([]string, error) {
	columns, err := GetTableColumns(db, tcfg.name)
	if err != nil {
		errstr := err.Error()
		if !strings.Contains(errstr, "doesn't exist") {
			return nil, err
		} else { //create new table
			if !tcfg.export {
				return nil, nil
			}
			return newTable(tcfg), nil
		}
	} else if !tcfg.export { //drop table
		newstr := "DROP TABLE " + tcfg.name
		return []string{newstr}, nil
	} else { //update table
		add := make([]dbColumn, 0, len(columns))
		for _, v1 := range tcfg.columns {
			if !v1.export {
				continue
			}
			isfind := false
			for _, v2 := range columns {
				if v1.name == v2 {
					isfind = true
					break
				}
			}
			if !isfind {
				add = append(add, v1)
			}
		}
		del := make([]string, 0, len(columns))
		for _, v1 := range columns {
			isfind := false
			for _, v2 := range tcfg.columns {
				if !v2.export {
					continue
				}
				if v1 == v2.name {
					isfind = true
					break
				}
			}
			if !isfind {
				del = append(del, v1)
			}
		}

		var restr []string
		if len(add) > 0 {
			newstr := "ALTER TABLE " + tcfg.name
			isfirst := true
			for _, v := range add {
				if !isfirst {
					newstr += ",\n"
				}
				newstr += " ADD " + v.name + " " + v.typ
				isfirst = false
			}
			newstr += "\n;\n"
			restr = make([]string, 0, 2)
			restr = append(restr, newstr)
		}
		if len(del) > 0 {
			newstr := "ALTER TABLE " + tcfg.name + " DROP "
			isfirst := true
			for _, v := range del {
				if !isfirst {
					newstr += ",DROP "
				}
				newstr += v
				isfirst = false
			}
			newstr += " ;\n"
			restr = append(restr, newstr)
		}
		return restr, nil
	}
}

func CheckTable(db *sql.DB, table interface{}) ([]string, error) {
	t, e := getTableConfig(table)
	if e != nil {
		return nil, e
	}
	return checkTable(db, t)
}

func UpdateTable(db *sql.DB, table interface{}) ([]string, error) {
	t, e := getTableConfig(table)
	if e != nil {
		return nil, e
	}

	sqlstrs, err := checkTable(db, t)
	if err != nil {
		return nil, err
	}
	cmds := make([]string, 0, len(sqlstrs))
	for _, v := range sqlstrs {
		cmds = append(cmds, v)
		_, err = db.Exec(v)
		if err != nil {
			return cmds, err
		}
	}
	return cmds, nil
}

//db, err := sql.Open("mysql", "name:pwd@tcp(127.0.0.1:3306)/")
func CreateDatabase(db *sql.DB, name string) error {
	sqlstr := "CREATE DATABASE IF NOT EXISTS " + name
	_, err := db.Exec(sqlstr)
	if err != nil {
		return err
	}
	sqlstr = "USE " + name
	_, err = db.Exec(sqlstr)
	if err != nil {
		return err
	}
	return nil
}
