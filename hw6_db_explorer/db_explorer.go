package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

//field struct describes field in a table in a database
type field struct {
	Name       string
	Type       string
	Collation  interface{}
	Null       string
	Key        string
	Default    interface{}
	Extra      string
	Privileges string
	Comment    string
}

type queryParams struct {
	limit  int
	offset int
}

//DbExplorer struct describes a MySQL database manager that allows CRUD queries
type DbExplorer struct {
	conn   *sql.DB
	tables map[string][]field
}

func NewDbExplorer(db *sql.DB) (*DbExplorer, error) {
	tables, err := getDBTables(db)
	if err != nil {
		return nil, errors.Wrap(err, "getting db tables error")
	}
	return &DbExplorer{db, tables}, nil
}

func getDBTables(db *sql.DB) (map[string][]field, error) {
	tables := make(map[string][]field)
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, errors.Wrap(err, "db query error")
	}
	var tableName string
	for rows.Next() {
		if err := rows.Scan(&tableName); err != nil {
			return nil, errors.Wrap(err, "db scan error")
		}
		tables[tableName] = []field{}
	}
	rows.Close()

	var field field
	for name := range tables {
		rows, err := db.Query("SHOW FULL COLUMNS FROM " + name + ";")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			if err := rows.Scan(&field.Name, &field.Type, &field.Collation, &field.Null, &field.Key, &field.Default, &field.Extra, &field.Privileges, &field.Comment); err != nil {
				return nil, err
			}
			tables[name] = append(tables[name], field)
		}
		rows.Close()
	}
	return tables, nil
}

func (dbe *DbExplorer) tableExist(name string) bool {
	_, ok := dbe.tables[name]
	return ok
}

func (dbe *DbExplorer) tablePrimaryFieldName(name string) (string, error) {
	fields, ok := dbe.tables[name]
	if !ok {
		return "", errors.New("no such table")
	}

	for _, f := range fields {
		if f.Key == "PRI" {
			return f.Name, nil
		}
	}
	return "", errors.New("no primary key")
}

func parseQueryParams(params map[string][]string) queryParams {
	qp := queryParams{
		limit:  5,
		offset: 0,
	}
	if sLimit, ok := params["limit"]; ok {
		limit, err := strconv.Atoi(sLimit[0])
		if err == nil {
			qp.limit = limit
		}
	}
	if sOffset, ok := params["offset"]; ok {
		offset, err := strconv.Atoi(sOffset[0])
		if err == nil {
			qp.offset = offset
		}
	}
	return qp
}

func (dbe *DbExplorer) parseValues(values []interface{}, tableName string) (map[string]interface{}, error) {
	object := make(map[string]interface{})
	for i, col := range dbe.tables[tableName] {
		val := values[i]

		var v interface{}
		b, ok := val.([]byte)
		if ok {
			switch col.Type {
			case "int":
				intV, err := strconv.Atoi(string(b))
				if err != nil {
					fmt.Println(err)
				}
				v = intV
			default:
				v = string(b)
			}
		} else {
			v = val
		}
		object[col.Name] = v
	}
	return object, nil
}

func (dbe *DbExplorer) setDefaultValue(value *interface{}, vType string) {
	switch vType {
	case "int":
		var i int
		*value = i
	case "varchar(255)", "text":
		var s string
		*value = s
	case "bool":
		var b bool
		*value = b
	default:
		var s string
		*value = s
	}
}

