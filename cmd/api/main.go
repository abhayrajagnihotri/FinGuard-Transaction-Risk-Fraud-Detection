package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type Transaction struct {
	ID            string    `json:"id"`
	CustomerID    string    `json:"customer_id"`
	AmountMinor   int64     `json:"amount_minor"`
	Currency      string    `json:"currency"`
	MerchantID    string    `json:"merchant_id"`
	CountryCode   string    `json:"country_code"`
	PaymentMethod string    `json:"payment_method"`
	RiskScore     int       `json:"risk_score"`
	RiskDecision  string    `json:"risk_decision"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateTransactionRequest struct {
	CustomerID    string `json:"customer_id" binding:"required"`
	AmountMinor   int64  `json:"amount_minor" binding:"required,gt=0"`
	Currency      string `json:"currency" binding:"required,len=3"`
	MerchantID    string `json:"merchant_id" binding:"required"`
	CountryCode   string `json:"country_code" binding:"required,len=2"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

func main() {
	// .env is optional; deployment environments can set variables directly.
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("database configuration failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	api := router.Group("/api/v1")
	{
		api.POST("/transactions", func(c *gin.Context) {
			var req CreateTransactionRequest

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid transaction request",
				})
				return
			}

			req.CustomerID = strings.TrimSpace(req.CustomerID)
			req.MerchantID = strings.TrimSpace(req.MerchantID)
			req.Currency = strings.ToUpper(
				strings.TrimSpace(req.Currency),
			)
			req.CountryCode = strings.ToUpper(
				strings.TrimSpace(req.CountryCode),
			)
			req.PaymentMethod = strings.ToLower(
				strings.TrimSpace(req.PaymentMethod),
			)

			if req.CustomerID == "" ||
				req.MerchantID == "" ||
				req.PaymentMethod == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "customer, merchant and payment method are required",
				})
				return
			}

			var tx Transaction

			err := db.QueryRow(
				c.Request.Context(),
				`INSERT INTO transactions (
					customer_id, amount_minor, currency,
					merchant_id, country_code, payment_method
				)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id, customer_id, amount_minor,
					currency, merchant_id, country_code,
					payment_method, risk_score, risk_decision,
					created_at`,
				req.CustomerID,
				req.AmountMinor,
				req.Currency,
				req.MerchantID,
				req.CountryCode,
				req.PaymentMethod,
			).Scan(
				&tx.ID,
				&tx.CustomerID,
				&tx.AmountMinor,
				&tx.Currency,
				&tx.MerchantID,
				&tx.CountryCode,
				&tx.PaymentMethod,
				&tx.RiskScore,
				&tx.RiskDecision,
				&tx.CreatedAt,
			)

			if err != nil {
				log.Printf("create transaction failed: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "could not create transaction",
				})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message": "transaction created",
				"data":    tx,
			})
		})

		api.GET("/transactions", func(c *gin.Context) {
			limit := 50

			if rawLimit := c.Query("limit"); rawLimit != "" {
				parsed, err := strconv.Atoi(rawLimit)
				if err != nil || parsed < 1 || parsed > 100 {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "limit must be between 1 and 100",
					})
					return
				}
				limit = parsed
			}

			rows, err := db.Query(
				c.Request.Context(),
				`SELECT id, customer_id, amount_minor,
					currency, merchant_id, country_code,
					payment_method, risk_score, risk_decision,
					created_at
				FROM transactions
				ORDER BY created_at DESC, id DESC
				LIMIT $1`,
				limit,
			)
			if err != nil {
				log.Printf("list transactions failed: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "could not fetch transactions",
				})
				return
			}
			defer rows.Close()

			transactions := make([]Transaction, 0)

			for rows.Next() {
				var tx Transaction

				if err := rows.Scan(
					&tx.ID,
					&tx.CustomerID,
					&tx.AmountMinor,
					&tx.Currency,
					&tx.MerchantID,
					&tx.CountryCode,
					&tx.PaymentMethod,
					&tx.RiskScore,
					&tx.RiskDecision,
					&tx.CreatedAt,
				); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": "could not read transactions",
					})
					return
				}

				transactions = append(transactions, tx)
			}

			if err := rows.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "could not read transactions",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"count": len(transactions),
				"data":  transactions,
			})
		})
	}

	log.Printf("FinGuard API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
