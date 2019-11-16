package git

import (
	"reflect"
	"sort"
	"testing"

	"github.com/gedex/inflector"
	"github.com/google/go-cmp/cmp"
	"github.com/sebach1/git-crud/internal/name"
)

func TestCommit_columns(t *testing.T) {
	comm := Commit{}
	exclusions := []string{"Reviewer", "Changes"}
	typeOf := reflect.TypeOf(comm)
	var want []string
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if isExcluded(exclusions, field.Name) {
			continue
		}
		col := name.ToSnakeCase(field.Name)
		want = append(want, col)
	}
	sort.Strings(want)

	got := comm.columns()
	sort.Strings(got)
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Commit.columns() mismatch (-want +got): %s", diff)
	}
}

func TestCommit_table(t *testing.T) {
	comm := Commit{}
	typeOf := reflect.TypeOf(comm)
	want := inflector.Pluralize(name.ToSnakeCase(typeOf.Name()))
	got := comm.table()
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Branch.table() mismatch (-want +got): %s", diff)
	}
}
