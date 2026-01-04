use moonwalk::{MoonWalk, ObjectId};
use moonwalk_bootstrap::{Application, Runner, WindowSettings};
use glam::{Vec2, Vec4};

use tungstenite::{connect, Message};
use url::Url;

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

        let hundo_font = mw.load_font_from_bytes(include_bytes!("../assets/Hundo.ttf"), "hundo").unwrap();

        let button_size = Vec2::new(150.0, 50.0);

        let button_bg = mw.new_rect();
        mw.set_position(button_bg, Vec2::new(50.0, 50.0));
        mw.set_size(button_bg, button_size);
        mw.set_color(button_bg, Vec4::new(0.0, 0.5, 0.4, 1.0));
        mw.set_z_index(button_bg, 0.01);

        let text = "v boy";
        let text_size = mw.measure_text(text, hundo_font, 16.0, 9999.0);

        let button_text = mw.new_text(text, hundo_font, 16.0);
        mw.set_position(button_text, Vec2::new(50.0 - text_size.x / 2.0 + button_size.x / 2.0, 50.0 - text_size.y / 4.0 + button_size.y / 2.0));
        mw.set_z_index(button_text, 0.02);
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
    
    // [HACK]
    // Тут анварпами насрано пж пофиксить нужно и юрл в константу пж пж
    // let ws_url = Url::parse("ws://64.188.64.35:8080/ws").unwrap();
    // let (mut socket, response) = connect(ws_url).expect("Не удалось подключится гандлн");
    //
    // println!("Я РОДИЛСЯЯЯ!!! ХТТП КОД: {}", response.status());
    // socket.write_message(Message::Text("Привет пидорас!".into())).unwrap();
    //
    // loop {
    //    let msg = socket.read_message().expect("Ошибка чтения сука ебланище тупое");
    //
    //    println!("ОДНО СООБЩЕНИЕ НАХУЙ, НАМ НАПИСАЛИ!!! АХУЕЕЕЕТЬ!!!!! {}", msg)
    // }
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