func (dbe *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")

	urlPaths := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	var err error

	switch r.Method {
	case "GET":
		if r.URL.Path == "/" {
			err = dbe.GetTables(w, r)
		} else {
			switch len(urlPaths) {
			case 1:
				err = dbe.GetColumns(urlPaths[0], w, r)
			case 2:
				err = dbe.GetColumn(urlPaths[0], urlPaths[1], w, r)
			default:
				http.Error(w, PageNotFoundErrJSON, http.StatusNotFound)
				return
			}
		}
	case "POST":
		if len(urlPaths) == 2 {
			err = dbe.UpdateColumn(urlPaths[0], urlPaths[1], w, r)
		}
	case "PUT":
		if len(urlPaths) == 1 {
			err = dbe.CreateColumn(urlPaths[0], w, r)
		}
	case "DELETE":
		if len(urlPaths) == 2 {
			err = dbe.DeleteColumn(urlPaths[0], urlPaths[1], w, r)
		}
	default:
		http.Error(w, InternalErrJSON, http.StatusInternalServerError)
	}

	if err != nil {
		//fmt.Println("error:", err)
		if serr, ok := err.(*SpecificError); ok {
			w.WriteHeader(serr.Code)
			if err := json.NewEncoder(w).Encode(serr); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(InternalErrJSON))
				return
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(InternalErrJSON))
		}
	}
}

