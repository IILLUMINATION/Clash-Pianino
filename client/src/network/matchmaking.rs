use tokio::runtime::Handle;
use tokio_tungstenite::{connect_async, tungstenite::protocol::Message as WsMessage};
use futures::{StreamExt, SinkExt};
use prost::Message;
use url::Url;
use std::sync::mpsc::Sender;

use super::{pb, GameEvent};

pub struct Matchmaker {
    player_name: String,
    trophies: i32,
    tx: Sender<GameEvent>,
    rt_handle: Handle,
}

impl Matchmaker {
    pub fn new(player_name: String, trophies: i32, tx: Sender<GameEvent>, rt_handle: Handle) -> Self {
        Self {
            player_name,
            trophies,
            tx,
            rt_handle,
        }
    }

    pub fn find_game(&self) {
        let tx = self.tx.clone();
        let player_id = self.player_name.clone();
        let trophies = self.trophies;
        
        self.rt_handle.spawn(async move {
            // [MAYBE]
            // Потом нужно создать константу
            let url = Url::parse("ws://64.188.64.35:8080/ws").unwrap();
            
            match connect_async(url).await {
                Ok((ws_stream, _)) => {
                    let (mut write, mut read) = ws_stream.split();

                    let request = pb::JoinQueueRequest {
                        player_id,
                        trophies,
                    };

                    let mut buf = Vec::new();
                    request.encode(&mut buf).unwrap();

                    if let Err(e) = write.send(WsMessage::Binary(buf)).await {
                        let _ = tx.send(GameEvent::NetworkError(format!("Send error: {}", e)));
                        return;
                    }

                    while let Some(msg) = read.next().await {
                        match msg {
                            Ok(WsMessage::Binary(data)) => {
                                if let Ok(response) = pb::MatchFoundResponse::decode(&data[..]) {
                                    let _ = tx.send(GameEvent::MatchFound {
                                        opponent_id: response.opponent_id,
                                        opponent_trophies: response.opponent_trophies,
                                        room_id: response.room_id,
                                    });
                                    break;
                                }
                            }

                            Ok(WsMessage::Close(_)) => break,

                            Err(e) => {
                                let _ = tx.send(GameEvent::NetworkError(format!("Read error: {}", e)));
                                break;
                            }

                            _ => {}
                        }
                    }
                }
                Err(e) => {
                    let _ = tx.send(GameEvent::NetworkError(format!("Connect error: {}", e)));
                }
            }
        });
    }
}
