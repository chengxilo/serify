// Copyright 2026 Chengxi Luo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//! The Rust client for the taskstore server.
//!
//!     cargo run --release --bin client -- list
//!     cargo run --release --bin client -- create "Buy milk" normal errand,home 1755820800
//!     cargo run --release --bin client -- read 1001
//!     cargo run --release --bin client -- update 1001 "Buy oat milk" true high errand
//!     cargo run --release --bin client -- delete 1001
//!
//! It takes the same arguments as the Go and Python clients and speaks the same
//! bytes, so it talks to the Go server without either side knowing which
//! language the other is.

use std::env;
use std::io::BufReader;
use std::net::TcpStream;
use std::process::exit;

use taskstore::frame::{read_frame, write_frame};
use taskstore::message::{Request, Response};
use taskstore::model::{ApiError, Draft, Task, TaskPage, PRIORITIES};
use taskstore::op::Op;
use taskstore::result::ApiResult;

const USAGE: &str = "usage: client [--addr host:port] <command> [args]

  list
  create <title> [priority] [tag,tag] [due-unix]
  read   <id>
  update <id> <title> <done> [priority] [tag,tag] [due-unix]
  delete <id>

priority is low, normal or high (default normal).";

fn main() {
    let mut args: Vec<String> = env::args().skip(1).collect();

    let mut addr = "127.0.0.1:9977".to_string();
    if args.first().map(String::as_str) == Some("--addr") && args.len() >= 2 {
        addr = args[1].clone();
        args.drain(0..2);
    }

    let op = match parse_op(&args) {
        Ok(op) => op,
        Err(e) => {
            eprintln!("{e}\n\n{USAGE}");
            exit(2);
        }
    };

    match send(&addr, Request { request_id: 1, op }) {
        Err(e) => {
            eprintln!("{addr}: {e}");
            exit(1);
        }
        Ok(resp) => {
            println!("{}", render(&resp.result));
            if matches!(resp.result, ApiResult::Failed(_)) {
                exit(1);
            }
        }
    }
}

/// Opens a connection, writes one request frame and reads one response frame.
/// Nothing here knows what an operation is — the codec does that.
fn send(addr: &str, req: Request) -> Result<Response, String> {
    let payload = req.marshal()?;

    let stream = TcpStream::connect(addr).map_err(|e| e.to_string())?;
    let mut writer = stream.try_clone().map_err(|e| e.to_string())?;
    let mut reader = BufReader::new(stream);

    write_frame(&mut writer, &payload).map_err(|e| e.to_string())?;
    match read_frame(&mut reader).map_err(|e| e.to_string())? {
        None => Err("server closed the connection without answering".into()),
        Some(frame) => Response::unmarshal(&frame),
    }
}

fn parse_op(args: &[String]) -> Result<Op, String> {
    let arg = |i: usize, fallback: &str| -> String {
        match args.get(i) {
            Some(s) if !s.is_empty() => s.clone(),
            _ => fallback.to_string(),
        }
    };
    let id = |raw: &String| raw.parse::<u64>().map_err(|_| format!("bad id {raw:?}"));

    let command = args.first().ok_or("no command given")?.as_str();
    match command {
        "list" => Ok(Op::ListAll),

        "create" => {
            let title = args.get(1).ok_or("create needs a title")?;
            Ok(Op::Create(Draft {
                title: title.clone(),
                priority: parse_priority(&arg(2, "normal"))?,
                tags: parse_tags(&arg(3, "")),
                due_at: parse_due(&arg(4, "")),
            }))
        }

        "read" => Ok(Op::Read(id(args.get(1).ok_or("read needs an id")?)?)),
        "delete" => Ok(Op::Delete(id(args.get(1).ok_or("delete needs an id")?)?)),

        "update" => {
            if args.len() < 4 {
                return Err("update needs an id, a title and done".into());
            }
            let done = match args[3].as_str() {
                "true" => true,
                "false" => false,
                other => return Err(format!("bad done {other:?}, want true or false")),
            };
            Ok(Op::Update(Task {
                id: id(&args[1])?,
                title: args[2].clone(),
                done,
                priority: parse_priority(&arg(4, "normal"))?,
                tags: parse_tags(&arg(5, "")),
                due_at: parse_due(&arg(6, "")),
            }))
        }

        other => Err(format!("unknown command {other:?}")),
    }
}

/// Rejects a bad priority here, before anything is encoded. This is the only
/// place in the project where one can exist: an enum has no wire representation
/// outside its declared variants, so by the time a request is bytes the value is
/// already known to be good — which is why the server does not check it again.
fn parse_priority(s: &str) -> Result<String, String> {
    if !PRIORITIES.contains(&s) {
        return Err(format!("{s:?} is not a priority ({})", PRIORITIES.join(", ")));
    }
    Ok(s.to_string())
}

fn parse_tags(s: &str) -> Vec<String> {
    if s.is_empty() {
        return Vec::new();
    }
    s.split(',').map(str::to_string).collect()
}

fn parse_due(s: &str) -> Option<i64> {
    s.parse().ok()
}

/// `match` over the enum is exhaustive: a fifth outcome is a compile error here,
/// which is the whole reason to model a response as a sum.
fn render(result: &ApiResult) -> String {
    match result {
        ApiResult::Accepted => "accepted".to_string(),
        ApiResult::Found(t) => render_task(t),
        ApiResult::Listing(TaskPage { items, total }) => {
            let mut lines: Vec<String> = items.iter().map(render_task).collect();
            lines.push(format!("({} shown, {total} total)", items.len()));
            lines.join("\n")
        }
        ApiResult::Failed(ApiError { code, message }) => format!("error: {code}: {message}"),
    }
}

fn render_task(t: &Task) -> String {
    let mut line = format!(
        "[{}] {}  {:<30}  {}",
        if t.done { "x" } else { " " },
        t.id,
        t.title,
        t.priority
    );
    if !t.tags.is_empty() {
        line += &format!("  #{}", t.tags.join(" #"));
    }
    if let Some(due) = t.due_at {
        line += &format!("  due={due}");
    }
    line
}
