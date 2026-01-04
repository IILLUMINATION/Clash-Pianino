mod network;

use moonwalk::{MoonWalk, ObjectId};
use moonwalk_bootstrap::{Application, Runner, WindowSettings, TouchPhase};
use glam::{Vec2, Vec4};
use network::{GameEvent, bridge::Bridge};

// Для тестовых данных через аргументы терминала
use std::env;

#[cfg(target_os = "android")]
use android_activity::AndroidApp;

struct Game {
    screen_size: Vec2,
    bridge: Bridge,
}

impl Game {
    fn new(player_name: String, trophies: i32) -> Self {
        println!("Клэш пианино для: {} у которого {} кубков", player_name, trophies);
        
        Self {
            screen_size: Vec2::new(800.0, 600.0),
            bridge: Bridge::new(player_name, trophies),
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
        mw.set_hit_group(button_bg, 1);

        let text = "v boy";
        let text_size = mw.measure_text(text, hundo_font, 16.0, 9999.0);
        let button_text = mw.new_text(text, hundo_font, 16.0);
        mw.set_position(button_text, Vec2::new(50.0 - text_size.x / 2.0 + button_size.x / 2.0, 50.0 - text_size.y / 4.0 + button_size.y / 2.0));
        mw.set_z_index(button_text, 0.02);
    }

    fn on_update(&mut self, _dt: f32) {
        if let Some(event) = self.bridge.poll() {
            match event {
                GameEvent::MatchFound { opponent_id, opponent_trophies, room_id } => {
                    println!("Мост: Матч найден против {} (кубки: {}), комната: {}", opponent_id, opponent_trophies, room_id);
                }

                GameEvent::NetworkError(err) => {
                    println!("Мост: Сетевая ошибка: {}", err);
                }
            }
        }
    }

    fn on_draw(&mut self, _mw: &mut MoonWalk) {

    }

    fn on_touch(&mut self, mw: &mut MoonWalk, phase: TouchPhase, position: Vec2) {
        if let Some(hit_id) = mw.resolve_hit(position, Vec2::new(1.0, 1.0), 1) {
            match phase {
                TouchPhase::Ended | TouchPhase::Cancelled => {
                    mw.set_color(hit_id, Vec4::new(0.0, 0.5, 0.4, 1.0));
                    println!("В поисках соперника...");
                    self.bridge.matchmaking.find_game();
                }

                TouchPhase::Started => {
                    mw.set_color(hit_id, Vec4::new(0.0, 0.3, 0.2, 1.0));
                }

                _ => {}
            }
        }
    }

    fn on_resize(&mut self, _mw: &mut MoonWalk, viewport: Vec2) {
        self.screen_size = viewport; 
    }
}

#[cfg(not(target_os = "android"))]
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();

    // 1 аргумент это имя, второй кубки
    let player_name = args.get(1)
        .cloned()
        .unwrap_or_else(|| "Player".to_string());

    let trophies = args.get(2)
        .and_then(|s| s.parse::<i32>().ok())
        .unwrap_or(0);

    let app = Game::new(player_name, trophies);
    let settings = WindowSettings::new("Clash Piano", 800.0, 600.0).resizable(true);
    Runner::run(app, settings)
}

#[cfg(target_os = "android")]
#[no_mangle]
fn android_main(app: AndroidApp) {
    android_logger::init_once(
        android_logger::Config::default().with_max_level(log::LevelFilter::Info)
    );
    
    let app_game = Game::new("Player Android".to_string(), 1000);
    
    let settings = WindowSettings::new("Game Android", 0.0, 0.0);
    Runner::run(app_game, settings, app).unwrap();
}
