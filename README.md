# Go Stripe MCP Server

An MCP server implemented in Go that wraps the Stripe API to simulate payment scenarios for AI agents.

## Prerequisites

- Go 1.22+
- a Stripe Secret Key (test mode)

## Installation

1.  Initialize dependencies:
    ```bash
    go mod tidy
    ```

2.  Build the server:
    ```bash
    go build -o stripe-mcp
    ```

## Usage

### Environment Variables

You must set the `STRIPE_SECRET_KEY` environment variable.

```bash
export STRIPE_SECRET_KEY=sk_test_...
```

### Running the Server

Run the server using stdio:

```bash
./stripe-mcp
```

### Tools

-   `create_payment`: Creates a PaymentIntent.
-   `mock_confirm_payment`: Confirms a PaymentIntent using a test token to simulate scenarios (Success, Declined, etc.).
