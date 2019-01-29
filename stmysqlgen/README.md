stmysqlgen is a lib to generate go files to operate mysql.
>
example
```
//table t_role{id,name,blobdata}
func main() {
	fmt.Println(GenGOFile("user", "pwd", "127.0.0.1", 3306, "db_test", "main"))

	db := &DB_db_test{}
	db.Open("moba", "moba2016", "192.168.40.220", 3306)
	defer db.Close()
	fmt.Println(db.T_t_role.Count())
	fmt.Println(db.T_t_role.Count("where id>? and id<?", 1000, 3000))
	fmt.Println(db.T_t_role.Max("name"))
	fmt.Println(db.T_t_role.SelectOne("where name=?", "123"))
	fmt.Println(db.T_t_role.Select("where id<?", 2000))
	fmt.Println(db.T_t_role.ReplaceOne(&D_db_test_t_role{1, "123", nil}))
	fmt.Println(db.T_t_role.ReplaceBatch([]*D_db_test_t_role{
		&D_db_test_t_role{1, "123", nil},
		&D_db_test_t_role{2, "234", nil},
		&D_db_test_t_role{3, "345", []byte{3, 1}},
	}))
	fmt.Println(db.T_t_role.InsertOne(&D_db_test_t_role{1, "123" , []byte{1, 2}}))
	fmt.Println(db.T_t_role.InsertOne(&D_db_test_t_role{7, "", []byte{7, 7}}))
	res, err := db.T_t_role.InsertBatch([]*D_db_test_t_role{
		&D_db_test_t_role{1, "123", []byte{1, 1}},
		&D_db_test_t_role{2, "123", nil},
		&D_db_test_t_role{5, "123", []byte{5, 5}},
	})
	fmt.Println(res, err)
	fmt.Println(db.T_t_role.Update(map[string]interface{}{"name": "hello"}, "where id=?", 2))
	fmt.Println(db.T_t_role.Select())
	fmt.Println(db.T_t_role.Delete("where id=?", 1))
}
```