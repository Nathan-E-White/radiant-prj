use std::env;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::thread::sleep;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use serde_json::json;

#[derive(Debug, Clone)]
struct CliConfig {
    source_id: String,
    reactor_id: Option<String>,
    worker_index: Option<usize>,
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
        let source = match (&config.reactor_id, config.worker_index) {
            (Some(reactor_id), Some(worker_index)) => {
                scada_standins::reactor_resident_source_declaration(
                    &config.source_id,
                    reactor_id,
                    worker_index,
                )
            }
            _ => scada_standins::resident_source_declaration(&config.source_id),
        };
        post_json(
            &source,
            &format!("{}/internal/scada/sources", base_url.trim_end_matches('/')),
            token,
        )?;
        let mut sequence = 1;
        loop {
            let frames = telemetry_frames_for_config(&config, sequence)?;
            post_json(
                &json!({ "frames": frames }),
                &format!(
                    "{}/internal/scada/telemetry",
                    base_url.trim_end_matches('/')
                ),
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
        let frames = telemetry_frames_for_config(&config, 1)?;
        for frame in frames {
            println!("{}", serde_json::to_string(&frame)?);
        }
    }
    Ok(())
}

fn telemetry_frames_for_config(
    config: &CliConfig,
    sequence: u64,
) -> scada_standins::Result<Vec<scada_standins::TelemetryFrame>> {
    match (&config.reactor_id, config.worker_index) {
        (Some(reactor_id), Some(worker_index)) => {
            let mut frames = scada_standins::reactor_telemetry_frames(
                &config.source_id,
                reactor_id,
                worker_index,
                sequence,
            );
            let sampled_at = current_rfc3339()?;
            for frame in &mut frames {
                frame.sampled_at = sampled_at.clone();
                frame.observed_at = sampled_at.clone();
            }
            Ok(frames)
        }
        _ => Ok(scada_standins::telemetry_frames(
            &config.source_id,
            sequence,
        )),
    }
}

fn current_rfc3339() -> scada_standins::Result<String> {
    let seconds = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
    Ok(unix_seconds_rfc3339(seconds))
}

fn unix_seconds_rfc3339(seconds: u64) -> String {
    let days = (seconds / 86_400) as i64;
    let seconds_of_day = seconds % 86_400;
    let z = days + 719_468;
    let era = if z >= 0 { z } else { z - 146_096 } / 146_097;
    let day_of_era = z - era * 146_097;
    let year_of_era =
        (day_of_era - day_of_era / 1_460 + day_of_era / 36_524 - day_of_era / 146_096) / 365;
    let mut year = year_of_era + era * 400;
    let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
    let month_prime = (5 * day_of_year + 2) / 153;
    let day = day_of_year - (153 * month_prime + 2) / 5 + 1;
    let month = month_prime + if month_prime < 10 { 3 } else { -9 };
    year += if month <= 2 { 1 } else { 0 };
    let hour = seconds_of_day / 3_600;
    let minute = (seconds_of_day % 3_600) / 60;
    let second = seconds_of_day % 60;
    format!("{year:04}-{month:02}-{day:02}T{hour:02}:{minute:02}:{second:02}Z")
}

fn parse_args<I>(args: I) -> scada_standins::Result<CliConfig>
where
    I: IntoIterator<Item = String>,
{
    let mut source_id = "SRC-MIXED-STANDIN-001".to_string();
    let mut reactor_id = None;
    let mut worker_index = None;
    let mut ingest_base_url = None;
    let mut ingest_token = None;
    let mut interval_ms = 1000;
    let mut max_frames = None;

    let mut args = args.into_iter();
    let _program = args.next();
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--source-id" => source_id = next_value(&mut args, "--source-id")?,
            "--reactor-id" => reactor_id = Some(next_value(&mut args, "--reactor-id")?),
            "--worker-index" => {
                let value: usize = next_value(&mut args, "--worker-index")?
                    .parse()
                    .map_err(|_| "--worker-index must be 0, 1, or 2".to_string())?;
                if value >= 3 {
                    return Err("--worker-index must be 0, 1, or 2".into());
                }
                worker_index = Some(value);
            }
            "--ingest-base-url" => {
                ingest_base_url = Some(next_value(&mut args, "--ingest-base-url")?)
            }
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

    if reactor_id.is_some() != worker_index.is_some() {
        return Err("--reactor-id and --worker-index must be supplied together".into());
    }

    Ok(CliConfig {
        source_id,
        reactor_id,
        worker_index,
        ingest_base_url,
        ingest_token,
        interval_ms,
        max_frames,
    })
}

fn next_value(
    args: &mut impl Iterator<Item = String>,
    flag: &str,
) -> scada_standins::Result<String> {
    args.next()
        .ok_or_else(|| format!("{flag} requires a value").into())
}

fn usage() -> &'static str {
    "usage: scada-standins [--source-id <id>] [--reactor-id <id> --worker-index <0|1|2>] [--ingest-base-url <url> --ingest-token <token>] [--interval-ms <n>] [--max-frames <n>]"
}

fn post_json<T: serde::Serialize>(
    payload: &T,
    url: &str,
    token: &str,
) -> scada_standins::Result<()> {
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_bounded_reactor_worker_identity() {
        let config = parse_args(
            [
                "scada-standins",
                "--source-id",
                "src-01",
                "--reactor-id",
                "reactor-a",
                "--worker-index",
                "2",
                "--max-frames",
                "1",
            ]
            .into_iter()
            .map(str::to_string),
        )
        .expect("parse reactor worker");
        assert_eq!(config.source_id, "src-01");
        assert_eq!(config.reactor_id.as_deref(), Some("reactor-a"));
        assert_eq!(config.worker_index, Some(2));
    }

    #[test]
    fn rejects_partial_or_out_of_bounds_reactor_identity() {
        for args in [
            vec!["scada-standins", "--reactor-id", "reactor-a"],
            vec!["scada-standins", "--worker-index", "0"],
            vec![
                "scada-standins",
                "--reactor-id",
                "reactor-a",
                "--worker-index",
                "3",
            ],
        ] {
            assert!(parse_args(args.into_iter().map(str::to_string)).is_err());
        }
    }

    #[test]
    fn formats_unix_time_for_live_resident_frames() {
        assert_eq!(unix_seconds_rfc3339(0), "1970-01-01T00:00:00Z");
        assert_eq!(unix_seconds_rfc3339(86_400), "1970-01-02T00:00:00Z");
    }

    #[test]
    fn dynamic_worker_frames_use_live_time_instead_of_fixture_time() {
        let config = CliConfig {
            source_id: "src-live".to_string(),
            reactor_id: Some("reactor-live".to_string()),
            worker_index: Some(0),
            ingest_base_url: None,
            ingest_token: None,
            interval_ms: 1_000,
            max_frames: Some(1),
        };
        let frames = telemetry_frames_for_config(&config, 1).expect("live frames");
        let now = current_rfc3339().expect("current timestamp");
        assert_eq!(&frames[0].observed_at[..10], &now[..10]);
        assert_ne!(frames[0].observed_at, "2026-07-06T15:00:01Z");
    }
}
