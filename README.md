nano-payment-server
===================

This is a server for processing NANO payments in conjunction with a node.

Install
-------

    go get -u github.com/hectorchu/nano-payment-server

Usage
-----

    -cb string
          Callback URL when payment is fulfilled
    -p int
          Listen port (default 8080)
    -pow string
          RPC Proof-of-Work URL
    -rpc string
          RPC URL (default "http://[::1]:7076")
    -ws string
          WebSocket URL (default "ws://[::1]:7078")

Method of operation
-------------------

The operator's regular server software (perhaps an e-commerce platform) will send a request to this server (`/new_payment`) with a JSON body containing the NANO `account` to receive on and the `amount` receivable. In response they will receive a payment `id`. The payment URL which should be sent to the payer will then be `/payment?id=<id>`. The payer's wallet should `POST` in JSON format a signed block (minus proof-of-work) to this URL. This server will then validate the block, calculate the proof-of-work and send the block on the network. The operator's server can be notified of successful payment via a callback URL.
