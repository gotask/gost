// gensql.go
package stmysqlgen

import (
	"fmt"
	"os"
	"path"
	"strings"
)

func GenGOFile(user, pwd, ip string, port int, dbname, packagename string) error {
	err := readDB(user, pwd, ip, port, dbname)
	if err != nil {
		return err
	}
	if len(Tables) == 0 {
		return nil
	}
	table := Tables[0]

	f, e := FileCreate("db_" + table.DB + ".go")
	if e != nil {
		return e
	}
	defer f.Close()

	f.WriteString(genDBImport(packagename))
	f.WriteString(genDBStruct())
	f.WriteString(genDBConfig())
	f.WriteString(genDBConnect())

	for _, v := range Tables {
		f.WriteString(genTableSqlStruct(v))
		f.WriteString(genTableDataStruct(v))
		f.WriteString(genTableRecordCount(v))
		f.WriteString(genTableRecordMax(v))
		f.WriteString(genTableSelectOne(v))
		f.WriteString(genTableSelectPriKey(v))
		f.WriteString(genTableSelectAll(v))
		f.WriteString(genTableReplaceOne(v, "InsertOne", "insert"))
		f.WriteString(genTableReplaceBatch(v, "InsertBatch", "insert"))
		f.WriteString(genTableReplaceOne(v, "ReplaceOne", "replace"))
		f.WriteString(genTableReplaceBatch(v, "ReplaceBatch", "replace"))
		f.WriteString(genTableCreate(v))
		f.WriteString(genTableDelete(v))
		f.WriteString(genTableDeletePriKey(v))
		f.WriteString(genTableUpdate(v))
		f.WriteString(genTableUpdatePriKey(v))
	}
	return nil
}

func getGOType(typ string) string {
	typ = strings.ToLower(typ)
	if typ == "bit" {
		return "byte"
	}
	if typ == "bool" {
		return "bool"
	}

	if strings.Contains(typ, "int") {
		ret := "int32"
		if strings.Contains(typ, "tiny") {
			ret = "int8"
		} else if strings.Contains(typ, "small") {
			ret = "int16"
		} else if strings.Contains(typ, "medium") {
			ret = "int32"
		} else if strings.Contains(typ, "big") {
			ret = "int64"
		}
		if strings.Contains(typ, "unsigned") {
			ret = "u" + ret
		}
		return ret
	}
	if strings.Contains(typ, "float") {
		return "float32"
	}
	if strings.Contains(typ, "double") || strings.Contains(typ, "decimal") {
		return "float64"
	}
	if strings.Contains(typ, "char") || strings.Contains(typ, "text") {
		return "string"
	}
	if strings.Contains(typ, "blob") {
		return "[]byte"
	}

	if typ == "datetime" || typ == "date" || typ == "time" {
		return "string"
	}
	if typ == "timestamp" {
		return "uint32"
	}
	return "sql.RawBytes"
}

func FileCreate(file string) (*os.File, error) {
	d, f := path.Split(file)
	if f == "" {
		return nil, fmt.Errorf("not a file")
	}
	if d != "" {
		err := os.MkdirAll(d, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	return os.Create(file)
}
