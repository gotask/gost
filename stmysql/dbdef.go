package stmysql

import (
	"time"
)

//in tag;
//mysql:"IsExport,Engine,Charset" key:PrimaryKey(..),Index(..),Index... autoinc:"ColumnName,StartNum";
//column:"IsExport,type";
//if mysql's IsExport is false, all columns's IsExport will be omited;
//these tags can be omited all;

type dbTableDefine struct {
	ID1         uint32    `mysql:"false,InnoDB,utf8" key:"primary(ID1,ID2),index(Name)" autoinc:"ID1,1"`
	ID2         uint32    `column:"true,INT(10) UNSIGNED NOT NULL DEFAULT 0"`
	Name        string    `column:"true,VARCHAR(255)"`
	Update_time time.Time `column:"timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`
}

/*
to create a new table or update a exiting table you can use "UpdateTableEx(dbTableDefine{})";dbTableDefine is can be replaced by your struct;

also, you can use "CheckTableEx(dbTableDefine{})" to see the table's status in the database,then use UpdateTableEx writing it into database;
*/
