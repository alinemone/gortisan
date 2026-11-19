package generator

import (
	"fmt"
	"strings"
)

// Field is one column of a generated model.
type Field struct {
	Column   string // user_id
	GoName   string // UserID
	GoType   string // int64, *string, ...
	SQLType  string // BIGINT
	Nullable bool
	JSON     bool
}

type columnType struct {
	goType  string
	sqlType string
}

// fieldTypes are the types --fields accepts.
var fieldTypes = map[string]columnType{
	"string":  {"string", "TEXT"},
	"text":    {"string", "TEXT"},
	"int":     {"int64", "BIGINT"},
	"int64":   {"int64", "BIGINT"},
	"bigint":  {"int64", "BIGINT"},
	"int32":   {"int32", "INTEGER"},
	"integer": {"int32", "INTEGER"},
	"int16":   {"int16", "SMALLINT"},
	"float":   {"float64", "DOUBLE PRECISION"},
	"float64": {"float64", "DOUBLE PRECISION"},
	"bool":    {"bool", "BOOLEAN"},
	"time":    {"time.Time", "TIMESTAMPTZ"},
	"json":    {"json.RawMessage", "JSONB"},
	"jsonb":   {"json.RawMessage", "JSONB"},
}

// reservedColumns are written by every model.
var reservedColumns = map[string]bool{"id": true, "created_at": true, "updated_at": true, "deleted_at": true}

// parseFields reads "user_id:int64,status:string,paid_at:time?". A trailing ?
// makes the column nullable (a pointer in Go).
func parseFields(spec string) ([]Field, error) {
	var fields []Field
	seen := map[string]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, typ, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("field %q needs a type, e.g. %s:string", part, name)
		}
		column, err := identifier("field", name)
		if err != nil {
			return nil, err
		}
		if reservedColumns[column] {
			return nil, fmt.Errorf("field %q is added by every model; leave it out", column)
		}
		if seen[column] {
			return nil, fmt.Errorf("field %q is listed twice", column)
		}
		seen[column] = true

		typ = strings.ToLower(strings.TrimSpace(typ))
		nullable := strings.HasSuffix(typ, "?")
		typ = strings.TrimSuffix(typ, "?")
		ct, ok := fieldTypes[typ]
		if !ok {
			return nil, fmt.Errorf("field %q has unknown type %q; use one of %s", column, typ, typeNames())
		}
		goType := ct.goType
		isJSON := goType == "json.RawMessage"
		// json.RawMessage is a slice, so NULL already scans into nil.
		if nullable && !isJSON {
			goType = "*" + goType
		}
		fields = append(fields, Field{
			Column:   column,
			GoName:   pascal(column),
			GoType:   goType,
			SQLType:  ct.sqlType,
			Nullable: nullable,
			JSON:     isJSON,
		})
	}
	return fields, nil
}

func typeNames() string {
	return "string, text, int, int64, bigint, int32, integer, int16, float, float64, bool, time, json, jsonb (add ? for nullable)"
}
