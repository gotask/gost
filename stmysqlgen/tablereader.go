// tablereader.go
package stmysqlgen

import (
	"fmt"
	"strings"
)

func genTableSqlStruct(table *MSTable) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "type T_%s_%s struct{\n", table.DB, table.Name)
	builder.WriteString("\tDB *sql.DB\n")
	builder.WriteString("}\n")
	return builder.String()
}
func genTableDataStruct(table *MSTable) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "type D_%s_%s struct{\n", table.DB, table.Name)
	for _, c := range table.Cols {
		builder.WriteString("\tF_")
		builder.WriteString(c.Name)
		builder.WriteString(" ")
		builder.WriteString(getGOType(c.Type))
		builder.WriteString(" `field:")
		builder.WriteString(c.Name)
		builder.WriteString(" type:")
		builder.WriteString(c.Type)
		builder.WriteString(" key:")
		builder.WriteString(c.Key)
		builder.WriteString("`\n")
	}
	builder.WriteString("}\n")
	return builder.String()
}

func genTableRecordCount(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//select count(*) as num from %s.%s where x=?\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) Count(whereArgs ...interface{}) (int, error) {\n", tname)
	builder.WriteString("\tsqlcmd := \"select count(*) as num from ")
	builder.WriteString(table.Name)
	builder.WriteString(" \"\n")
	code := `	if len(whereArgs) > 0 {
		sqlcmd += whereArgs[0].(string)
	}
	var (
		rows *sql.Rows
		err  error
	)
	if len(whereArgs) > 1 {
		rows, err = t.DB.Query(sqlcmd, whereArgs[1:]...)
	} else {
		rows, err = t.DB.Query(sqlcmd)
	}
	defer func() {
        if rows != nil {
            rows.Close()
        }
     }()
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var num int
		err = rows.Scan(&num)
		return num, err
	}
	return 0, nil
`
	builder.WriteString(code)
	builder.WriteString("}\n")
	return builder.String()
}
func genTableRecordMax(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//select column as c from %s.%s where x=? order by c desc limit 1\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) Max(column string, whereArgs ...interface{}) (string, error) {\n", tname)
	builder.WriteString("\tsqlcmd := \"select \" + column + \" as c from ")
	builder.WriteString(table.Name)
	builder.WriteString(" \"\n")
	code := `	if len(whereArgs) > 0 {
		sqlcmd += whereArgs[0].(string)
	}
	sqlcmd += " order by c desc limit 1"
	var (
		rows *sql.Rows
		err  error
	)
	if len(whereArgs) > 1 {
		rows, err = t.DB.Query(sqlcmd, whereArgs[1:]...)
	} else {
		rows, err = t.DB.Query(sqlcmd)
	}
	defer func() {
        if rows != nil {
            rows.Close()
        }
     }()
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var max string
		err = rows.Scan(&max)
		return max, err
	}
	return "", nil
`
	builder.WriteString(code)
	builder.WriteString("}\n")
	return builder.String()
}

func genSelectOneScan(table *MSTable) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "\tfor rows.Next() {\n\t\tvar result D_%s_%s\n", table.DB, table.Name)
	builder.WriteString("\t\terr = rows.Scan(")
	for i, c := range table.Cols {
		builder.WriteString("&result.F_")
		builder.WriteString(c.Name)
		if i != len(table.Cols)-1 {
			builder.WriteString(", ")
		}
	}
	builder.WriteString(")\n\t\treturn &result, err\n\t}\n")
	return builder.String()
}

func genTableSelectOne(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	dname := "D_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//select * from %s.%s where x=? limit 1\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) SelectOne(whereArgs ...interface{}) (*%s, error) {\n", tname, dname)
	builder.WriteString("\tsqlcmd := \"select * from ")
	builder.WriteString(table.Name)
	builder.WriteString(" \"\n")
	code := `	if len(whereArgs) > 0 {
		sqlcmd += whereArgs[0].(string)
	}
	sqlcmd += " limit 1"
	var (
		rows *sql.Rows
		err  error
	)
	if len(whereArgs) > 1 {
		rows, err = t.DB.Query(sqlcmd, whereArgs[1:]...)
	} else {
		rows, err = t.DB.Query(sqlcmd)
	}
	defer func() {
        if rows != nil {
            rows.Close()
        }
     }()
	if err != nil {
		return nil, err
	}
`
	builder.WriteString(code)
	builder.WriteString(genSelectOneScan(table))
	builder.WriteString("\treturn nil, nil\n")
	builder.WriteString("}\n")
	return builder.String()
}

