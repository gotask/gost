package stmysql

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

//mysql table column description
type dbColumn struct {
	name    string
	export  bool
	typ     string
	autoinc bool
}

//mysql table description
type dbTable struct {
	name       string
	typ        reflect.Type
	export     bool
	engine     string
	charset    string
	primarykey string
	index      []string
	autoincc   string
	autoinci   uint32
	columns    map[string]dbColumn
}

var (
	tableMutex   sync.RWMutex
	tableCfgType map[reflect.Type]dbTable = make(map[reflect.Type]dbTable)
	tableCfgName map[string]dbTable       = make(map[string]dbTable)
)

func getTableConfigByName(table string) *dbTable {
	tableMutex.RLock()
	dbt, has := tableCfgName[table]
	tableMutex.RUnlock()
	if has {
		return &dbt
	}
	return nil
}

func getTableConfig(table interface{}) (*dbTable, error) {
	types := reflect.TypeOf(table)
	if types.Kind() == reflect.String {
		return getTableConfigByName(table.(string)), nil
	}

	var tbtype reflect.Type
	if types.Kind() == reflect.Struct {
		tbtype = types
	} else if types.Kind() == reflect.Ptr && types.Elem().Kind() == reflect.Struct {
		tbtype = types.Elem()
	} else {
		return nil, fmt.Errorf("table should be a struct or a ptr of a struct")
	}

	tableMutex.RLock()
	dbt, has := tableCfgType[tbtype]
	tableMutex.RUnlock()
	if has {
		return &dbt, nil
	}

	tableMutex.Lock()
	d, e := addTable(tbtype)
	tableMutex.Unlock()
	return d, e
}

func addTable(tbtype reflect.Type) (*dbTable, error) {
	tbcfg := dbTable{
		name:     tbtype.Name(),
		typ:      tbtype,
		export:   true,
		engine:   mysqlCfg["engine"],
		charset:  mysqlCfg["charset"],
		autoinci: 1,
		index:    make([]string, 0, 3),
		columns:  make(map[string]dbColumn),
	}
	for i := 0; i < tbtype.NumField(); i++ {
		val := tbtype.Field(i).Tag

		colcfg := dbColumn{
			name:    tbtype.Field(i).Name,
			export:  true,
			autoinc: false,
		}
		//read tag column:IsExport,type
		col := val.Get("column")
		if col != "" {
			res := strings.Split(col, ",")
			if len(res) >= 1 {
				if res[0] == "true" {
					colcfg.export = true
				} else {
					colcfg.export = false
				}
			}
			if len(res) >= 2 {
				colcfg.typ = res[1]
			}
		}
		if colcfg.typ == "" {
			if v, ok := dbColumnType[tbtype.Field(i).Type]; ok {
				colcfg.typ = v
			} else {
				other, ok := reflect.TypeOf(globalDBCfg{}).FieldByName("st_others")
				if ok {
					colcfg.typ = dbColumnType[other.Type]
				}
			}
		}
		tbcfg.columns[colcfg.name] = colcfg
		//read tag mysql:IsExport,Engine,Charset
		mys := val.Get("mysql")
		if mys != "" {
			res := strings.Split(mys, ",")
			if len(res) >= 1 {
				if res[0] == "true" {
					tbcfg.export = true
				} else {
					tbcfg.export = false
				}
			}
			if len(res) >= 2 {
				tbcfg.engine = res[1]
			}
			if len(res) >= 3 {
				tbcfg.charset = res[2]
			}
		}
		//read tag key:primary(..),index(..),index...
		keys := val.Get("key")
		if keys != "" {
			if strings.HasPrefix(keys, "primary") {
				keys = keys[len("primary"):]
				if keys == "" || keys[0] != '(' {
					return nil, fmt.Errorf("table[%s] primary format error key:[%s]", tbtype.Name(), val.Get("key"))
				}
				pos := 0
				for ; pos < len(keys); pos++ {
					if keys[pos] == ')' {
						break
					}
				}
				if keys[pos] != ')' {
					return nil, fmt.Errorf("table[%s] primary format error key:[%s]", tbtype.Name(), val.Get("key"))
				}
				tbcfg.primarykey = keys[:pos+1]
				if len(keys) > pos+2 {
					keys = keys[pos+2:]
				}
			}
			for strings.HasPrefix(keys, "index") {
				keys = keys[len("index"):]
				if keys == "" || keys[0] != '(' {
					return nil, fmt.Errorf("table[%s] index format error key:[%s]", tbtype.Name(), val.Get("key"))
				}
				pos := 0
				for ; pos < len(keys); pos++ {
					if keys[pos] == ')' {
						break
					}
				}
				if keys[pos] != ')' {
					return nil, fmt.Errorf("table[%s] index format error key:[%s]", tbtype.Name(), val.Get("key"))
				}
				tbcfg.index = append(tbcfg.index, keys[:pos+1])
				if len(keys) > pos+2 {
					keys = keys[pos+2:]
				}
			}
		}
		//read tag autoinc:"ColumnName,StartNum"
		autoinc := val.Get("autoinc")
		if autoinc != "" {
			res := strings.Split(autoinc, ",")
			if len(res) >= 1 {
				tbcfg.autoincc = res[0]
			}
			if len(res) >= 2 {
				i, e := strconv.Atoi(res[1])
				if e != nil {
					return nil, e
				}
				tbcfg.autoinci = uint32(i)
			}
		}
	}
	//when tag autoinc is not null,change the column's config
	if tbcfg.autoincc != "" {
		if v, ok := tbcfg.columns[tbcfg.autoincc]; ok {
			v.autoinc = true
			tbcfg.columns[tbcfg.autoincc] = v
		} else {
			return nil, fmt.Errorf("table[%s] autoinc format error autoinc:[%s]", tbcfg.name, tbcfg.autoincc)
		}
	}
	tableCfgType[tbcfg.typ] = tbcfg
	tableCfgName[tbcfg.name] = tbcfg
	return &tbcfg, nil
}
