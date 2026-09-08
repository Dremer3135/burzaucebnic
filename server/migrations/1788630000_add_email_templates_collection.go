package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		coll, _ := app.FindCollectionByNameOrId("email_templates")
		if coll != nil {
			return nil
		}

		coll = core.NewBaseCollection("email_templates")
		coll.ListRule = types.Pointer("@request.auth.isCashier = true")
		coll.ViewRule = types.Pointer("@request.auth.isCashier = true")
		coll.CreateRule = types.Pointer("@request.auth.isCashier = true")
		coll.UpdateRule = types.Pointer("@request.auth.isCashier = true")
		coll.DeleteRule = types.Pointer("@request.auth.isCashier = true")

		coll.Fields.Add(
			&core.TextField{Name: "key", Required: true},
			&core.TextField{Name: "name", Required: true},
			&core.TextField{Name: "subject", Required: true},
			&core.TextField{Name: "bodyIntro"},
			&core.TextField{Name: "unacceptedWarning"},
			&core.TextField{Name: "payoutBankNote"},
			&core.TextField{Name: "payoutCashNote"},
			&core.TextField{Name: "bodyOutro"},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)

		return app.Save(coll)
	}, func(app core.App) error {
		coll, _ := app.FindCollectionByNameOrId("email_templates")
		if coll != nil {
			return app.Delete(coll)
		}
		return nil
	}, "1788630000_add_email_templates_collection.go")
}
