fn main() {
    let path = std::env::args().nth(1).expect("need file path arg");
    std::fs::write(&path, "written by wasi").expect("failed to write file");
}
