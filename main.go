package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/refund"
)

// allowedTestTokens maps human-readable scenario names to Stripe test PaymentMethod IDs.
// Note: tok_* are legacy Charges API tokens; PaymentIntents require pm_card_* IDs.
var allowedTestTokens = map[string]bool{
	"pm_card_visa":                                      true,
	"pm_card_chargeDeclined":                            true,
	"pm_card_visa_chargeDeclinedInsufficientFunds":      true,
	"pm_card_cvcCheckFail":                              true,
}

func main() {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		fmt.Fprintf(os.Stderr, "Error: STRIPE_SECRET_KEY environment variable is required\n")
		os.Exit(1)
	}
	stripe.Key = stripeKey

	s := server.NewMCPServer(
		"Stripe Payment MCP",
		"1.0.0",
		server.WithLogging(),
	)

	s.AddTool(createPaymentTool(), handleCreatePayment)
	s.AddTool(mockConfirmTool(), handleMockConfirm)
	s.AddTool(getPaymentTool(), handleGetPayment)
	s.AddTool(cancelPaymentTool(), handleCancelPayment)
	s.AddTool(createRefundTool(), handleCreateRefund)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}

// --- Tool definitions ---

func createPaymentTool() mcp.Tool {
	return mcp.NewTool("create_payment",
		mcp.WithDescription("Creates a Stripe PaymentIntent to initiate a card payment flow. Returns the PaymentIntent ID and Client Secret."),
		mcp.WithString("amount", mcp.Description("Amount in major currency units (e.g. 10.50 for $10.50). Minimum 0.50. Will be converted to cents."), mcp.Required()),
		mcp.WithString("currency", mcp.Description("Three-letter ISO currency code (e.g. 'usd')."), mcp.Required()),
		mcp.WithString("description", mcp.Description("Description of the payment."), mcp.Required()),
	)
}

func mockConfirmTool() mcp.Tool {
	return mcp.NewTool("mock_confirm_payment",
		mcp.WithDescription(`Confirms a PaymentIntent using a Stripe test PaymentMethod ID to simulate specific outcomes.

Supported test PaymentMethod IDs:
- pm_card_visa                                   → Success
- pm_card_chargeDeclined                         → Generic decline
- pm_card_visa_chargeDeclinedInsufficientFunds   → Insufficient funds
- pm_card_cvcCheckFail                           → CVC check failure
`),
		mcp.WithString("payment_intent_id", mcp.Description("The ID of the PaymentIntent to confirm."), mcp.Required()),
		mcp.WithString("test_token", mcp.Description("Stripe test PaymentMethod ID (e.g. pm_card_visa) to simulate the scenario."), mcp.Required()),
	)
}

func getPaymentTool() mcp.Tool {
	return mcp.NewTool("get_payment",
		mcp.WithDescription("Retrieves the current status and details of a Stripe PaymentIntent."),
		mcp.WithString("payment_intent_id", mcp.Description("The ID of the PaymentIntent to retrieve."), mcp.Required()),
	)
}

func cancelPaymentTool() mcp.Tool {
	return mcp.NewTool("cancel_payment",
		mcp.WithDescription("Cancels a Stripe PaymentIntent. Only intents in 'requires_payment_method', 'requires_capture', 'requires_confirmation', or 'requires_action' status can be cancelled."),
		mcp.WithString("payment_intent_id", mcp.Description("The ID of the PaymentIntent to cancel."), mcp.Required()),
	)
}

