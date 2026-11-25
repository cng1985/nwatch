package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cng1985/nwatch/erp/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type API struct {
	DB *gorm.DB
}

func (a *API) Register(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		api.GET("/products", a.listProducts)
		api.GET("/products/:id", a.getProduct)
		api.POST("/products", a.createProduct)

		api.GET("/inventory", a.listInventory)
		api.GET("/inventory/movements", a.listMovements)

		api.GET("/orders", a.listOrders)
		api.GET("/orders/:id", a.getOrder)
		api.POST("/orders", a.createOrder)
		api.POST("/orders/:id/confirm", a.confirmOrder)
		api.POST("/orders/:id/fulfill", a.fulfillOrder)
	}
}

func (a *API) listProducts(c *gin.Context) {
	var items []models.Product
	if err := a.DB.Order("id asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (a *API) getProduct(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var p models.Product
	if err := a.DB.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

type createProductReq struct {
	SKU         string   `json:"sku" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	UnitPrice   *float64 `json:"unit_price" binding:"required"` // pointer: distinguish omitted vs 0
	InitialQty  int      `json:"initial_qty"`
}

func (a *API) createProduct(c *gin.Context) {
	var req createProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UnitPrice == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit_price is required"})
		return
	}
	if *req.UnitPrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit_price must be >= 0"})
		return
	}
	if req.InitialQty < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "initial_qty must be >= 0"})
		return
	}
	var product models.Product
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		product = models.Product{
			SKU:         req.SKU,
			Name:        req.Name,
			Description: req.Description,
			UnitPrice:   *req.UnitPrice,
		}
		if err := tx.Create(&product).Error; err != nil {
			return err
		}
		inv := models.Inventory{ProductID: product.ID, Quantity: req.InitialQty}
		if err := tx.Create(&inv).Error; err != nil {
			return err
		}
		if req.InitialQty > 0 {
			mv := models.StockMovement{
				ProductID: product.ID,
				Type:      "seed",
				Quantity:  req.InitialQty,
				Note:      "Initial stock on create",
			}
			return tx.Create(&mv).Error
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, product)
}

func (a *API) listInventory(c *gin.Context) {
	var items []models.Inventory
	if err := a.DB.Preload("Product").Order("id asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (a *API) listMovements(c *gin.Context) {
	var items []models.StockMovement
	q := a.DB.Preload("Product").Order("id desc").Limit(100)
	if pid := c.Query("product_id"); pid != "" {
		q = q.Where("product_id = ?", pid)
	}
	if err := q.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (a *API) listOrders(c *gin.Context) {
	var items []models.SalesOrder
	if err := a.DB.Preload("Lines").Preload("Lines.Product").Order("id desc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (a *API) getOrder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var order models.SalesOrder
	if err := a.DB.Preload("Lines").Preload("Lines.Product").First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

type createOrderLineReq struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required"`
}

type createOrderReq struct {
	CustomerName string               `json:"customer_name" binding:"required"`
	Notes        string               `json:"notes"`
	Lines        []createOrderLineReq `json:"lines" binding:"required,min=1"`
}

func (a *API) createOrder(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var order models.SalesOrder
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		order = models.SalesOrder{
			OrderNo:      nextOrderNo(tx),
			CustomerName: req.CustomerName,
			Status:       models.OrderStatusDraft,
			Notes:        req.Notes,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		var total float64
		for _, line := range req.Lines {
			if line.Quantity <= 0 {
				return fmt.Errorf("quantity must be > 0")
			}
			var p models.Product
			if err := tx.First(&p, line.ProductID).Error; err != nil {
				return fmt.Errorf("product %d not found", line.ProductID)
			}
			lt := float64(line.Quantity) * p.UnitPrice
			ol := models.SalesOrderLine{
				OrderID:   order.ID,
				ProductID: p.ID,
				Quantity:  line.Quantity,
				UnitPrice: p.UnitPrice,
				LineTotal: lt,
			}
			if err := tx.Create(&ol).Error; err != nil {
				return err
			}
			total += lt
		}
		order.TotalAmount = total
		return tx.Model(&order).Update("total_amount", total).Error
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_ = a.DB.Preload("Lines").Preload("Lines.Product").First(&order, order.ID)
	c.JSON(http.StatusCreated, order)
}

func (a *API) confirmOrder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var order models.SalesOrder
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Lines").First(&order, id).Error; err != nil {
			return err
		}
		if order.Status != models.OrderStatusDraft {
			return fmt.Errorf("only draft orders can be confirmed (status=%s)", order.Status)
		}
		if len(order.Lines) == 0 {
			return fmt.Errorf("order has no lines")
		}
		now := time.Now()
		order.Status = models.OrderStatusConfirmed
		order.ConfirmedAt = &now
		return tx.Model(&order).Updates(map[string]any{
			"status":       order.Status,
			"confirmed_at": order.ConfirmedAt,
		}).Error
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	_ = a.DB.Preload("Lines").Preload("Lines.Product").First(&order, order.ID)
	c.JSON(http.StatusOK, order)
}

func (a *API) fulfillOrder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var order models.SalesOrder
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Lines").First(&order, id).Error; err != nil {
			return err
		}
		if order.Status != models.OrderStatusConfirmed {
			return fmt.Errorf("only confirmed orders can be fulfilled (status=%s)", order.Status)
		}
		for _, line := range order.Lines {
			var inv models.Inventory
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("product_id = ?", line.ProductID).First(&inv).Error; err != nil {
				return fmt.Errorf("inventory missing for product %d", line.ProductID)
			}
			if inv.Quantity < line.Quantity {
				return fmt.Errorf("insufficient stock for product_id=%d (have %d, need %d)",
					line.ProductID, inv.Quantity, line.Quantity)
			}
			inv.Quantity -= line.Quantity
			if err := tx.Save(&inv).Error; err != nil {
				return err
			}
			oid := order.ID
			mv := models.StockMovement{
				ProductID: line.ProductID,
				OrderID:   &oid,
				Type:      "sale_out",
				Quantity:  -line.Quantity,
				Note:      fmt.Sprintf("Fulfill %s", order.OrderNo),
			}
			if err := tx.Create(&mv).Error; err != nil {
				return err
			}
		}
		now := time.Now()
		order.Status = models.OrderStatusFulfilled
		order.FulfilledAt = &now
		return tx.Model(&order).Updates(map[string]any{
			"status":       order.Status,
			"fulfilled_at": order.FulfilledAt,
		}).Error
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	_ = a.DB.Preload("Lines").Preload("Lines.Product").First(&order, order.ID)
	c.JSON(http.StatusOK, order)
}

func parseID(c *gin.Context) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id64), true
}

func nextOrderNo(tx *gorm.DB) string {
	prefix := time.Now().Format("20060102")
	var count int64
	_ = tx.Model(&models.SalesOrder{}).
		Where("order_no LIKE ?", "SO-"+prefix+"-%").
		Count(&count).Error
	return fmt.Sprintf("SO-%s-%03d", prefix, count+1)
}
