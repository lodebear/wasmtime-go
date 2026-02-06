fn main() {
    match std::env::var("TEST_KEY") {
        Ok(val) => print!("{}", val),
        Err(_) => print!("NOT_SET"),
    }
}
