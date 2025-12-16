package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
)

func main() {
	// 1. Load STRIPE_SECRET_KEY
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		fmt.Fprintf(os.Stderr, "Error: STRIPE_SECRET_KEY environment variable is required\n")
		os.Exit(1)
	}
	stripe.Key = stripeKey

	// 2. Initialize MCP Server
	s := server.NewMCPServer(
		"Stripe Payment MCP",
		"1.0.0",
		server.WithLogging(),
	)

	// 3. Register Tool: create_payment
	createPaymentTool := mcp.NewTool("create_payment",
		mcp.WithDescription("Initiates a Stripe PaymentIntent. Use this to start a payment flow. Returns the PaymentIntent ID and Client Secret."),
		mcp.WithString("amount", mcp.Description("Amount in major currency units (e.g. 10.50 for $10.50). Will be converted to cents."), mcp.Required()),
		mcp.WithString("currency", mcp.Description("Three-letter ISO currency code (e.g. 'usd')."), mcp.Required()),
		mcp.WithString("description", mcp.Description("Description of the payment."), mcp.Required()),
	)

	s.AddTool(createPaymentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		amountStr, ok := request.Arguments["amount"].(string)
		if !ok {
			return mcp.NewToolResultError("amount must be a string"), nil
		}
		// Basic parsing to float then cents
		// Note: Robust production code would use decimal library to avoid float errors,
		// but for simulation float64 is acceptable as requested.
		var amountFloat float64
		if _, err := fmt.Sscanf(amountStr, "%f", &amountFloat); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount format: %v", err)), nil
		}
		amountCents := int64(amountFloat * 100)

		currency, ok := request.Arguments["currency"].(string)
		if !ok {
			return mcp.NewToolResultError("currency must be a string"), nil
		}
		description, ok := request.Arguments["description"].(string)
		if !ok {
			return mcp.NewToolResultError("description must be a string"), nil
		}

		params := &stripe.PaymentIntentParams{
			Amount:   stripe.Int64(amountCents),
			Currency: stripe.String(currency),
			AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
				Enabled: stripe.Bool(true),
			},
			Description: stripe.String(description),
		}

		pi, err := paymentintent.New(params)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("stripe error: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("PaymentIntent Created:\nID: %s\nClient Secret: %s\nStatus: %s", pi.ID, pi.ClientSecret, pi.Status)), nil
	})

	// 4. Register Tool: mock_confirm_payment
	mockConfirmTool := mcp.NewTool("mock_confirm_payment",
		mcp.WithDescription(`Simulates the client-side confirmation step using Stripe test tokens to force specific outcomes.
Supported Magic Tokens:
- tok_visa (Success)
- tok_chargeDeclined (Generic Decline)
- tok_insufficientFunds (Insufficient Funds)
- tok_cvcCheckFail (CVC Error)
`),
		mcp.WithString("payment_intent_id", mcp.Description("The ID of the PaymentIntent to confirm."), mcp.Required()),
		mcp.WithString("test_token", mcp.Description("Stripe test token (e.g. tok_visa, tok_chargeDeclined) to simulate the scenario."), mcp.Required()),
	)

	s.AddTool(mockConfirmTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		piID, ok := request.Arguments["payment_intent_id"].(string)
		if !ok {
			return mcp.NewToolResultError("payment_intent_id must be a string"), nil
		}
		token, ok := request.Arguments["test_token"].(string)
		if !ok {
			return mcp.NewToolResultError("test_token must be a string"), nil
		}

		// To "mock" confirmation server-side with a token (which usually represents a card),
		// we update the PaymentIntent with the payment_method data method.
		// NOTE: In a real client-side flow, 'confirm' happens on the frontend.
		// Server-side confirmation requires a PaymentMethod ID usually.
		// However, for test tokens like 'tok_visa', we can attach it as a 'payment_method_data' 
		// or create a PaymentMethod from it first.
		// But Stripe's Confirm endpoint accepts `payment_method` as an ID or `payment_method_data`.
		// Using 'tok_...' directly often works in legacy flows or as source, but for PaymentIntents 
		// we often need to be careful. The prompt says: "The test_token argument must be passed to Stripe's PaymentMethod param."
		
		confirmParams := &stripe.PaymentIntentConfirmParams{
			PaymentMethod: stripe.String(token), // Passing token directly as requested
		}

		pi, err := paymentintent.Confirm(piID, confirmParams)
		if err != nil {
			// Return the stripe error message so the AI can see "Declined", etc.
			return mcp.NewToolResultText(fmt.Sprintf("Stripe Error (Scenario Triggered): %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Payment Confirmed Successfully.\nID: %s\nStatus: %s", pi.ID, pi.Status)), nil
	})

	// 5. Serve
	if err := s.ServeStdio(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}