func genTableSelectPriKey(table *MSTable) string {
	if table.PriKeyNum != 1 {
		return ""
	}
	tname := "T_" + table.DB + "_" + table.Name
	dname := "D_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//select * from %s.%s where %s=? limit 1\n", table.DB, table.Name, table.PriKey.Name)
	fmt.Fprintf(&builder, "func (t *%s) SelectPriKey(key %s) (*%s, error) {\n", tname, getGOType(table.PriKey.Type), dname)
	builder.WriteString("\tsqlcmd := \"select * from ")
	builder.WriteString(table.Name)
	builder.WriteString(" where ")
	builder.WriteString(table.PriKey.Name)
	builder.WriteString("=?  limit 1\"\n")
	code := `
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = t.DB.Query(sqlcmd, key)
	defer func() {
        if rows != nil {
            rows.Close()
        }
     }()
	if err != nil {
		return nil, err
	}
`
	builder.WriteString(code)
	builder.WriteString(genSelectOneScan(table))
	builder.WriteString("\treturn nil, nil\n")
	builder.WriteString("}\n")
	return builder.String()
}

func genSelectAllScan(table *MSTable) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "\tvar results []D_%s_%s\n\tfor rows.Next() {\n\t\tvar result D_%s_%s\n", table.DB, table.Name, table.DB, table.Name)
	builder.WriteString("\t\terr = rows.Scan(")
	for i, c := range table.Cols {
		builder.WriteString("&result.F_")
		builder.WriteString(c.Name)
		if i != len(table.Cols)-1 {
			builder.WriteString(", ")
		}
	}
	builder.WriteString(")\n\t\tif err != nil {\n\t\t\treturn nil,err\n\t\t}\n\t\tresults = append(results, result)\n\t}\n")
	return builder.String()
}

func genTableSelectAll(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	dname := "D_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//select * from %s.%s where x=?\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) Select(whereArgs ...interface{}) ([]%s, error) {\n", tname, dname)
	builder.WriteString("\tsqlcmd := \"select * from ")
	builder.WriteString(table.Name)
	builder.WriteString(" \"\n")
	code := `	if len(whereArgs) > 0 {
		sqlcmd += whereArgs[0].(string)
	}
	var (
		rows *sql.Rows
		err  error
	)
	if len(whereArgs) > 1 {
		rows, err = t.DB.Query(sqlcmd, whereArgs[1:]...)
	} else {
		rows, err = t.DB.Query(sqlcmd)
	}
	defer func() {
        if rows != nil {
            rows.Close()
        }
     }()
	if err != nil {
		return nil, err
	}
`
	builder.WriteString(code)
	builder.WriteString(genSelectAllScan(table))
	builder.WriteString("\treturn results, nil\n")
	builder.WriteString("}\n")
	return builder.String()
}

func genReplaceValue(table *MSTable) string {
	var builder strings.Builder
	builder.WriteString("(")
	for i, _ := range table.Cols {
		builder.WriteString("?")
		if i != len(table.Cols)-1 {
			builder.WriteString(", ")
		}
	}
	builder.WriteString(")")
	return builder.String()
}
func genTableReplaceOne(table *MSTable, fun, oper string) string {
	tname := "T_" + table.DB + "_" + table.Name
	dname := "D_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//replace into %s.%s (x, y, z)values(?, ?, ?)\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) %s(data *%s) (sql.Result, error) {\n\tsqlcmd := \"%s into %s (", tname, fun, dname, oper, table.Name)
	for i, c := range table.Cols {
		builder.WriteString(c.Name)
		if i != len(table.Cols)-1 {
			builder.WriteString(", ")
		}
	}
	builder.WriteString(")values")
	builder.WriteString(genReplaceValue(table))
	builder.WriteString("\"\n\treturn t.DB.Exec(sqlcmd, ")
	for i, c := range table.Cols {
		builder.WriteString("data.F_")
		builder.WriteString(c.Name)
		if i != len(table.Cols)-1 {
			builder.WriteString(", ")
		}
	}
	builder.WriteString(")\n}\n")
	return builder.String()
}

func genReplaceBatchValue(table *MSTable) string {
	var builder strings.Builder
	builder.WriteString("\tvalins := make([]interface{}, 0, len(data))\n")
	builder.WriteString("\tfor _,d:=range data{\n")
	for _, c := range table.Cols {
		builder.WriteString("\t\tvalins = append(valins, d.F_")
		builder.WriteString(c.Name)
		builder.WriteString(")\n")
	}
	builder.WriteString("\t}\n")
	return builder.String()
}

func genTableReplaceBatch(table *MSTable, fun, oper string) string {
	tname := "T_" + table.DB + "_" + table.Name
	dname := "D_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//replace into %s.%s (x, y)values(?, ?),(?, ?)\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) %s(data []*%s) (sql.Result, error) {\n\tvar sqlcmd strings.Builder\n\tsqlcmd.WriteString(\"%s into %s (", tname, fun, dname, oper, table.Name)
	for i, c := range table.Cols {
		builder.WriteString(c.Name)
		if i != len(table.Cols)-1 {
			builder.WriteString(", ")
		}
	}
	builder.WriteString(")values\")\n")
	builder.WriteString("\tvals := \"")
	builder.WriteString(genReplaceValue(table))
	builder.WriteString("\"\n\tfor i:=0;i<len(data);i++{\n\t\tsqlcmd.WriteString(vals)\n\t\tif i!=len(data)-1{\n\t\t\tsqlcmd.WriteString(\", \")\n\t\t}\n\t}\n")
	builder.WriteString(genReplaceBatchValue(table))
	builder.WriteString("\n\treturn t.DB.Exec(sqlcmd.String(), valins...)\n}\n")
	return builder.String()
}

