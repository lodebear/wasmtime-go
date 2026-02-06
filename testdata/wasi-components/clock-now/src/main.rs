use std::time::{Instant, SystemTime, UNIX_EPOCH};

fn main() {
    let mono = {
        let start = Instant::now();
        start.elapsed().as_nanos()
    };
    let wall = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("time went backwards")
        .as_secs();
    print!("mono:{} wall:{}", mono, wall);
}
