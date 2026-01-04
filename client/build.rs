fn main() {
    prost_build::compile_protos(&["../proto/game.proto"], &["../proto/"]).unwrap();
}
