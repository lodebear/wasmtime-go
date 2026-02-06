fn main() {
    let path = std::env::args().nth(1).expect("need file path arg");
    let contents = std::fs::read_to_string(&path).expect("failed to read file");
    print!("{}", contents);
}
