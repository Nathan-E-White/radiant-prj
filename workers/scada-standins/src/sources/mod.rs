pub mod actuator;
pub mod comms;
pub mod electrical;
pub mod flux;
pub mod pressure;
pub mod temperature;

use crate::SourceTag;

pub fn all_tags() -> Vec<SourceTag> {
    vec![
        flux::tag(),
        temperature::tag(),
        pressure::tag(),
        actuator::tag(),
        electrical::tag(),
        comms::tag(),
    ]
}
