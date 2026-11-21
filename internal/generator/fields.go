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
	Nullable bool
	JSON     bool
	Tag      string // the bun tag: user_id, meta,type:jsonb
}

// fieldTypes are the types --fields accepts, and the Go type of each.
var fieldTypes = map[string]string{
	"string":  "string",
	"text":    "string",
	"int":     "int64",
	"int64":   "int64",
	"bigint":  "int64",
	"int32":   "int32",
	"integer": "int32",
	"int16":   "int16",
	"float":   "float64",
	"float64": "float64",
	"bool":    "bool",
	"time":    "time.Time",
	"json":    "json.RawMessage",
	"jsonb":   "json.RawMessage",
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
		goType, ok := fieldTypes[typ]
		if !ok {
			return nil, fmt.Errorf("field %q has unknown type %q; use one of %s", column, typ, typeNames())
		}
		isJSON := goType == "json.RawMessage"
		// json.RawMessage is a slice, so NULL already scans into nil.
		if nullable && !isJSON {
			goType = "*" + goType
		}
		tag := column
		if isJSON {
			// Without the type bun would send the bytes as bytea; nullzero
			// writes a nil value as NULL instead of the JSON null.
			tag += ",type:jsonb"
			if nullable {
				tag += ",nullzero"
			}
		}
		fields = append(fields, Field{
			Column:   column,
			GoName:   pascal(column),
			GoType:   goType,
			Nullable: nullable,
			JSON:     isJSON,
			Tag:      tag,
		})
	}
	return fields, nil
}

func typeNames() string {
	return "string, text, int, int64, bigint, int32, integer, int16, float, float64, bool, time, json, jsonb (add ? for nullable)"
}
