pub mod cli;
pub mod envelope;
pub mod generators;
pub mod manifest;
pub mod output;
pub mod sampler;

use std::error::Error;
use std::fmt::{Display, Formatter};

pub type Result<T> = std::result::Result<T, SimopsError>;

#[derive(Debug, Clone)]
pub struct SimopsError {
    message: String,
}

impl SimopsError {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }
}

impl Display for SimopsError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl Error for SimopsError {}

impl From<std::io::Error> for SimopsError {
    fn from(value: std::io::Error) -> Self {
        Self::new(value.to_string())
    }
}

impl From<serde_json::Error> for SimopsError {
    fn from(value: serde_json::Error) -> Self {
        Self::new(value.to_string())
    }
}

impl From<time::error::Parse> for SimopsError {
    fn from(value: time::error::Parse) -> Self {
        Self::new(value.to_string())
    }
}

impl From<time::error::Format> for SimopsError {
    fn from(value: time::error::Format) -> Self {
        Self::new(value.to_string())
    }
}
