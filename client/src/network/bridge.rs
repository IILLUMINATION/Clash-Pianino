use tokio::runtime::Runtime;
use std::sync::mpsc::{self, Receiver};
use super::{GameEvent, matchmaking::Matchmaker};

pub struct Bridge {
    _rt: Runtime, 
    rx: Receiver<GameEvent>,
    pub matchmaking: Matchmaker,
}

impl Bridge {
    pub fn new(player_name: String, trophies: i32) -> Self {
        let rt = Runtime::new().expect("Не удалось создать рантайм токио");
        let (tx, rx) = mpsc::channel();

        let matchmaking = Matchmaker::new(
            player_name, 
            trophies, 
            tx.clone(), 
            rt.handle().clone()
        );

        Self {
            _rt: rt,
            rx,
            matchmaking,
        }
    }

    pub fn poll(&self) -> Option<GameEvent> {
        self.rx.try_recv().ok()
    }
}
