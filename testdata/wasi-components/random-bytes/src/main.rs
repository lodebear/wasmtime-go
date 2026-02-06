fn main() {
    let mut buf = [0u8; 16];
    getrandom::getrandom(&mut buf).expect("failed to get random bytes");
    let hex: String = buf.iter().map(|b| format!("{:02x}", b)).collect();
    print!("{}", hex);
}
