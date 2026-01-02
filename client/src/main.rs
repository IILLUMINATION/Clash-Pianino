use moonwalk::{MoonWalk, ObjectId};
use moonwalk_bootstrap::{Application, Runner, WindowSettings};
use glam::{Vec2, Vec4};

#[cfg(target_os = "android")]
use android_activity::AndroidApp;

struct Game {
    screen_size: Vec2,
}

impl Game {
    fn new() -> Self {
        Self {
            screen_size: Vec2::new(800.0, 600.0),
        }
    }
}

impl Application for Game {
    fn on_start(&mut self, mw: &mut MoonWalk, viewport: Vec2) {
        self.screen_size = viewport;

        let bg = mw.new_rect();
        mw.set_position(bg, Vec2::ZERO);
        mw.set_size(bg, viewport * 2.0);
        mw.set_color(bg, Vec4::new(0.1, 0.1, 0.1, 1.0));
        mw.set_z_index(bg, 0.0); 

        let mut pb = mw.new_path_builder();
        pb.set_color(Vec4::new(1.0, 0.0, 0.0, 1.0));
        pb.move_to(10.0, 10.0);
        pb.line_to(100.0, 10.0);
        pb.line_to(50.0, 100.0);
        pb.close();
        
        let tex_id = pb.tessellate(mw, 200, 200);

        let id = mw.new_rect();
        mw.set_texture(id, tex_id);
    }

    fn on_update(&mut self, dt: f32) {
        
    }

    fn on_draw(&mut self, mw: &mut MoonWalk) {
        
    }

    fn on_resize(&mut self, mw: &mut MoonWalk, viewport: Vec2) {
        self.screen_size = viewport; 
    }
}

#[cfg(not(target_os = "android"))]
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let app = Game::new();
    let settings = WindowSettings::new("Clash", 800.0, 600.0).resizable(true);
    Runner::run(app, settings)
}

#[cfg(target_os = "android")]
#[unsafe(no_mangle)]
fn android_main(app: AndroidApp) {
    android_logger::init_once(
        android_logger::Config::default().with_max_level(log::LevelFilter::Info)
    );

    log::info!("MoonWalk: android_main started");

    let stress_app = Game::new();
    let settings = WindowSettings::new("Game Android", 0.0, 0.0);
    Runner::run(stress_app, settings, app).unwrap();
}

#[cfg(target_os = "android")]
fn main() {}
