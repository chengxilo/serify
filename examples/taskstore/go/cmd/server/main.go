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

// Command server is the taskstore itself: it listens on TCP, reads
// length-prefixed request frames, applies them to an in-memory store and writes
// back response frames.
//
// It imports api and store. It does not import serify — the harness has no
// place in the running server, and the fact that this binary links none of it is
// the clearest statement of that.
//
//	go run ./cmd/server -addr 127.0.0.1:9977
package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"net"

	"github.com/chengxilo/serify/example-taskstore/api"
	"github.com/chengxilo/serify/example-taskstore/store"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9977", "address to listen on")
	flag.Parse()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	log.Printf("taskstore listening on %s", listener.Addr())

	tasks := store.New()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}
		go serve(conn, tasks)
	}
}

// serve handles one connection until the client hangs up. A client may send any
// number of requests down it; each is answered in order.
func serve(conn net.Conn, tasks *store.Store) {
	defer conn.Close()

	for {
		frame, err := api.ReadFrame(conn)
		if errors.Is(err, io.EOF) {
			return // a clean hang-up between requests
		}
		if err != nil {
			log.Printf("%s: read: %v", conn.RemoteAddr(), err)
			return
		}

		var req api.Request
		if err := req.UnmarshalBinary(frame); err != nil {
			// The frame was not a request this server understands, and there is
			// no request id to answer with — the id lives inside the bytes that
			// just failed to parse. Nothing to do but drop the connection.
			log.Printf("%s: bad request: %v", conn.RemoteAddr(), err)
			return
		}

		resp := tasks.Apply(req)
		payload, err := resp.MarshalBinary()
		if err != nil {
			log.Printf("%s: marshal response: %v", conn.RemoteAddr(), err)
			return
		}
		if err := api.WriteFrame(conn, payload); err != nil {
			log.Printf("%s: write: %v", conn.RemoteAddr(), err)
			return
		}
	}
}
