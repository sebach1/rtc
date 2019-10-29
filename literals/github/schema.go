package github

import (
	"github.com/sebach1/git-crud/integrity"
	"github.com/sebach1/git-crud/schema"
	"github.com/sebach1/git-crud/schema/valide"
)

// GitHub is the hub of git
var GitHub = &schema.Schema{
	Name: "github",
	Blueprint: []*schema.Table{

		&schema.Table{
			Name: "repositories",
			Columns: []*schema.Column{
				&schema.Column{Name: "name", Validator: valide.String},
				&schema.Column{Name: "private", Validator: valide.String},
			},
			OptionKeys: []integrity.OptionKey{"username"},
		},
		&schema.Table{
			Name: "organizations",
			Columns: []*schema.Column{
				&schema.Column{Name: "name", Validator: valide.String},
				&schema.Column{Name: "projects", Validator: valide.Bytes},
			},
			OptionKeys: []integrity.OptionKey{"owner"},
		},
	},
}