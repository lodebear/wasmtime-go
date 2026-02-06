fn main() {
    let code: i32 = std::env::args()
        .nth(1)
        .expect("need exit code arg")
        .parse()
        .expect("invalid exit code");
    std::process::exit(code);
}