func (dbe *DbExplorer) GetTables(w http.ResponseWriter, r *http.Request) error {
	var tables []string
	rows, err := dbe.conn.Query("SHOW TABLES")
	if err != nil {
		return errors.Wrap(err, "db query error")
	}

	var tableName string
	for rows.Next() {
		if err := rows.Scan(&tableName); err != nil {
			return errors.Wrap(err, "db scan error")
		}
		tables = append(tables, tableName)
	}
	rows.Close()

	resp := map[string]map[string][]string{
		"response": {
			"tables": tables,
		},
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (dbe *DbExplorer) GetColumns(tableName string, w http.ResponseWriter, r *http.Request) error {
	if !dbe.tableExist(tableName) {
		return &SpecificError{"unknown table", http.StatusNotFound}
	}
	qp, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return errors.Wrap(err, "parse query error")
	}

	queryParams := parseQueryParams(qp)

	query := fmt.Sprintf(`SELECT * FROM %s LIMIT %v OFFSET %v;`, tableName, queryParams.limit, queryParams.offset)
	rows, err := dbe.conn.Query(query)
	if err != nil {
		return errors.Wrap(err, "db query error")
	}

	values := make([]interface{}, len(dbe.tables[tableName]))
	valuePtrs := make([]interface{}, len(dbe.tables[tableName]))
	for i := range dbe.tables[tableName] {
		valuePtrs[i] = &values[i]
	}

	var objects []map[string]interface{}
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return errors.Wrap(err, "db scan error")
		}

		obj, err := dbe.parseValues(values, tableName)
		if err != nil {
			return errors.Wrap(err, "parse record values error")
		}
		objects = append(objects, obj)
	}
	rows.Close()

	resp := map[string]map[string][]map[string]interface{}{
		"response": {
			"records": objects,
		},
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (dbe *DbExplorer) GetColumn(tableName string, recordID string, w http.ResponseWriter, r *http.Request) error {
	if !dbe.tableExist(tableName) {
		return &SpecificError{"unknown table", http.StatusNotFound}
	}

	id, err := strconv.Atoi(recordID)
	if err != nil {
		return errors.Wrap(err, "converting record id to int error")
	}

	pFieldName, err := dbe.tablePrimaryFieldName(tableName)
	if err != nil {
		return err
	}

	row := dbe.conn.QueryRow("SELECT * FROM "+tableName+" WHERE "+pFieldName+" = ?", id)

	values := make([]interface{}, len(dbe.tables[tableName]))
	valuePtrs := make([]interface{}, len(dbe.tables[tableName]))
	for i := range dbe.tables[tableName] {
		valuePtrs[i] = &values[i]
	}

	if err = row.Scan(valuePtrs...); err == sql.ErrNoRows {
		return &SpecificError{"record not found", http.StatusNotFound}
	} else if err != nil {
		return errors.Wrap(err, "db scan error")
	}

	obj, err := dbe.parseValues(values, tableName)
	if err != nil {
		return errors.Wrap(err, "parse record values error")
	}

	resp := map[string]map[string]interface{}{
		"response": {
			"record": obj,
		},
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (dbe *DbExplorer) CreateColumn(tableName string, w http.ResponseWriter, r *http.Request) error {
	defer r.Body.Close()

	if !dbe.tableExist(tableName) {
		return &SpecificError{"unknown table", http.StatusNotFound}
	}

	data := make(map[string]interface{})
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return err
	}

	var values []interface{}
	var fieldNames, fieldPlaceholders []string

	for _, field := range dbe.tables[tableName] {
		if strings.Contains(field.Extra, "auto_increment") {
			continue
		}
		fieldNames = append(fieldNames, "`"+field.Name+"`")
		fieldPlaceholders = append(fieldPlaceholders, "?")

		var value interface{}

		if v, ok := data[field.Name]; ok {
			value = v
		} else if field.Default != nil {
			value = field.Default
		} else if field.Null == "NO" {
			dbe.setDefaultValue(&value, field.Type)
		}
		values = append(values, value)
	}

	qFieldNamesStr := strings.Join(fieldNames, ", ")
	qFieldPlaceholdersStr := strings.Join(fieldPlaceholders, ", ")
	query := fmt.Sprintf("INSERT INTO %v (%s) VALUES (%s)", tableName, qFieldNamesStr, qFieldPlaceholdersStr)

	result, err := dbe.conn.Exec(query, values...)
	if err != nil {
		return err
	}
	lastID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	primaryFieldName, err := dbe.tablePrimaryFieldName(tableName)
	if err != nil {
		return err
	}

	resp := map[string]map[string]int{
		"response": {
			primaryFieldName: int(lastID),
		},
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (dbe *DbExplorer) UpdateColumn(tableName string, recordID string, w http.ResponseWriter, r *http.Request) error {
	if !dbe.tableExist(tableName) {
		return &SpecificError{"unknown table", http.StatusNotFound}
	}

	id, err := strconv.Atoi(recordID)
	if err != nil {
		return errors.Wrap(err, "converting record id to int error")
	}

	data := make(map[string]interface{})
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return err
	}

	var values []interface{}
	var fieldNames []string
	for _, field := range dbe.tables[tableName] {
		if v, ok := data[field.Name]; ok {
			if field.Extra == "auto_increment" {
				return &SpecificError{"field " + field.Name + " have invalid type", http.StatusBadRequest}
			}

			switch field.Type {
			case "int", "float":
				if _, ok := v.(float64); !ok {
					return &SpecificError{"field " + field.Name + " have invalid type", http.StatusBadRequest}
				}
			case "varchar(255)", "text":
				if _, ok := v.(string); !ok && v != nil {
					return &SpecificError{"field " + field.Name + " have invalid type", http.StatusBadRequest}
				}
			case "bool":
				if _, ok := v.(bool); !ok {
					return &SpecificError{"field " + field.Name + " have invalid type", http.StatusBadRequest}
				}
			}

			if v == nil && field.Null == "NO" {
				return &SpecificError{"field " + field.Name + " have invalid type", http.StatusBadRequest}
			}

			fieldNames = append(fieldNames, "`"+field.Name+"` = ?")
			values = append(values, v)
		}
	}

	qFieldNamesStr := strings.Join(fieldNames, ", ")
	primaryFieldName, err := dbe.tablePrimaryFieldName(tableName)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("UPDATE %v SET %s WHERE %s = ?", tableName, qFieldNamesStr, primaryFieldName)

	//add to values record id
	values = append(values, id)

	result, err := dbe.conn.Exec(query, values...)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	resp := map[string]map[string]int{
		"response": {
			"updated": int(affected),
		},
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

func (dbe *DbExplorer) DeleteColumn(tableName string, recordID string, w http.ResponseWriter, r *http.Request) error {
	if !dbe.tableExist(tableName) {
		return &SpecificError{"unknown table", http.StatusNotFound}
	}

	id, err := strconv.Atoi(recordID)
	if err != nil {
		return errors.Wrap(err, "converting record id to int error")
	}

	primaryFieldName, err := dbe.tablePrimaryFieldName(tableName)
	if err != nil {
		return err
	}

	result, err := dbe.conn.Exec("DELETE FROM "+tableName+" WHERE "+primaryFieldName+" = ?", id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	resp := map[string]map[string]int{
		"response": {
			"deleted": int(affected),
		},
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}
