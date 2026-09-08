package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		eventsColl, err := app.FindCollectionByNameOrId("events")
		if err != nil {
			return nil
		}

		if eventsColl.Fields.GetByName("maintenance_break") == nil {
			eventsColl.Fields.Add(&core.BoolField{
				Name: "maintenance_break",
			})
			return app.Save(eventsColl)
		}

		return nil
	}, func(app core.App) error {
		eventsColl, err := app.FindCollectionByNameOrId("events")
		if err != nil {
			return nil
		}

		if eventsColl.Fields.GetByName("maintenance_break") != nil {
			eventsColl.Fields.RemoveByName("maintenance_break")
			return app.Save(eventsColl)
		}

		return nil
	}, "1788620000_add_maintenance_break_to_events.go")
}
