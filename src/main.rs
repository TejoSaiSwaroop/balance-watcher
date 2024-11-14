use std::env;
use std::str::FromStr;
use std::time::Duration;

use alloy::primitives::Address;
use alloy::providers::{Provider, ProviderBuilder};
use dotenv::dotenv;
use serde::{Deserialize, Serialize};
use serenity::builder::ExecuteWebhook;
use serenity::http::Http;
use serenity::model::webhook::Webhook;
use std::fs::File;
use std::io::Read;
use tracing::{error, info};

const MEMPOOL_API: &str = "https://mempool.space/testnet/api/address";

#[derive(Debug, Deserialize)]
struct AddressAndChain {
    address: String,
    chain: String,
    rpc_url: String,
    alert_balance: u64,
}

#[derive(Debug, Deserialize)]
struct Config {
    addresses: Vec<AddressAndChain>,
}

#[derive(Debug, Serialize, Deserialize)]
struct BitcoinResponse {
    chain_stats: BitcoinDetails,
}

#[derive(Debug, Serialize, Deserialize)]
struct BitcoinDetails {
    funded_txo_count: u64,
    funded_txo_sum: u64,
    spent_txo_count: u64,
    spent_txo_sum: u64,
    tx_count: u64,
}

fn get_config() -> Config {
    let mut file = File::open("AddressChainAndBalance.toml").unwrap();
    let mut contents = String::new();
    file.read_to_string(&mut contents).unwrap();
    let config: Config = toml::from_str(&contents).unwrap();

    return config;
}

#[tokio::main]
async fn main() {
    dotenv().ok();
    tracing_subscriber::fmt().init();

    info!("Starting Main");

    loop {
        info!("*************************");
        ping().await;
        tokio::time::sleep(Duration::from_secs(60)).await;
    }
}

async fn ping() {
    let config = get_config();
    for detail in config.addresses {
        let client = reqwest::Client::new();
        let address = detail.address;

        match detail.chain.as_str() {
            "Bitcoin" => match get_bitcoin_balance(client, &address).await {
                Ok(bitcoin_balance) => {
                    info!("Bitcoin Balance: {}", format_crypto_balance(bitcoin_balance, "Bitcoin"));
                    if bitcoin_balance < detail.alert_balance {
                        send_result(&format!(
                            "Bitcoin balance low: {} Address: {}", 
                            format_crypto_balance(bitcoin_balance, "Bitcoin"),
                            address
                        )).await;
                    }
                }
                Err(e) => {
                    error!("Failed to fetch Bitcoin balance: {}", e);
                }
            },
            chain => match get_evm_balance(&address, &detail.rpc_url).await {
                Ok(evm_balance) => {
                    info!("{} Balance: {} Address: {}", 
                        chain,
                        format_crypto_balance(evm_balance, chain),
                        address
                    );
                    if evm_balance < detail.alert_balance {
                        send_result(&format!(
                            "{} Balance low: {} Address: {}", 
                            chain,
                            format_crypto_balance(evm_balance, chain),
                            address
                        )).await;
                    }
                }
                Err(e) => {
                    error!("Failed to fetch {} balance: {} address: {}", chain, e, address);
                }
            },
        }
    }
}

async fn get_evm_balance(address: &str, rpc_url: &str) -> Result<u64, String> {
    let provider = ProviderBuilder::new().on_http(
        reqwest::Url::from_str(&rpc_url).map_err(|e| format!("Failed to parse RPC URL: {}", e))?,
    );

    let address =
        Address::from_str(address).map_err(|e| format!("Invalid Ethereum address: {}", e))?;

    let balance = provider
        .get_balance(address)
        .await
        .map_err(|e| format!("Failed to fetch {} balance: {}", "EVM", e))?;

    balance
        .to_string()
        .parse::<u64>()
        .map_err(|e| format!("Failed to parse balance as u64: {}", e))
}

async fn get_bitcoin_balance(client: reqwest::Client, address: &str) -> Result<u64, String> {
    let api_url = format!("{}/{}", MEMPOOL_API, address);

    let response = client
        .get(api_url)
        .send()
        .await
        .map_err(|e| format!("Request failed: {}", e))?;

    let response_body = response
        .text()
        .await
        .map_err(|e| format!("Failed to read response body as text: {}", e))?;

    let val: BitcoinResponse = serde_json::from_str(&response_body)
        .map_err(|e| format!("Failed to parse response as BitcoinResponse: {}", e))?;

    Ok(val.chain_stats.funded_txo_sum - val.chain_stats.spent_txo_sum)
}

const SATOSHI_TO_BTC: f64 = 100_000_000.0; 
const WEI_TO_ETH: f64 = 1_000_000_000_000_000_000.0; 

fn get_conversion_factor(chain: &str) -> f64 {
    match chain {
        "BTC" | "Bitcoin" => SATOSHI_TO_BTC,
        "ETH" | "Ethereum" | "Arbitrum" | "Sepolia" | "Base" => WEI_TO_ETH,
        _ => WEI_TO_ETH
    }
}

fn format_crypto_balance(balance: u64, chain: &str) -> String {
    let conversion_factor = get_conversion_factor(chain);
    let converted_balance = balance as f64 / conversion_factor;
    
    let symbol = match chain {
        "BTC" | "Bitcoin" => "BTC",
        _ => "ETH"
    };
    
    format!("{:.4} {}", converted_balance, symbol)
}

async fn send_result(data: &str) {
    let http = Http::new("");
    let webhook = Webhook::from_url(&http, &env::var("WEBHOOK").expect("Should be set"))
        .await
        .expect("Replace the webhook with your own");

    let formatted_data = format!("`{}`", data);

    let builder = ExecuteWebhook::new()
        .content(formatted_data)
        .username("Balance Watcher");

    webhook
        .execute(&http, false, builder)
        .await
        .expect("Could not execute webhook.");
}
