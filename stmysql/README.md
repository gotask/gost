stmysql is a lib to operate the database of mysql.
>
to create a new table or update a exiting table you can use "UpdateTableEx(dbTableDefine{})";
dbTableDefine is can be replaced by your struct;
also, you can use "CheckTableEx(dbTableDefine{})" to see the table's status in the database,
then use UpdateTableEx writing it into database;

example
```
import (
	"database/sql"
	_ "encoding/json"
	"fmt"
	"golib/stmysql"
	_ "github.com/go-sql-driver/mysql"
)

type person struct {
	Id   uint
	Name string
	Age  uint8
}

func main() {
	db, err := sql.Open("mysql", "name:pwd@tcp(10.0.0.227:3306)/")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	fmt.Println(stmysql.CreateDatabase(db, "Test"))
	tablestr, err1 := stmysql.UpdateTableEx(db, person{})
	fmt.Println(tablestr, err1)

	per := &person{}
	fmt.Println(stmysql.SelectOne(db, per, "where id=?", 2))
	fmt.Println(per)
	cols, _ := stmysql.SelectAll(db, person{})
	for _, v := range cols {
		fmt.Println(v.(*person))
	}
	fmt.Println(stmysql.GetRecordCount(db, per))
	fmt.Println(stmysql.GetRecordMax(db, per, "id"))
	fmt.Println(stmysql.ReplaceBatch(db, []person{person{5, "e", 10}}))
	fmt.Println(stmysql.InsertBatch(db, []*person{&person{6, "f", 10}}))
	fmt.Println(stmysql.InsertOne(db, person{1, "e", 10}))
	fmt.Println(stmysql.InsertOne(db, &person{9, "f", 10}))
	fmt.Println(stmysql.ReplaceOne(db, per))
	fmt.Println(stmysql.DeleteRecord(db, per, "where id=?", 1))
	fmt.Println(stmysql.UpdateRecord(db, "person", map[string]interface{}{"Name": "XXX", "Age": 66}, "where id=?", 2))
	fmt.Println(stmysql.UpdateRecordEx(db, person{2, "e", 10}, "where id=?", 2))
}
```