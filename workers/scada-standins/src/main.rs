fn main() {
    let tag_count = scada_standins::default_tags().len();
    println!("scada-standins scaffold: {tag_count} resident measured tags");
}
