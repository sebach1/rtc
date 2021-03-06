/*
Package schema is the main package of the schema definition
It is responsible for creating a structure over the data, and pretends to perform validations before having to communicate
*/
package schema

import (
	"sync"

	"github.com/sebach1/rtc/integrity"
	"github.com/sebach1/rtc/internal/xerrors"
)

// The Schema is the representation of a Database instructive. It uses concepts of SQL.
// It provides the validation and construction structure.
type Schema struct {
	Id        int64                `json:"id,omitempty"`
	Name      integrity.SchemaName `json:"name,omitempty"`
	Blueprint []*Table             `json:"blueprint,omitempty"`
}

// ValidateSelf performs a deep self-validation to check data integrity
// It wraps internal method validateSelf
func (sch *Schema) ValidateSelf() (errs xerrors.MultiErr) {
	done := make(chan bool)
	validationErrs := make(chan error)
	go sch.validateSelf(done, validationErrs)
	for {
		select {
		case <-done:
			return
		case vErr := <-validationErrs:
			errs = append(errs, vErr)
		}
	}
}

func (sch *Schema) validateSelf(done chan<- bool, vErrCh chan<- error) {
	defer func() {
		done <- true
		close(vErrCh)
	}()

	if sch == nil {
		vErrCh <- sch.validationErr(errNilSchema)
		return
	}

	tablesQt := len(sch.Blueprint)
	if tablesQt == 0 {
		vErrCh <- sch.validationErr(errNilBlueprint)
	}

	var schVWg sync.WaitGroup
	schVWg.Add(tablesQt)
	for _, table := range sch.Blueprint {
		go table.validateSelf(&schVWg, vErrCh)
	}

	if sch.Name == "" {
		vErrCh <- sch.validationErr(errNilSchemaName)
	}

	schVWg.Wait()
}

func (sch *Schema) validationErr(err error) *xerrors.ValidationError {
	var name string
	if sch == nil {
		name = ""
	} else {
		name = string(sch.Name)
	}
	return &xerrors.ValidationError{Err: err, OriginType: "schema", OriginName: name}
}

// ValidateCtx checks if the context of the given tableName and colName is valid
// Notice that, as well as the wrapper validations should provoke a chained
// of undesired (and maybe more confusing than clear) errs, the errCh should be buffered w/sz=1
func (sch *Schema) ValidateCtx(
	tableName integrity.TableName,
	colName integrity.ColumnName,
	optionKeys []integrity.OptionKey,
	val interface{},
	helperScope *Planisphere,
	wg *sync.WaitGroup,
	errCh chan<- error,
) {
	defer wg.Done()

	table, err := sch.tableByName(tableName, helperScope)
	if err != nil {
		errCh <- err
		return
	}

	for _, key := range optionKeys {
		if !table.optionKeyIsValid(key) {
			errCh <- errInvalidOptionKey
			return
		}
	}

	if colName == "" {
		return
	}

	for _, col := range table.Columns {
		if colName == col.Name {
			err = col.Validate(val)
			if err != nil {
				errCh <- err
				return
			}

			return
		}
	}
	errCh <- sch.preciseColErr(colName)
}

func (t *Table) optionKeyIsValid(key integrity.OptionKey) bool {
	for _, validKey := range t.OptionKeys {
		if validKey == key {
			return true
		}
	}
	return false
}

func (sch *Schema) tableByName(tableName integrity.TableName, helperScope *Planisphere) (*Table, error) {
	for _, table := range sch.Blueprint {
		if tableName == table.Name {
			return table, nil
		}
	}
	return nil, helperScope.preciseTableErr(tableName)
}

// colNames plucks all the columnNames from its tables
func (sch *Schema) colNames() (colNames []integrity.ColumnName) {
	for _, table := range sch.Blueprint {
		for _, column := range table.Columns {
			colNames = append(colNames, column.Name)
		}
	}
	return
}

// tableNames plucks the name from its tables
func (sch *Schema) tableNames() (tableNames []integrity.TableName) {
	for _, table := range sch.Blueprint {
		tableNames = append(tableNames, table.Name)
	}
	return
}

// preciseColErr gives a more accurate error to a validation of a column
// It assumes the column is errored, and checks if it exists or if instead its a context err
func (sch *Schema) preciseColErr(colName integrity.ColumnName) (err error) {
	for _, column := range sch.colNames() {
		if column == colName {
			return errForeignColumn
		}
	}
	return errNonexistentColumn
}

// Wraps Column.applyBuiltinValidator() over all cols
func (sch *Schema) applyBuiltinValidators() (err error) {
	for _, table := range sch.Blueprint {
		for _, col := range table.Columns {
			err = col.applyBuiltinValidator()
			if err != nil {
				return
			}
		}
	}
	return nil
}