func createRefundTool() mcp.Tool {
	return mcp.NewTool("create_refund",
		mcp.WithDescription("Refunds a succeeded Stripe PaymentIntent, fully or partially."),
		mcp.WithString("payment_intent_id", mcp.Description("The ID of the succeeded PaymentIntent to refund."), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to refund in major currency units (e.g. 5.00). Omit for a full refund."), mcp.Required()),
	)
}

// --- Handlers ---

func handleCreatePayment(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	amountStr, ok := request.Params.Arguments["amount"].(string)
	if !ok {
		return mcp.NewToolResultError("amount must be a string"), nil
	}
	var amountFloat float64
	if _, err := fmt.Sscanf(amountStr, "%f", &amountFloat); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid amount format: %v", err)), nil
	}
	if amountFloat < 0.50 {
		return mcp.NewToolResultError("amount must be at least 0.50"), nil
	}
	amountCents := int64(amountFloat * 100)

	currency, ok := request.Params.Arguments["currency"].(string)
	if !ok {
		return mcp.NewToolResultError("currency must be a string"), nil
	}
	if len(currency) != 3 {
		return mcp.NewToolResultError("currency must be a 3-letter ISO code (e.g. 'usd')"), nil
	}

	description, ok := request.Params.Arguments["description"].(string)
	if !ok {
		return mcp.NewToolResultError("description must be a string"), nil
	}

	params := &stripe.PaymentIntentParams{
		Amount:             stripe.Int64(amountCents),
		Currency:           stripe.String(strings.ToLower(currency)),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Description:        stripe.String(description),
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stripe error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"PaymentIntent Created:\nID: %s\nClient Secret: %s\nStatus: %s",
		pi.ID, pi.ClientSecret, pi.Status,
	)), nil
}

func handleMockConfirm(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	piID, ok := request.Params.Arguments["payment_intent_id"].(string)
	if !ok {
		return mcp.NewToolResultError("payment_intent_id must be a string"), nil
	}

	token, ok := request.Params.Arguments["test_token"].(string)
	if !ok {
		return mcp.NewToolResultError("test_token must be a string"), nil
	}
	if !allowedTestTokens[token] {
		return mcp.NewToolResultError(fmt.Sprintf(
			"unsupported test_token %q — use one of: pm_card_visa, pm_card_chargeDeclined, pm_card_visa_chargeDeclinedInsufficientFunds, pm_card_cvcCheckFail",
			token,
		)), nil
	}

	confirmParams := &stripe.PaymentIntentConfirmParams{
		PaymentMethod: stripe.String(token),
	}

	pi, err := paymentintent.Confirm(piID, confirmParams)
	if err != nil {
		// Return stripe error as text so the AI can inspect the decline reason
		return mcp.NewToolResultText(fmt.Sprintf("Stripe Error (Scenario Triggered): %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"Payment Confirmed Successfully.\nID: %s\nStatus: %s",
		pi.ID, pi.Status,
	)), nil
}

func handleGetPayment(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	piID, ok := request.Params.Arguments["payment_intent_id"].(string)
	if !ok {
		return mcp.NewToolResultError("payment_intent_id must be a string"), nil
	}

	pi, err := paymentintent.Get(piID, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stripe error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"PaymentIntent:\nID: %s\nAmount: %d cents\nCurrency: %s\nStatus: %s\nDescription: %s",
		pi.ID, pi.Amount, pi.Currency, pi.Status, pi.Description,
	)), nil
}

func handleCancelPayment(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	piID, ok := request.Params.Arguments["payment_intent_id"].(string)
	if !ok {
		return mcp.NewToolResultError("payment_intent_id must be a string"), nil
	}

	pi, err := paymentintent.Cancel(piID, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stripe error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"PaymentIntent Cancelled.\nID: %s\nStatus: %s",
		pi.ID, pi.Status,
	)), nil
}

func handleCreateRefund(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	piID, ok := request.Params.Arguments["payment_intent_id"].(string)
	if !ok {
		return mcp.NewToolResultError("payment_intent_id must be a string"), nil
	}

	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(piID),
	}

	// Optional partial refund amount
	if amountStr, ok := request.Params.Arguments["amount"].(string); ok && amountStr != "" {
		var amountFloat float64
		if _, err := fmt.Sscanf(amountStr, "%f", &amountFloat); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount format: %v", err)), nil
		}
		if amountFloat < 0.50 {
			return mcp.NewToolResultError("refund amount must be at least 0.50"), nil
		}
		params.Amount = stripe.Int64(int64(amountFloat * 100))
	}

	r, err := refund.New(params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stripe error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"Refund Created.\nID: %s\nAmount: %d cents\nStatus: %s",
		r.ID, r.Amount, r.Status,
	)), nil
}