func genTableCreate(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//create table %s.%s\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) Create() (sql.Result, error) {\n", tname)
	builder.WriteString("\tsqlcmd := `")
	builder.WriteString(table.CreateCmd)
	builder.WriteString("`\n")
	builder.WriteString("\treturn t.DB.Exec(sqlcmd)\n}\n")
	return builder.String()
}

func genTableDelete(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//delete from %s.%s where x=?\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) Delete(where string, whereArgs ...interface{}) (sql.Result, error) {\n", tname)
	builder.WriteString("\tsqlcmd := \"delete from ")
	builder.WriteString(table.Name)
	builder.WriteString(" \" + where\n")
	builder.WriteString("\treturn t.DB.Exec(sqlcmd, whereArgs...)\n}\n")
	return builder.String()
}

func genTableDeletePriKey(table *MSTable) string {
	if table.PriKeyNum != 1 {
		return ""
	}
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//delete from %s.%s where %s=?\n", table.DB, table.Name, table.PriKey.Name)
	fmt.Fprintf(&builder, "func (t *%s) DeletePriKey(key %s) (sql.Result, error) {\n", tname, getGOType(table.PriKey.Type))
	builder.WriteString("\tsqlcmd := \"delete from ")
	builder.WriteString(table.Name)
	builder.WriteString(" where ")
	builder.WriteString(table.PriKey.Name)
	builder.WriteString("=?\"\n")
	builder.WriteString("\treturn t.DB.Exec(sqlcmd, key)\n}\n")
	return builder.String()
}

func genTableUpdate(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//update %s.%s set x=?,y=? where w=?\n", table.DB, table.Name)
	fmt.Fprintf(&builder, "func (t *%s) Update(data map[string]interface{},where string, whereArgs ...interface{}) (sql.Result, error) {\n", tname)
	builder.WriteString("\tvar sqlcmd strings.Builder\n")
	builder.WriteString("\tsqlcmd.WriteString(\"update ")
	builder.WriteString(table.Name)
	builder.WriteString(" set \")\n\tisfirst:=true\n")
	builder.WriteString("\tvalins := make([]interface{}, 0, len(data)+len(whereArgs))\n")
	builder.WriteString("\tfor k,v:=range data{\n\t\tif !isfirst{\n\t\t\tsqlcmd.WriteString(\",\")\n\t\t}\n\t\tsqlcmd.WriteString(k)\n\t\tsqlcmd.WriteString(\"=? \")\n\t\tisfirst=false\n")
	builder.WriteString("\t\tvalins = append(valins, v)\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\tvalins = append(valins, whereArgs...)\n")
	builder.WriteString("\tsqlcmd.WriteString(where)\n")
	builder.WriteString("\treturn t.DB.Exec(sqlcmd.String(), valins...)\n}\n")
	return builder.String()
}

func genTableUpdatePriKey(table *MSTable) string {
	tname := "T_" + table.DB + "_" + table.Name
	var builder strings.Builder
	fmt.Fprintf(&builder, "//update %s.%s set x=?,y=? where %s=?\n", table.DB, table.Name, table.PriKey.Name)
	fmt.Fprintf(&builder, "func (t *%s) UpdatePriKey(data map[string]interface{},key %s) (sql.Result, error) {\n", tname, getGOType(table.PriKey.Type))
	builder.WriteString("\tvar sqlcmd strings.Builder\n")
	builder.WriteString("\tsqlcmd.WriteString(\"update ")
	builder.WriteString(table.Name)
	builder.WriteString(" set \")\n\tisfirst:=true\n")
	builder.WriteString("\tvalins := make([]interface{}, 0, len(data)+1)\n")
	builder.WriteString("\tfor k,v:=range data{\n\t\tif !isfirst{\n\t\t\tsqlcmd.WriteString(\",\")\n\t\t}\n\t\tsqlcmd.WriteString(k)\n\t\tsqlcmd.WriteString(\"=? \")\n\t\tisfirst=false\n")
	builder.WriteString("\t\tvalins = append(valins, v)\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\tvalins = append(valins, key)\n")
	str := " where " + table.PriKey.Name + "=?"
	fmt.Fprintf(&builder, "\tsqlcmd.WriteString(\"%s\")\n", str)
	builder.WriteString("\treturn t.DB.Exec(sqlcmd.String(), valins...)\n}\n")
	return builder.String()
}
