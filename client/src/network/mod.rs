pub mod bridge;
pub mod matchmaking;

pub mod pb {
    include!(concat!(env!("OUT_DIR"), "/game.rs"));
}

#[derive(Debug)]
pub enum GameEvent {
    // События матчмейкинга
    MatchFound { 
        opponent_id: String, 
        opponent_trophies: i32, 
        room_id: String 
    },

    // Технические события
    NetworkError(String),
}
