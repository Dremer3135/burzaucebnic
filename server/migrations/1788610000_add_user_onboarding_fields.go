package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		usersColl, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		var changed bool
		if usersColl.Fields.GetByName("payoutToBank") == nil {
			usersColl.Fields.Add(&core.BoolField{
				Name: "payoutToBank",
			})
			changed = true
		}
		if usersColl.Fields.GetByName("iban") == nil {
			usersColl.Fields.Add(&core.TextField{
				Name: "iban",
			})
			changed = true
		}
		if usersColl.Fields.GetByName("onboardingComplete") == nil {
			usersColl.Fields.Add(&core.BoolField{
				Name: "onboardingComplete",
			})
			changed = true
		}
		if usersColl.UpdateRule == nil || *usersColl.UpdateRule != "@request.auth.id = id" {
			rule := "@request.auth.id = id"
			usersColl.UpdateRule = &rule
			changed = true
		}

		if changed {
			return app.Save(usersColl)
		}
		return nil
	}, func(app core.App) error {
		usersColl, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		var changed bool
		for _, name := range []string{"payoutToBank", "iban", "onboardingComplete"} {
			if usersColl.Fields.GetByName(name) != nil {
				usersColl.Fields.RemoveByName(name)
				changed = true
			}
		}

		if changed {
			return app.Save(usersColl)
		}
		return nil
	}, "1788610000_add_user_onboarding_fields.go")
}
