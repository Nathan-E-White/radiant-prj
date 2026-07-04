fn main() {
    if let Err(error) = simops_generator::cli::run_from_env() {
        eprintln!("simops-generator: {error}");
        std::process::exit(2);
    }
}
