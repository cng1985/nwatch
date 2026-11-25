package seed

import (
	"fmt"
	"time"

	"github.com/cng1985/nwatch/erp/backend/internal/models"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Product{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	products := []models.Product{
		{SKU: "SKU-NOTEBOOK-A5", Name: "A5 Lined Notebook", Description: "80-page softcover notebook", UnitPrice: 12.50},
		{SKU: "SKU-PEN-BLUE", Name: "Blue Gel Pen (12pk)", Description: "Smooth 0.5mm gel pens", UnitPrice: 8.90},
		{SKU: "SKU-MUG-CERAMIC", Name: "Ceramic Mug 350ml", Description: "Matte white office mug", UnitPrice: 15.00},
		{SKU: "SKU-USB-C-HUB", Name: "USB-C Hub 7-in-1", Description: "HDMI + SD + USB-A ports", UnitPrice: 49.00},
		{SKU: "SKU-DESK-MAT", Name: "Desk Mat XL", Description: "Waterproof desk pad 80x40cm", UnitPrice: 22.00},
	}

	stocks := map[string]int{
		"SKU-NOTEBOOK-A5": 120,
		"SKU-PEN-BLUE":    80,
		"SKU-MUG-CERAMIC": 45,
		"SKU-USB-C-HUB":   30,
		"SKU-DESK-MAT":    55,
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for i := range products {
			p := &products[i]
			if err := tx.Create(p).Error; err != nil {
				return err
			}
			qty := stocks[p.SKU]
			inv := models.Inventory{ProductID: p.ID, Quantity: qty}
			if err := tx.Create(&inv).Error; err != nil {
				return err
			}
			mv := models.StockMovement{
				ProductID: p.ID,
				Type:      "seed",
				Quantity:  qty,
				Note:      "Initial stock for demo",
			}
			if err := tx.Create(&mv).Error; err != nil {
				return err
			}
		}

		// Sample draft order
		now := time.Now()
		draft := models.SalesOrder{
			OrderNo:      fmt.Sprintf("SO-%s-001", now.Format("20060102")),
			CustomerName: "Acme Office Supplies",
			Status:       models.OrderStatusDraft,
			Notes:        "Demo draft — confirm then fulfill",
		}
		if err := tx.Create(&draft).Error; err != nil {
			return err
		}
		p0, p1 := products[0], products[1]
		lines := []models.SalesOrderLine{
			{OrderID: draft.ID, ProductID: p0.ID, Quantity: 10, UnitPrice: p0.UnitPrice, LineTotal: float64(10) * p0.UnitPrice},
			{OrderID: draft.ID, ProductID: p1.ID, Quantity: 5, UnitPrice: p1.UnitPrice, LineTotal: float64(5) * p1.UnitPrice},
		}
		var total float64
		for _, l := range lines {
			total += l.LineTotal
			if err := tx.Create(&l).Error; err != nil {
				return err
			}
		}
		return tx.Model(&draft).Update("total_amount", total).Error
	})
}
