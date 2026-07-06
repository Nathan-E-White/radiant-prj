use std::env;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::thread::sleep;
use std::time::Duration;

use serde_json::json;

#[derive(Debug, Clone)]
struct CliConfig {
    source_id: String,
    ingest_base_url: Option<String>,
    ingest_token: Option<String>,
    interval_ms: u64,
    max_frames: Option<u64>,
}

fn main() {
    if let Err(error) = run(env::args()) {
        eprintln!("scada-standins: {error}");
        std::process::exit(2);
    }
}

fn run<I>(args: I) -> scada_standins::Result<()>
where
    I: IntoIterator<Item = String>,
{
    let config = parse_args(args)?;
    if let Some(base_url) = config.ingest_base_url.as_deref() {
        let token = config.ingest_token.as_deref().ok_or_else(|| {
            "missing --ingest-token when --ingest-base-url is supplied".to_string()
        })?;
        let source = scada_standins::resident_source_declaration(&config.source_id);
        post_json(
            &source,
            &format!("{}/internal/scada/sources", base_url.trim_end_matches('/')),
            token,
        )?;
        let mut sequence = 1;
        loop {
            let frames = scada_standins::telemetry_frames(&config.source_id, sequence);
            post_json(
                &json!({ "frames": frames }),
                &format!("{}/internal/scada/telemetry", base_url.trim_end_matches('/')),
                token,
            )?;
            if config
                .max_frames
                .map(|max_frames| sequence >= max_frames)
                .unwrap_or(false)
            {
                break;
            }
            sequence += 1;
            sleep(Duration::from_millis(config.interval_ms));
        }
    } else {
        let frames = scada_standins::telemetry_frames(&config.source_id, 1);
        for frame in frames {
            println!("{}", serde_json::to_string(&frame)?);
        }
    }
    Ok(())
}

fn parse_args<I>(args: I) -> scada_standins::Result<CliConfig>
where
    I: IntoIterator<Item = String>,
{
    let mut source_id = "SRC-MIXED-STANDIN-001".to_string();
    let mut ingest_base_url = None;
    let mut ingest_token = None;
    let mut interval_ms = 1000;
    let mut max_frames = None;

    let mut args = args.into_iter();
    let _program = args.next();
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--source-id" => source_id = next_value(&mut args, "--source-id")?,
            "--ingest-base-url" => ingest_base_url = Some(next_value(&mut args, "--ingest-base-url")?),
            "--ingest-token" => ingest_token = Some(next_value(&mut args, "--ingest-token")?),
            "--interval-ms" => {
                interval_ms = next_value(&mut args, "--interval-ms")?
                    .parse()
                    .map_err(|_| "--interval-ms must be a positive integer".to_string())?;
                if interval_ms == 0 {
                    return Err("--interval-ms must be greater than zero".into());
                }
            }
            "--max-frames" => {
                let value: u64 = next_value(&mut args, "--max-frames")?
                    .parse()
                    .map_err(|_| "--max-frames must be a positive integer".to_string())?;
                if value == 0 {
                    return Err("--max-frames must be greater than zero".into());
                }
                max_frames = Some(value);
            }
            "--help" | "-h" => {
                println!("{}", usage());
                std::process::exit(0);
            }
            other => return Err(format!("unknown argument {other}\n{}", usage()).into()),
        }
    }

    Ok(CliConfig {
        source_id,
        ingest_base_url,
        ingest_token,
        interval_ms,
        max_frames,
    })
}

fn next_value(args: &mut impl Iterator<Item = String>, flag: &str) -> scada_standins::Result<String> {
    args.next()
        .ok_or_else(|| format!("{flag} requires a value").into())
}

fn usage() -> &'static str {
    "usage: scada-standins [--source-id <id>] [--ingest-base-url <url> --ingest-token <token>] [--interval-ms <n>] [--max-frames <n>]"
}

fn post_json<T: serde::Serialize>(payload: &T, url: &str, token: &str) -> scada_standins::Result<()> {
    let target = HttpTarget::parse(url)?;
    let body = serde_json::to_vec(payload)?;
    let mut stream = TcpStream::connect(&target.connect_addr)?;
    stream.set_read_timeout(Some(Duration::from_secs(10)))?;
    stream.set_write_timeout(Some(Duration::from_secs(10)))?;
    let request = format!(
        "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nAccept: application/json\r\nX-Workbench-Ingest-Token: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        target.path,
        target.host_header,
        token,
        body.len()
    );
    stream.write_all(request.as_bytes())?;
    stream.write_all(&body)?;
    stream.flush()?;

    let mut response = String::new();
    stream.read_to_string(&mut response)?;
    let status_line = response.lines().next().unwrap_or_default();
    if !status_line.starts_with("HTTP/1.1 2") && !status_line.starts_with("HTTP/1.0 2") {
        return Err(format!("ingest endpoint returned non-success status: {status_line}").into());
    }
    Ok(())
}

struct HttpTarget {
    connect_addr: String,
    host_header: String,
    path: String,
}

impl HttpTarget {
    fn parse(url: &str) -> scada_standins::Result<Self> {
        let without_scheme = url
            .strip_prefix("http://")
            .ok_or_else(|| "ingest URL must use http:// for local ingestion".to_string())?;
        let (host, path) = match without_scheme.split_once('/') {
            Some((host, path)) => (host, format!("/{path}")),
            None => (without_scheme, "/".to_string()),
        };
        if host.is_empty() {
            return Err("ingest URL host is required".into());
        }
        let connect_addr = if host.contains(':') {
            host.to_string()
        } else {
            format!("{host}:80")
        };
        Ok(Self {
            connect_addr,
            host_header: host.to_string(),
            path,
        })
    }
}
