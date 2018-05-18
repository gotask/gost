package stmysql

import (
	"reflect"
	"time"
)

//mysql config
//go's types mapping mysql's types
type globalDBCfg struct {
	mysql string `engine:"InnoDB" charset:"utf8"`

	st_bool   bool        `type:"BOOL"`
	st_int    int         `type:"INT(10)"`
	st_uint   uint        `type:"INT(10) UNSIGNED"`
	st_int8   int8        `type:"TINYINT(1)"`
	st_uint8  uint8       `type:"TINYINT(1) UNSIGNED"`
	st_int32  int32       `type:"INT(10)"`
	st_uint32 uint32      `type:"INT(10) UNSIGNED"`
	st_int64  int64       `type:"BIGINT(20)"`
	st_uint64 uint64      `type:"BIGINT(20) UNSIGNED"`
	st_float  float32     `type:"FLOAT(10)"`
	st_double float64     `type:"DOUBLE(20)"`
	st_string string      `type:"VARCHAR(255)"`
	st_bytes  []byte      `type:"BLOB(65535)"`
	st_time   time.Time   `type:"timestamp"`
	st_others interface{} `type:"BLOB(65535)"`
}

//types that can be stored in database immediately;
//others will be encoded to []byte firstly,then be stored into database;
type typesNotNeedEncode struct {
	st_bool   bool
	st_int    int
	st_uint   uint
	st_int8   int8
	st_uint8  uint8
	st_int32  int32
	st_uint32 uint32
	st_int64  int64
	st_uint64 uint64
	st_float  float32
	st_double float64
	st_string string
	st_bytes  []byte
	st_time   time.Time
}

var mysqlCfg map[string]string
var dbColumnType map[reflect.Type]string
var notEncodeTypes map[reflect.Type]int

func init() {
	mysqlCfg = make(map[string]string)
	dbColumnType = make(map[reflect.Type]string)
	notEncodeTypes = make(map[reflect.Type]int)

	types := reflect.TypeOf(globalDBCfg{})
	glcfg, ok := types.FieldByName("mysql")
	if ok {
		mysqlCfg["engine"] = glcfg.Tag.Get("engine")
		mysqlCfg["charset"] = glcfg.Tag.Get("charset")
	}

	for i := 0; i < types.NumField(); i++ {
		val := types.Field(i).Tag.Get("type")
		if val != "" {
			dbColumnType[types.Field(i).Type] = val
		}
	}

	encTypes := reflect.TypeOf(typesNotNeedEncode{})
	for i := 0; i < encTypes.NumField(); i++ {
		notEncodeTypes[encTypes.Field(i).Type] = 1
	}
}
