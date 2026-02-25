# Go Stripe MCP Server

An MCP server implemented in Go that wraps the Stripe API to simulate payment scenarios for AI agents.

## Prerequisites

- Go 1.22+
- A Stripe Secret Key (test mode)

## Installation

1. Initialize dependencies:
    ```bash
    go mod tidy
    ```

2. Build the server:
    ```bash
    go build -o stripe-mcp
    ```

## Usage

### Environment Variables

```bash
export STRIPE_SECRET_KEY=sk_test_...
```

### Running the Server

```bash
./stripe-mcp
```

### Tools

| Tool | Description |
|---|---|
| `create_payment` | Creates a Stripe PaymentIntent (card only). Returns the PI ID and Client Secret. |
| `mock_confirm_payment` | Confirms a PaymentIntent using a test PaymentMethod ID to simulate outcomes. |
| `get_payment` | Retrieves the current status and details of a PaymentIntent. |
| `cancel_payment` | Cancels a PaymentIntent. |
| `create_refund` | Issues a full or partial refund on a succeeded PaymentIntent. |

### Test PaymentMethod IDs for `mock_confirm_payment`

> **Note**: These are Stripe PaymentMethod test IDs (`pm_card_*`), not legacy source tokens (`tok_*`).

| `test_token` value | Simulated outcome |
|---|---|
| `pm_card_visa` | Success |
| `pm_card_chargeDeclined` | Generic decline |
| `pm_card_visa_chargeDeclinedInsufficientFunds` | Insufficient funds |
| `pm_card_cvcCheckFail` | CVC check failure |

### Example Flow

1. Create a payment:
   ```
   create_payment(amount="20.00", currency="usd", description="Test order")
   → PaymentIntent ID: pi_xxx
   ```

2. Simulate confirmation:
   ```
   mock_confirm_payment(payment_intent_id="pi_xxx", test_token="pm_card_visa")
   → Payment Confirmed Successfully. Status: succeeded
   ```

3. Check status:
   ```
   get_payment(payment_intent_id="pi_xxx")
   → Status: succeeded
   ```

4. Issue a refund:
   ```
   create_refund(payment_intent_id="pi_xxx", amount="10.00")
   → Refund Created. Status: succeeded
   ```
