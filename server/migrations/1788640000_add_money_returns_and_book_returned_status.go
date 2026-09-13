package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	core.AppMigrations.Register(func(app core.App) error {
		usersColl, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return nil
		}

		eventsColl, err := app.FindCollectionByNameOrId("events")
		if err != nil {
			return nil
		}

		booksColl, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return nil
		}

		// 1. Create money_returns collection
		moneyReturnsColl, _ := app.FindCollectionByNameOrId("money_returns")
		if moneyReturnsColl == nil {
			moneyReturnsColl = core.NewBaseCollection("money_returns")
			moneyReturnsColl.ListRule = types.Pointer("@request.auth.isCashier = true || @request.auth.id = seller.id")
			moneyReturnsColl.ViewRule = types.Pointer("@request.auth.isCashier = true || @request.auth.id = seller.id")
			moneyReturnsColl.CreateRule = types.Pointer("@request.auth.isCashier = true")
			moneyReturnsColl.UpdateRule = types.Pointer("@request.auth.isCashier = true")
			moneyReturnsColl.DeleteRule = types.Pointer("@request.auth.isCashier = true")

			moneyReturnsColl.Fields.Add(
				&core.RelationField{
					Name:         "seller",
					Required:     true,
					CollectionId: usersColl.Id,
					MaxSelect:    1,
				},
				&core.RelationField{
					Name:         "event",
					Required:     true,
					CollectionId: eventsColl.Id,
					MaxSelect:    1,
				},
				&core.NumberField{
					Name:     "amount",
					Required: true,
				},
				&core.SelectField{
					Name:      "method",
					Required:  true,
					Values:    []string{"cash", "bank"},
					MaxSelect: 1,
				},
				&core.TextField{
					Name: "fio_transaction_id",
				},
				&core.TextField{
					Name: "variableSymbol",
				},
				&core.RelationField{
					Name:         "cashier",
					CollectionId: usersColl.Id,
					MaxSelect:    1,
				},
				&core.AutodateField{Name: "created", OnCreate: true},
				&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
			)

			if err := app.Save(moneyReturnsColl); err != nil {
				return err
			}
		}

		// 2. Add "returned" to books status options
		statusField := booksColl.Fields.GetByName("status")
		if statusSelect, ok := statusField.(*core.SelectField); ok {
			hasReturned := false
			for _, val := range statusSelect.Values {
				if val == "returned" {
					hasReturned = true
					break
				}
			}
			if !hasReturned {
				statusSelect.Values = append(statusSelect.Values, "returned")
				if err := app.Save(booksColl); err != nil {
					return err
				}
			}
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete money_returns
		moneyReturnsColl, _ := app.FindCollectionByNameOrId("money_returns")
		if moneyReturnsColl != nil {
			_ = app.Delete(moneyReturnsColl)
		}

		// Revert books status options
		booksColl, err := app.FindCollectionByNameOrId("books")
		if err == nil && booksColl != nil {
			statusField := booksColl.Fields.GetByName("status")
			if statusSelect, ok := statusField.(*core.SelectField); ok {
				newValues := make([]string, 0, len(statusSelect.Values))
				for _, val := range statusSelect.Values {
					if val != "returned" {
						newValues = append(newValues, val)
					}
				}
				statusSelect.Values = newValues
				_ = app.Save(booksColl)
			}
		}

		return nil
	}, "1788640000_add_money_returns_and_book_returned_status.go")
}
