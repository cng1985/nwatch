package models

import "time"

type Product struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	SKU         string    `json:"sku" gorm:"uniqueIndex;size:64;not null"`
	Name        string    `json:"name" gorm:"size:200;not null"`
	Description string    `json:"description" gorm:"size:500"`
	UnitPrice   float64   `json:"unit_price" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Inventory struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ProductID uint      `json:"product_id" gorm:"uniqueIndex;not null"`
	Product   *Product  `json:"product,omitempty" gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Quantity  int       `json:"quantity" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StockMovement struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ProductID uint      `json:"product_id" gorm:"index;not null"`
	Product   *Product  `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	OrderID   *uint     `json:"order_id,omitempty" gorm:"index"`
	Type      string    `json:"type" gorm:"size:32;not null"` // seed, adjust, sale_out
	Quantity  int       `json:"quantity" gorm:"not null"`     // signed delta
	Note      string    `json:"note" gorm:"size:300"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	OrderStatusDraft     = "draft"
	OrderStatusConfirmed = "confirmed"
	OrderStatusFulfilled = "fulfilled"
	OrderStatusCancelled = "cancelled"
)

type SalesOrder struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	OrderNo      string           `json:"order_no" gorm:"uniqueIndex;size:32;not null"`
	CustomerName string           `json:"customer_name" gorm:"size:120;not null"`
	Status       string           `json:"status" gorm:"size:32;not null;index"`
	TotalAmount  float64          `json:"total_amount" gorm:"not null;default:0"`
	Notes        string           `json:"notes" gorm:"size:300"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	ConfirmedAt  *time.Time       `json:"confirmed_at,omitempty"`
	FulfilledAt  *time.Time       `json:"fulfilled_at,omitempty"`
	Lines        []SalesOrderLine `json:"lines,omitempty" gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type SalesOrderLine struct {
	ID        uint     `json:"id" gorm:"primaryKey"`
	OrderID   uint     `json:"order_id" gorm:"index;not null"`
	ProductID uint     `json:"product_id" gorm:"index;not null"`
	Product   *Product `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	Quantity  int      `json:"quantity" gorm:"not null"`
	UnitPrice float64  `json:"unit_price" gorm:"not null"`
	LineTotal float64  `json:"line_total" gorm:"not null"`
}
