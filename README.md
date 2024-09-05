# Amount Watcher

## Steps to setup

1. Clone the Repository
2. Configure `AddressesAndChain.toml`

```toml
[[addresses]]
address = "<ADDRESS>"
rpc_url = "<RPC_URL>"
chain = "<CHAIN_NAME>"
alert_balance = 400
```

- `alert_balance` : `sat` for BTC or `Allouy::Unit<256,4>` for EVM chains

3. Create a `.env` File with

```
WEBHOOK=<DISCORD_WEBHOOK_URL>
```
- **WEBHOOK**: From Discord > `Channel Settings` > `Integrations` > `Webhooks`.\

4. Run the Application

```bash
cargo run --release
```
