use std::env;
use std::fs::read_to_string;

use crate::generators::{WorkerSelection, generate_run};
use crate::manifest::RunManifest;
use crate::output::{build_summary, ensure_positive_frames, write_ndjson, write_summary};
use crate::{Result, SimopsError};

#[derive(Debug, Clone)]
struct CliConfig {
    manifest_path: String,
    worker_selection: WorkerSelection,
    frames: Option<usize>,
    output_path: String,
    summary_path: Option<String>,
}

pub fn run_from_env() -> Result<()> {
    run(env::args())
}

pub fn run<I>(args: I) -> Result<()>
where
    I: IntoIterator<Item = String>,
{
    let config = parse_args(args)?;
    let manifest_text = read_to_string(&config.manifest_path)?;
    let manifest = RunManifest::from_json(&manifest_text)?;
    let run = generate_run(&manifest, config.worker_selection, config.frames)?;

    write_ndjson(&run, &config.output_path)?;

    if let Some(summary_path) = config.summary_path {
        let summary = build_summary(&manifest, &run)?;
        write_summary(&summary, &summary_path)?;
    }

    Ok(())
}

fn parse_args<I>(args: I) -> Result<CliConfig>
where
    I: IntoIterator<Item = String>,
{
    let mut manifest_path = None;
    let mut worker_selection = WorkerSelection::All;
    let mut frames = None;
    let mut output_path = "-".to_string();
    let mut summary_path = None;

    let mut args = args.into_iter();
    let _program = args.next();

    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--manifest" => manifest_path = Some(next_value(&mut args, "--manifest")?),
            "--worker" => {
                worker_selection = WorkerSelection::parse(&next_value(&mut args, "--worker")?)?
            }
            "--frames" => {
                let value = next_value(&mut args, "--frames")?
                    .parse::<usize>()
                    .map_err(|_| SimopsError::new("--frames must be a positive integer"))?;
                frames = Some(ensure_positive_frames(value)?);
            }
            "--output" => output_path = next_value(&mut args, "--output")?,
            "--summary" => summary_path = Some(next_value(&mut args, "--summary")?),
            "--help" | "-h" => {
                println!("{}", usage());
                std::process::exit(0);
            }
            other => {
                return Err(SimopsError::new(format!(
                    "unknown argument {other}\n{}",
                    usage()
                )));
            }
        }
    }

    let manifest_path = manifest_path.ok_or_else(|| {
        SimopsError::new(format!("missing required --manifest <path>\n{}", usage()))
    })?;

    Ok(CliConfig {
        manifest_path,
        worker_selection,
        frames,
        output_path,
        summary_path,
    })
}

fn next_value(args: &mut impl Iterator<Item = String>, flag: &str) -> Result<String> {
    args.next()
        .ok_or_else(|| SimopsError::new(format!("{flag} requires a value")))
}

fn usage() -> &'static str {
    "usage: simops-generator --manifest <path> [--worker scheduler|storage|burst|fabric|all] [--frames <n>] [--output <path|->] [--summary <path>]"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn requires_manifest_path() {
        let error =
            parse_args(["simops-generator".to_string()]).expect_err("missing manifest should fail");
        assert!(error.to_string().contains("missing required --manifest"));
    }

    #[test]
    fn parses_worker_and_frames() {
        let config = parse_args([
            "simops-generator".to_string(),
            "--manifest".to_string(),
            "manifest.json".to_string(),
            "--worker".to_string(),
            "scheduler".to_string(),
            "--frames".to_string(),
            "2".to_string(),
            "--output".to_string(),
            "-".to_string(),
        ])
        .expect("valid args");

        assert_eq!(
            config.worker_selection,
            WorkerSelection::One(crate::manifest::WorkerKind::Scheduler)
        );
        assert_eq!(config.frames, Some(2));
    }
}
