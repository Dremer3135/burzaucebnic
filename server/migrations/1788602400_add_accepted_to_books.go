package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		booksColl, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}

		if booksColl.Fields.GetByName("accepted") == nil {
			booksColl.Fields.Add(&core.BoolField{
				Name: "accepted",
			})
			return app.Save(booksColl)
		}

		return nil
	}, func(app core.App) error {
		booksColl, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}

		if booksColl.Fields.GetByName("accepted") != nil {
			booksColl.Fields.RemoveByName("accepted")
			return app.Save(booksColl)
		}

		return nil
	})
}
