use std::{fs, time::Duration};

use bumpalo::Bump;
use criterion::{criterion_group, Criterion};
use itertools::Itertools;
use uplc::{binder::DeBruijn, flat};

pub fn run(c: &mut Criterion) {
    let data_dir = std::path::Path::new("benches/benchmarks/data");

    for path in fs::read_dir(data_dir)
        .unwrap()
        .map(|entry| entry.unwrap())
        .map(|entry| entry.path())
        .sorted()
    {
        if path.is_file() {
            let file_name = path
                .file_name()
                .unwrap()
                .to_str()
                .unwrap()
                .replace(".flat", "");

            let script = std::fs::read(&path).unwrap();

            let mut arena = Bump::with_capacity(1_024_000);

            c.bench_function(&file_name, |b| {
                b.iter(|| {
                    let program =
                        flat::decode::<DeBruijn>(&arena, &script).expect("Failed to decode");

                    let result = program.eval(&arena);

                    let _term = result.term.expect("Failed to evaluate");

                    arena.reset();
                })
            });
        }
    }
}

criterion_group! {
    name = haskell;
    config = Criterion::default()
        .measurement_time(Duration::from_secs(10));
    targets = run
}
