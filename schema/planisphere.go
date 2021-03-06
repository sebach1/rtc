package schema

import (
	"github.com/sebach1/rtc/integrity"
)

// A Planisphere describes the scope in which the gSchemas will be searched in
type Planisphere []*Schema

// GetSchemaFromName retrieves the schema assigned to the name and checks if it exists and is of the desired kind
func (psph Planisphere) GetSchemaFromName(schemaName integrity.SchemaName) (*Schema, error) {
	for _, sch := range psph {
		if sch == nil {
			continue
		}
		if sch.Name != "" && sch.Name == schemaName {
			return sch, nil
		}
	}
	return nil, errSchemaNotFoundInScope
}

// preciseTableErr will assume there is an error with the tableName. Then, it precises the current behaviour.
// To achieve it, checks if the given table exists in the planisphere.
func (psph Planisphere) preciseTableErr(tableName integrity.TableName) (err error) {
	for _, sch := range psph {
		for _, table := range sch.tableNames() {
			if table == tableName {
				return errForeignTable
			}
		}
	}
	return errNonexistentTable
}
